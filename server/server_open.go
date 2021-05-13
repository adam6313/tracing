package server

import (
	"fmt"
	"log"
	"net"
	tracing "tracing/proto"

	"google.golang.org/grpc"
)

// Run -
func RunOpen() {

	//tp, err := tracer.TracerProvider("http://localhost:14268/api/traces")
	//if err != nil {
	//panic(err)
	//}

	fmt.Println("starting gRPC server...")

	lis, err := net.Listen("tcp", "localhost:50051")
	if err != nil {
		log.Fatalf("failed to listen: %v \n", err)
	}

	grpcServer := grpc.NewServer()

	tracing.RegisterHelloServiceServer(grpcServer, new(server))

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v \n", err)
	}
}
