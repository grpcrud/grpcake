package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

func dial(args args) (*grpc.ClientConn, error) {
	// always parse tls params, even if they are ultimately ignored, so that the
	// user gets validation errors early
	tlsConfig, err := tlsConfig(args)
	if err != nil {
		return nil, err
	}

	target, isShorthand := parseTarget(args.Target)

	var creds credentials.TransportCredentials
	if isShorthand || args.Insecure {
		creds = insecure.NewCredentials()
	} else {
		creds = credentials.NewTLS(tlsConfig)
	}

	cc, err := grpc.Dial(target, grpc.WithTransportCredentials(creds), grpc.WithUserAgent(args.UserAgent))
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}

	return cc, nil
}

func tlsConfig(args args) (*tls.Config, error) {
	var certPool *x509.CertPool
	if len(args.ServerRootCA) > 0 {
		// tls.Config's default is to use system pool, so only override
		// certPool if user provides CAs
		certPool = x509.NewCertPool()
		for _, f := range args.ServerRootCA {
			serverCA, err := ioutil.ReadFile(f)
			if err != nil {
				panic(err)
			}

			if !certPool.AppendCertsFromPEM(serverCA) {
				return nil, fmt.Errorf("could not parse server CA file: %s", f)
			}
		}
	}

	if len(args.ClientCert) != len(args.ClientKey) {
		return nil, fmt.Errorf("--client-cert and --client-key must be passed an equal number of times")
	}

	var certs []tls.Certificate
	for i, c := range args.ClientCert {
		k := args.ClientKey[i]
		cert, err := tls.LoadX509KeyPair(c, k)
		if err != nil {
			return nil, fmt.Errorf("loading client key pair: cert: %s, key: %s: %w", c, k, err)
		}

		certs = append(certs, cert)
	}

	return &tls.Config{
		InsecureSkipVerify: args.InsecureSkipServerVerify,
		RootCAs:            certPool,
		ServerName:         args.ServerName,
		Certificates:       certs,
	}, nil
}
