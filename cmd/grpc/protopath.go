package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type protopathMethodSource struct {
	protosetMethodSource
}

func newProtopathMethodSource(ctx context.Context, protopath []string) (protopathMethodSource, error) {
	if len(protopath) == 0 {
		return protopathMethodSource{}, fmt.Errorf("proto paths list cannot be empty")
	}

	var protoFile string
	for _, p := range protopath {
		f, err := findArbitraryProtoFile(p)
		if err != nil {
			return protopathMethodSource{}, err
		}

		if f != "" {
			protoFile = f
			break
		}
	}

	if protoFile == "" {
		return protopathMethodSource{}, fmt.Errorf("cannot find any .proto files in proto paths list")
	}

	f, err := os.CreateTemp("", "protoset")
	if err != nil {
		return protopathMethodSource{}, fmt.Errorf("create temp file: %w", err)
	}

	defer os.Remove(f.Name())

	var args []string
	for _, p := range protopath {
		args = append(args, "-I", p)
	}

	args = append(args, "--descriptor_set_out", f.Name(), "--include_imports", protoFile)

	cmd := exec.CommandContext(ctx, "protoc", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return protopathMethodSource{}, fmt.Errorf("protoc: %w", err)
	}

	msrc, err := newProtosetMethodSource([]string{f.Name()})
	if err != nil {
		return protopathMethodSource{}, err
	}

	return protopathMethodSource{protosetMethodSource: msrc}, nil
}

var stopWalkDir = errors.New("protopath: stop walk dir")

func findArbitraryProtoFile(path string) (string, error) {
	var out string
	err := filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		if strings.HasSuffix(path, ".proto") {
			out = path
			return stopWalkDir
		}

		return nil
	})

	if err != nil && err != stopWalkDir {
		return "", err
	}

	return out, nil
}
