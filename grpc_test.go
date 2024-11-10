package flowtest

import (
	"context"
	"testing"

	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/reflection/grpc_reflection_v1"
)

func TestGrpcPair(t *testing.T) {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pair := NewGRPCPair(t)
	pair.ServeUntilDone(t, ctx)

	reflection.Register(pair.Server)

	reflectionClient := grpc_reflection_v1.NewServerReflectionClient(pair.Client)

	// Test reflection
	resp, err := reflectionClient.ServerReflectionInfo(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("resp: %v", resp)

}
