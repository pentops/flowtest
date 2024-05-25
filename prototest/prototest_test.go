package prototest

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
)

func TestSingletonDescriptor(t *testing.T) {

	// When parsing the entire file set, the descriptors for imported types are
	// equivalent but not equal to the descriptors for the same types in the
	// global registry.

	// This test ensures, through the string wrapper as an example, that a
	// directly constructed wrapperspb.StringValue is the same as one constructed
	// via the reflection on the on-the-fly parsed descriptor.

	runTest := func(t *testing.T, msgDesc protoreflect.MessageDescriptor) {
		stringFieldDesc := msgDesc.Fields().ByName("string")

		wrapperVal := wrapperspb.String("value")

		stringValDynamic := dynamicpb.NewMessage(stringFieldDesc.Message())
		stringValDynamic.Set(stringValDynamic.Descriptor().Fields().ByName("value"), protoreflect.ValueOfString("value"))

		if !proto.Equal(stringValDynamic, wrapperVal) {
			t.Errorf("dynamic and concrete not equal")
		}

		if stringValDynamic.ProtoReflect().Descriptor() != wrapperVal.ProtoReflect().Descriptor() {
			t.Fatal("descriptors of the two values not equal")
		}
	}

	t.Run("target", func(t *testing.T) {
		// This test sets the target baseline, proving the test is possible
		// (which of course it is now that it passes, but that wasn't always as obvious)
		fileDesc, err := protodesc.NewFile(&descriptorpb.FileDescriptorProto{
			Name:    proto.String("test.proto"),
			Package: proto.String("test"),
			Syntax:  proto.String("proto3"),
			Dependency: []string{
				"google/protobuf/wrappers.proto",
			},
			MessageType: []*descriptorpb.DescriptorProto{{
				Name: proto.String("Wrapper"),
				Field: []*descriptorpb.FieldDescriptorProto{{
					Name:     proto.String("string"),
					Number:   proto.Int32(1),
					Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
					TypeName: proto.String(".google.protobuf.StringValue"),
				}},
			}},
		}, protoregistry.GlobalFiles)

		if err != nil {
			t.Fatal(err)
		}

		msgDesc := fileDesc.Messages().ByName("Wrapper")
		if msgDesc == nil {
			t.Fatal("no foo message")
		}
		runTest(t, msgDesc)

	})

	t.Run("using library", func(t *testing.T) {

		rs := DescriptorsFromSource(t, map[string]string{
			"test.proto": `
		syntax = "proto3";

		package test;

		import "google/protobuf/wrappers.proto";

		message Wrapper {
			google.protobuf.StringValue string = 1;
		}
		`,
		})
		msgDesc := rs.MessageByName(t, "test.Wrapper")
		runTest(t, msgDesc)

	})
}

func TestAnnotations(t *testing.T) {

	set := DescriptorsFromSource(t, map[string]string{

		"test.proto": `
		syntax = "proto3";

		package test;

		import "buf/validate/validate.proto";

		message Foo {
			option (buf.validate.message).disabled = true;

			string field1 = 1;
		}

		message Bar {
			option (buf.validate.message) = {
				disabled: true
			};
		}
		`,
	})

	for _, name := range []string{"Foo", "Bar"} {

		msgDesc := set.MessageByName(t, protoreflect.FullName("test."+name))
		if msgDesc == nil {
			t.Fatalf("no %s message", name)
		}

		opts := proto.GetExtension(msgDesc.Options(), validate.E_Message).(*validate.MessageConstraints)
		if opts == nil {
			t.Fatalf("no %s options", name)
		}
		if opts.Disabled == nil || !*opts.Disabled {
			t.Fatalf("option not set in %s", name)
		}
	}

}

func TestParserErrors(t *testing.T) {

	t.Run("syntax error", func(t *testing.T) {

		_, err := TryDescriptorsFromSource(map[string]string{

			"test.proto": `
		syntax = "proto3";

		package test;

		message Foo {
			syntax-error
		}
		`,
		})

		if !strings.Contains(err.Error(), "syntax error") {
			t.Fatal("expected syntax error")
		}
		if err == nil {
			t.Fatal("expected error")
		}

	})

	t.Run("missing option", func(t *testing.T) {

		_, err := TryDescriptorsFromSource(map[string]string{
			"test.proto": `
		syntax = "proto3";

		package test;

		message Foo {
			option (foo.bar).thing = 1;
		}
		`,
		})
		if err == nil {
			t.Fatal("expected error")
		}

		if !strings.Contains(err.Error(), "foo.bar") {
			t.Fatalf("expected missing option error, got %s", err.Error())
		}

	})

}
