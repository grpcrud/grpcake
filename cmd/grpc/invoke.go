package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

func invokeMethod(ctx context.Context, cc *grpc.ClientConn, msrc methodSource, args args) error {
	method, err := msrc.Method(protoreflect.FullName(args.Method))
	if err != nil {
		return err
	}

	streamDesc := grpc.StreamDesc{
		ServerStreams: method.IsStreamingServer(),
		ClientStreams: method.IsStreamingClient(),
	}

	stream, err := cc.NewStream(ctx, &streamDesc, methodInvokeName(string(method.FullName())))
	if err != nil {
		return err
	}

	scan := bufio.NewScanner(os.Stdin)
	for scan.Scan() {
		msgIn := dynamicpb.NewMessage(method.Input())
		if err := protojson.Unmarshal(scan.Bytes(), msgIn); err != nil {
			return err
		}

		if err := stream.SendMsg(msgIn); err != nil {
			return err
		}
	}

	if err := stream.CloseSend(); err != nil {
		return err
	}

	if args.DumpHeader {
		header, err := stream.Header()
		if err != nil {
			return err
		}

		log, err := json.Marshal(headerTrailer{Header: header})
		if err != nil {
			panic(fmt.Errorf("marshal header/trailer: %w", err))
		}

		_, _ = fmt.Fprintln(os.Stderr, string(log))
	}

	for {
		msgOut := dynamicpb.NewMessage(method.Output())
		if err := stream.RecvMsg(msgOut); err != nil {
			if err == io.EOF {
				break
			}

			return err
		}

		outputBytes, err := protojson.Marshal(msgOut)
		if err != nil {
			return err
		}

		fmt.Println(string(outputBytes))
	}

	if args.DumpTrailer {
		log, err := json.Marshal(headerTrailer{Trailer: stream.Trailer()})
		if err != nil {
			panic(fmt.Errorf("marshal header/trailer: %w", err))
		}

		_, _ = fmt.Fprintln(os.Stderr, string(log))
	}

	return nil
}

type headerTrailer struct {
	Header  metadata.MD `json:"header,omitempty"`
	Trailer metadata.MD `json:"trailer,omitempty"`
}

func methodInvokeName(name string) string {
	i := strings.LastIndexByte(name, '.')
	return "/" + name[:i] + "/" + name[i+1:]
}
