package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/sync/errgroup"
	"golang.org/x/term"
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

	g, ctx := errgroup.WithContext(ctx)

	stream, err := cc.NewStream(ctx, &streamDesc, methodInvokeName(string(method.FullName())))
	if err != nil {
		return err
	}

	// warn about stdin being a tty
	if !args.NoWarnStdinTTY && term.IsTerminal(int(os.Stdin.Fd())) {
		_, _ = fmt.Fprintln(os.Stderr, "warning: reading message(s) from stdin (disable this message with --no-warn-stdin-tty)")
	}

	// write stdin to stream
	g.Go(func() error {
		scan := bufio.NewScanner(os.Stdin)
		for scan.Scan() {
			msg := dynamicpb.NewMessage(method.Input())
			if err := protojson.Unmarshal(scan.Bytes(), msg); err != nil {
				return err
			}

			if err := stream.SendMsg(msg); err != nil {
				return err
			}
		}

		return stream.CloseSend()
	})

	// write stream to stdout (and header/trailer to stderr)
	g.Go(func() error {
		header, err := stream.Header()
		if err != nil {
			return err
		}

		if args.DumpHeader {
			log, err := json.Marshal(headerTrailer{Header: header})
			if err != nil {
				return fmt.Errorf("marshal header/trailer: %w", err)
			}

			_, _ = fmt.Fprintln(os.Stderr, string(log))
		}

		for {
			msg := dynamicpb.NewMessage(method.Output())
			if err := stream.RecvMsg(msg); err != nil {
				if err == io.EOF {
					break
				}

				return err
			}

			b, err := protojson.Marshal(msg)
			if err != nil {
				return err
			}

			fmt.Println(string(b))
		}

		trailer := stream.Trailer()
		if args.DumpTrailer {
			log, err := json.Marshal(headerTrailer{Trailer: trailer})
			if err != nil {
				return fmt.Errorf("marshal header/trailer: %w", err)
			}

			_, _ = fmt.Fprintln(os.Stderr, string(log))
		}

		return nil
	})

	return g.Wait()
}

type headerTrailer struct {
	Header  metadata.MD `json:"header,omitempty"`
	Trailer metadata.MD `json:"trailer,omitempty"`
}

func methodInvokeName(name string) string {
	i := strings.LastIndexByte(name, '.')
	return "/" + name[:i] + "/" + name[i+1:]
}
