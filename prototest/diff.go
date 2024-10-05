package prototest

import (
	"github.com/google/go-cmp/cmp"
	"github.com/pentops/flowtest"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

func AssertEqualProto(t flowtest.TB, want, got proto.Message) {
	t.Helper()
	diff := cmp.Diff(want, got, protocmp.Transform())
	if diff != "" {
		t.Error(diff)
	}
}
