package server

import (
	"context"
	"fmt"
	"log"
	"net"
	tracing "tracing/proto"

	"github.com/davecgh/go-spew/spew"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

type server struct{}

func (s *server) Pin(ctx context.Context, in *tracing.Request) (*tracing.Response, error) {
	spew.Dump(in.GetId())

	return &tracing.Response{
		Id: in.GetId(),
	}, nil
}

// Run -
func Run() {
	fmt.Println("starting gRPC server...")

	lis, err := net.Listen("tcp", "localhost:50051")
	if err != nil {
		log.Fatalf("failed to listen: %v \n", err)
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor()),
	)

	tracing.RegisterHelloServiceServer(grpcServer, new(server))

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v \n", err)
	}
}
