package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

type protosetMethodSource struct {
	reg *protoregistry.Files
}

func newProtosetMethodSource(protosets []string) (protosetMethodSource, error) {
	var fds descriptorpb.FileDescriptorSet
	for _, p := range protosets {
		f, err := os.Open(p)
		if err != nil {
			return protosetMethodSource{}, fmt.Errorf("open %s: %w", p, err)
		}

		b, err := ioutil.ReadAll(f)
		if err != nil {
			return protosetMethodSource{}, fmt.Errorf("read %s: %w", p, err)
		}

		var subFDS descriptorpb.FileDescriptorSet
		if err := proto.Unmarshal(b, &subFDS); err != nil {
			return protosetMethodSource{}, fmt.Errorf("unmarshal %s: %w", p, err)
		}

		fds.File = append(fds.File, subFDS.File...)
	}

	reg, err := protodesc.NewFiles(&fds)
	if err != nil {
		return protosetMethodSource{}, fmt.Errorf("create file registry: %w", err)
	}

	return protosetMethodSource{reg: reg}, nil
}

func (p protosetMethodSource) Methods() ([]protoreflect.MethodDescriptor, error) {
	var mds []protoreflect.MethodDescriptor
	p.reg.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		svcs := fd.Services()
		for i, l := 0, svcs.Len(); i < l; i++ {
			methods := svcs.Get(i).Methods()
			for i, l := 0, methods.Len(); i < l; i++ {
				mds = append(mds, methods.Get(i))
			}
		}

		return true
	})

	return mds, nil
}

func (p protosetMethodSource) Method(name protoreflect.FullName) (protoreflect.MethodDescriptor, error) {
	d, err := p.reg.FindDescriptorByName(name)
	if err != nil {
		return nil, err
	}

	return d.(protoreflect.MethodDescriptor), nil
}

func (p protosetMethodSource) Close() error {
	return nil
}

