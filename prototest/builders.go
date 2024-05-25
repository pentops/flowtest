package prototest

import (
	"fmt"

	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func BuildHTTPMethod(name string, rule *annotations.HttpRule) *descriptorpb.MethodDescriptorProto {
	mm := &descriptorpb.MethodDescriptorProto{
		Name:       proto.String(name),
		InputType:  proto.String(fmt.Sprintf("%sRequest", name)),
		OutputType: proto.String(fmt.Sprintf("%sResponse", name)),
		Options:    &descriptorpb.MethodOptions{},
	}
	proto.SetExtension(mm.Options, annotations.E_Http, rule)
	return mm
}
