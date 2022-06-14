package main

import "google.golang.org/protobuf/reflect/protoreflect"

type methodSource interface {
	Methods() ([]protoreflect.MethodDescriptor, error)
	Method(protoreflect.FullName) (protoreflect.MethodDescriptor, error)
	Close() error
}
