package flowtest

import (
	"context"
	"net"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func NewGRPCPair(t TB, middleware ...grpc.UnaryServerInterceptor) *GRPCPair {
	lis := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(middleware...)),
	)

	conn, err := grpc.DialContext(context.Background(), "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}

	return &GRPCPair{
		Server:   grpcServer,
		Client:   conn,
		listener: lis,
	}
}

type GRPCPair struct {
	Server   *grpc.Server
	Client   *grpc.ClientConn
	listener *bufconn.Listener
}

func (gg *GRPCPair) ServeUntilDone(t TB, ctx context.Context) {
	go func() {
		if err := gg.Server.Serve(gg.listener); err != nil {
			t.Logf("grpc server exited: %s", err)
		}
	}()
	go func() {
		<-ctx.Done()
		t.Log("stopping grpc server")
		gg.Server.GracefulStop()
		gg.Client.Close()
		gg.listener.Close()
	}()
}
