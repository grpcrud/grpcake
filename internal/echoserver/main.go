package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"io"
	"io/ioutil"
	"net"
	"strconv"
	"time"

	"github.com/grpcrud/grpcake/internal/echo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/emptypb"
)

func main() {
	network := flag.String("network", "tcp", "serve network")
	addr := flag.String("addr", "localhost:50051", "serve address")
	insecure_ := flag.Bool("insecure", false, "disable transport security")
	serverTLS := flag.Bool("server-tls", false, "serve over tls")
	serverCertFile := flag.String("server-cert-file", "internal/echoserver/server.crt", "server cert file")
	serverKeyFile := flag.String("server-key-file", "internal/echoserver/server.key", "server key file")
	clientTLS := flag.Bool("client-tls", false, "require client tls auth")
	clientCACertFile := flag.String("client-ca-cert-file", "internal/echoserver/client-ca.crt", "client CA cert file")
	reflection_ := flag.Bool("reflection", false, "enable reflection")
	flag.Parse()

	var tlsConfig tls.Config

	if *serverTLS {
		serverCert, err := tls.LoadX509KeyPair(*serverCertFile, *serverKeyFile)
		if err != nil {
			panic(err)
		}

		tlsConfig.Certificates = []tls.Certificate{serverCert}
	}

	if *clientTLS {
		var certPool *x509.CertPool
		if clientCACertFile != nil {
			certPool = x509.NewCertPool()

			clientCA, err := ioutil.ReadFile(*clientCACertFile)
			if err != nil {
				panic(err)
			}

			if !certPool.AppendCertsFromPEM(clientCA) {
				panic("could not parse client CA file")
			}
		} else {
			var err error
			certPool, err = x509.SystemCertPool()
			if err != nil {
				panic(err)
			}
		}

		tlsConfig.ClientCAs = certPool
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	l, err := net.Listen(*network, *addr)
	if err != nil {
		panic(err)
	}

	var creds credentials.TransportCredentials
	if *insecure_ {
		creds = insecure.NewCredentials()
	} else {
		creds = credentials.NewTLS(&tlsConfig)
	}

	s := grpc.NewServer(grpc.Creds(creds), grpc.UnaryInterceptor(unaryInterceptor), grpc.StreamInterceptor(streamInterceptor))
	echo.RegisterEchoServer(s, server{})

	if *reflection_ {
		reflection.Register(s)
	}

	if err := s.Serve(l); err != nil {
		panic(err)
	}
}

func unaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	if err := grpc.SetHeader(ctx, metadata.Pairs("full_method", info.FullMethod)); err != nil {
		return nil, err
	}

	start := time.Now()
	resp, err := handler(ctx, req)

	if err := grpc.SetTrailer(ctx, metadata.Pairs("latency", time.Since(start).String())); err != nil {
		return nil, err
	}

	return resp, err
}

func streamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	if err := ss.SetHeader(metadata.Pairs("full_method", info.FullMethod)); err != nil {
		return err
	}

	start := time.Now()
	if err := handler(srv, ss); err != nil {
		return err
	}

	ss.SetTrailer(metadata.Pairs("latency", time.Since(start).String()))
	return nil
}

type server struct {
	echo.UnimplementedEchoServer
}

func (s server) Ping(_ context.Context, msg *emptypb.Empty) (*echo.PingMessage, error) {
	return &echo.PingMessage{Pong: true}, nil
}

func (s server) Echo(ctx context.Context, msg *echo.EchoMessage) (*echo.EchoMessage, error) {
	return msg, nil
}

func (s server) ClientStreamEcho(stream echo.Echo_ClientStreamEchoServer) error {
	var count int32
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		count++
	}

	return stream.SendAndClose(&echo.CountMessage{Count: count})
}

func (s server) ServerStreamEcho(msg *echo.CountMessage, stream echo.Echo_ServerStreamEchoServer) error {
	for i := 0; i < int(msg.Count); i++ {
		if err := stream.Send(&echo.EchoMessage{Message: strconv.Itoa(i)}); err != nil {
			return err
		}
	}

	return nil
}

func (s server) BidiStreamEcho(stream echo.Echo_BidiStreamEchoServer) error {
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}

		if err != nil {
			return err
		}

		if err := stream.Send(msg); err != nil {
			return err
		}
	}
}

func (s server) EchoMetadata(ctx context.Context, _ *emptypb.Empty) (*echo.MetadataMessage, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	res := &echo.MetadataMessage{Metadata: map[string]*echo.MetadataMessage_Values{}}
	for k, vs := range md {
		res.Metadata[k] = &echo.MetadataMessage_Values{}
		for _, v := range vs {
			res.Metadata[k].Values = append(res.Metadata[k].Values, v)
		}
	}

	return res, nil
}
