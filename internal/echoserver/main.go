package main

import (
	"context"
	"io"
	"net"
	"strconv"

	"github.com/grpcrud/grpcake/internal/echo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/emptypb"
)

func main() {
	l, err := net.Listen("tcp", "localhost:8080")
	if err != nil {
		panic(err)
	}

	s := grpc.NewServer()
	echo.RegisterEchoServer(s, server{})
	reflection.Register(s)

	if err := s.Serve(l); err != nil {
		panic(err)
	}
}

type server struct {
	echo.UnimplementedEchoServer
}

func (s server) Ping(_ context.Context, msg *emptypb.Empty) (*echo.PingMessage, error) {
	return &echo.PingMessage{Pong: true}, nil
}

func (s server) Echo(_ context.Context, msg *echo.EchoMessage) (*echo.EchoMessage, error) {
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
