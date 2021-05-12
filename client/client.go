package client

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"

	tracing "tracing/proto"

	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	"github.com/opentracing/opentracing-go"
	jaegerClientConfig "github.com/uber/jaeger-client-go/config"
)

// JaegerSamplerParam - 取樣所有追蹤（不能再online環境使用 -
const JaegerSamplerParam = 1

// JaegerReportingHost -
const JaegerReportingHost = "127.0.0.1:6831"

// Client -
func Client() {
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

	conn, err := grpc.Dial(
		"localhost:50051",
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(otgrpc.OpenTracingClientInterceptor(tracer)),
	)
	if err != nil {
		log.Fatalf("failed to dial: %v", err)
	}

	defer conn.Close()

	c := tracing.NewHelloServiceClient(conn)

	for i := 0; i < 9999; i++ {
		time.Sleep(time.Second * 1)
		c.Pin(context.Background(), &tracing.Request{Id: "123"})
	}

}
