package client

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	tracing "tracing/proto"

	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	opentracing "github.com/opentracing/opentracing-go"

	"github.com/opentracing/opentracing-go/ext"
	jaegerClientConfig "github.com/uber/jaeger-client-go/config"

	"github.com/opentracing/opentracing-go/log"
)

var (
	gRPCComponentTag = opentracing.Tag{string(ext.Component), "gRPC"}
)

type metadataReaderWriter struct {
	metadata.MD
}

// JaegerSamplerParam - 取樣所有追蹤（不能再online環境使用 -
const JaegerSamplerParam = 1

// JaegerReportingHost -
const JaegerReportingHost = "127.0.0.1:6831"

// Client -
func Client() {
	var tracer opentracing.Tracer

	cfg := jaegerClientConfig.Configuration{
		//Sampler: &jaegerClientConfig.SamplerConfig{
		//Type:  "const",
		//Param: 1.0, // sample all traces
		//},
		//Reporter: &jaegerClientConfig.ReporterConfig{
		//LogSpans:           true,
		//LocalAgentHostPort: "127.0.0.1:6831",
		//},
	}

	tracer, closer, err := cfg.New("tracing-test")
	if err != nil {
		panic(fmt.Sprintf("ERROR: cannot init Jaeger: %v\n", err))
	}

	defer closer.Close()

	conn, err := grpc.Dial(
		"localhost:50051",
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(otgrpc.OpenTracingClientInterceptor(tracer)),
		//grpc.WithUnaryInterceptor(O(tracer)),
	)
	if err != nil {
		panic(err)
	}

	defer conn.Close()

	c := tracing.NewHelloServiceClient(conn)

	for i := 0; i < 9999; i++ {
		time.Sleep(time.Second * 1)
		c.Pin(context.Background(), &tracing.Request{Id: "123"})
	}
}

func O(tracer opentracing.Tracer) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, resp interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		var err error
		var parentCtx opentracing.SpanContext
		if parent := opentracing.SpanFromContext(ctx); parent != nil {
			parentCtx = parent.Context()
		}

		clientSpan := tracer.StartSpan(
			method,
			opentracing.ChildOf(parentCtx),
			//ext.SpanKindRPCClient,
			//gRPCComponentTag,
			//opentracing.Tag{
			//Key:   "test~",
			//Value: "gRPC~~~",
			//},
		)

		defer clientSpan.Finish()

		ctx = injectSpanContext(ctx, tracer, clientSpan)

		err = invoker(ctx, method, req, resp, cc, opts...)

		return err
	}
}

func injectSpanContext(ctx context.Context, tracer opentracing.Tracer, clientSpan opentracing.Span) context.Context {
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	} else {
		md = md.Copy()
	}
	mdWriter := metadataReaderWriter{md}
	err := tracer.Inject(clientSpan.Context(), opentracing.HTTPHeaders, mdWriter)
	// We have no better place to record an error than the Span itself :-/
	if err != nil {
		clientSpan.LogFields(log.String("event", "Tracer.Inject() failed"), log.Error(err))
	}
	return metadata.NewOutgoingContext(ctx, md)
}

// Operator - 操作者資訊
type Operator struct {
	// Name - 操作者姓名
	Name string `json:"name"`
	// Account - 帳號
	Account string `json:"account"`
	// Identifier - 身份類型
	Identifier int32 `json:"identifier"`
	// Time - 操作時間
	Time time.Time `json:"time"`
}
