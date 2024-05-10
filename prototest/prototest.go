package prototest

// Package prototest provides utilities for dynamically parsing proto files
// into reflection for test cases

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/jhump/protoreflect/desc/protoparse"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

type ResultSet struct {
	messages map[protoreflect.FullName]protoreflect.MessageDescriptor
	services map[protoreflect.FullName]protoreflect.ServiceDescriptor
}

func (rs ResultSet) MessageByName(t testing.TB, name protoreflect.FullName) protoreflect.MessageDescriptor {
	t.Helper()

	md, ok := rs.messages[name]
	if !ok {
		t.Fatalf("message not found: %s", name)
	}
	return md
}

func (rs ResultSet) ServiceByName(t testing.TB, name protoreflect.FullName) protoreflect.ServiceDescriptor {
	t.Helper()
	md, ok := rs.services[name]
	if !ok {
		t.Fatalf("service not found: %s", name)
	}
	return md
}

func DescriptorsFromSource(t testing.TB, source map[string]string) *ResultSet {
	t.Helper()
	rs, err := TryDescriptorsFromSource(source)
	if err != nil {
		t.Fatal(err)
	}
	return rs
}
func TryDescriptorsFromSource(source map[string]string) (*ResultSet, error) {

	allFiles := make([]string, 0, len(source))
	for filename := range source {
		allFiles = append(allFiles, filename)
	}

	parser := protoparse.Parser{
		ImportPaths:           []string{""},
		IncludeSourceCodeInfo: false,

		Accessor: func(filename string) (io.ReadCloser, error) {
			src, ok := source[filename]
			if !ok {
				return nil, fmt.Errorf("file not found: %s", filename)
			}
			return io.NopCloser(strings.NewReader(src)), nil
		},
	}

	customDesc, err := parser.ParseFilesButDoNotLink(allFiles...)
	if err != nil {
		return nil, err
	}

	rs := &ResultSet{
		messages: make(map[protoreflect.FullName]protoreflect.MessageDescriptor),
		services: make(map[protoreflect.FullName]protoreflect.ServiceDescriptor),
	}

	for _, file := range customDesc {
		fd, err := protodesc.NewFile(file, protoregistry.GlobalFiles)
		if err != nil {
			return nil, err
		}

		messages := fd.Messages()
		for i := 0; i < messages.Len(); i++ {
			msg := messages.Get(i)

			rs.messages[msg.FullName()] = msg

			options := msg.Options().(*descriptorpb.MessageOptions)
			if options != nil {
				if err := setUninterpretedOptions(options, options.UninterpretedOption); err != nil {
					return nil, fmt.Errorf("parsing options on %s: %w", msg.FullName(), err)
				}
			}

			fields := msg.Fields()
			for i := 0; i < fields.Len(); i++ {
				field := fields.Get(i)
				options := field.Options().(*descriptorpb.FieldOptions)
				if options != nil {
					if err := setUninterpretedOptions(options, options.UninterpretedOption); err != nil {
						return nil, fmt.Errorf("parsing field options on %s: %w", field.FullName(), err)
					}
				}
			}

		}

		services := fd.Services()
		for i := 0; i < services.Len(); i++ {
			svc := services.Get(i)
			rs.services[svc.FullName()] = svc
		}
	}

	return rs, nil
}

func setUninterpretedOptions(optionsMsg proto.Message, toParse []*descriptorpb.UninterpretedOption) error {

	seen := map[protoreflect.FullName]protoreflect.Message{}

	for _, opt := range toParse {
		name := opt.Name
		fullName := protoreflect.FullName(*name[0].NamePart)
		optionsMessage, ok := seen[fullName]
		if !ok {
			extDesc, err := protoregistry.GlobalTypes.FindExtensionByName(fullName)
			if errors.Is(err, protoregistry.NotFound) {
				return fmt.Errorf("unknown extension: %s", fullName)
			} else if err != nil {
				return fmt.Errorf("find extension: %w", err)
			}
			if extDesc == nil {
				return fmt.Errorf("not found desc: %s", extDesc)
			}
			optionsMessage = extDesc.New().Message()
			proto.SetExtension(optionsMsg, extDesc, optionsMessage.Interface())
		}

		path := name[1:]
		err := setOptionField(optionsMessage, path, opt)
		if err != nil {
			return fmt.Errorf("error walking path: %w", err)
		}
	}
	return nil
}

func setOptionField(msg protoreflect.Message, path []*descriptorpb.UninterpretedOption_NamePart, opt *descriptorpb.UninterpretedOption) error {
	if len(path) == 0 {
		if opt.AggregateValue == nil {
			return fmt.Errorf("no aggregate value, but the option is a message")
		}
		if err := prototext.Unmarshal([]byte(*opt.AggregateValue), msg.Interface()); err != nil {
			return err
		}
		return nil
	}

	fieldName := protoreflect.Name(*path[0].NamePart)

	field := msg.Descriptor().Fields().ByName(fieldName)
	if field == nil {
		return fmt.Errorf("field not found: %s", fieldName)
	}

	if len(path) > 1 {
		if field.Kind() != protoreflect.MessageKind {
			return fmt.Errorf("field is not a message: %s", fieldName)
		}

		fieldValue := msg.Mutable(field).Message()
		if err := setOptionField(fieldValue, path[1:], opt); err != nil {
			return fmt.Errorf("%s: %w", fieldName, err)
		}
	}

	// This is the last element. It is either going to be a message, or a
	// field scalar.

	// Message is parsed from the textproto
	if field.Kind() == protoreflect.MessageKind {
		fieldValue := msg.Mutable(field).Message()
		return setOptionField(fieldValue, path[1:], opt)
	}

	// Field is a scalar, warp it in textproto... This is clearly a hack.
	var srcText string

	if opt.IdentifierValue != nil {
		srcText = *opt.IdentifierValue
	} else if opt.NegativeIntValue != nil {
		srcText = fmt.Sprintf("-%d", *opt.NegativeIntValue)
	} else if opt.PositiveIntValue != nil {
		srcText = fmt.Sprintf("%d", *opt.PositiveIntValue)
	} else {
		return fmt.Errorf("no identifier value for %s but the option is a scalar (%s)", fieldName, field.Kind())
	}

	textprotoValue := fmt.Sprintf("%s: %s", *path[0].NamePart, srcText)
	if err := prototext.Unmarshal([]byte(textprotoValue), msg.Interface()); err != nil {
		return err
	}

	return nil

}

type MessageOption func(*messageOption)

type messageOption struct {
	name    string
	imports []string
}

func WithMessageName(name string) MessageOption {
	return func(o *messageOption) {
		o.name = name
	}
}

func WithMessageImports(imports ...string) MessageOption {
	return func(o *messageOption) {
		o.imports = append(o.imports, imports...)
	}
}

func SingleMessage(t testing.TB, content ...interface{}) protoreflect.MessageDescriptor {
	t.Helper()
	options := &messageOption{
		name: "Wrapper",
	}
	lines := make([]string, 0, len(content))
	for _, c := range content {
		if opt, ok := c.(MessageOption); ok {
			opt(options)
			continue
		}
		if str, ok := c.(string); ok {
			lines = append(lines, str)
			continue
		}
		t.Fatalf("unknown content type: %T", c)
	}

	importLines := make([]string, 0, len(options.imports))
	for _, imp := range options.imports {
		importLines = append(importLines, fmt.Sprintf(`import "%s";`, imp))
	}

	rs := DescriptorsFromSource(t, map[string]string{
		"test.proto": fmt.Sprintf(`
		syntax = "proto3";
		%s
		package test;
		message %s {
			%s
		}
		`, strings.Join(importLines, "\n"), options.name, strings.Join(lines, "\n")),
	})

	return rs.MessageByName(t, protoreflect.FullName("test."+options.name))
}
