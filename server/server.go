package server

import (
	"context"
	"fmt"
	"log"
	"net"
	tracing "tracing/proto"

	"github.com/davecgh/go-spew/spew"
	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	"github.com/opentracing/opentracing-go"
	jaegerClientConfig "github.com/uber/jaeger-client-go/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type server struct{}

func (s *server) Pin(ctx context.Context, in *tracing.Request) (*tracing.Response, error) {
	spew.Dump(in.GetId())
	md, _ := metadata.FromOutgoingContext(ctx)

	spew.Dump(md)
	return &tracing.Response{
		Id: in.GetId(),
	}, nil
}

// Run -
func Run() {

	var tracer opentracing.Tracer

	cfg := jaegerClientConfig.Configuration{
		Sampler: &jaegerClientConfig.SamplerConfig{
			Type:  "const",
			Param: 1.0, // sample all traces
		},
		Reporter: &jaegerClientConfig.ReporterConfig{
			LogSpans:           true,
			LocalAgentHostPort: "127.0.0.1:6831",
		}}

	tracer, closer, err := cfg.New("tracing-test")
	if err != nil {
		panic(fmt.Sprintf("ERROR: cannot init Jaeger: %v\n", err))
	}

	defer closer.Close()

	fmt.Println("starting gRPC server...")

	lis, err := net.Listen("tcp", "localhost:50051")
	if err != nil {
		log.Fatalf("failed to listen: %v \n", err)
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(otgrpc.OpenTracingServerInterceptor(tracer)),
	)

	tracing.RegisterHelloServiceServer(grpcServer, new(server))

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v \n", err)
	}
}
