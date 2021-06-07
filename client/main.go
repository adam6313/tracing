package main

import (
	"context"
	tracing "tracing/proto"
	"tracing/tracer"

	"github.com/kataras/iris/v12"
	"go.opentelemetry.io/contrib"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	defaultTracerName = "go.opentelemetry.io/contrib/instrumentation/github.com/Shopify/sarama/otelsarama"

	kafkaPartitionKey = attribute.Key("messaging.kafka.partition")
)

type metadataSupplier struct {
	metadata *metadata.MD
}

func (s *metadataSupplier) Get(key string) string {
	values := s.metadata.Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (s *metadataSupplier) Set(key string, value string) {
	s.metadata.Set(key, value)
}

func (s *metadataSupplier) Keys() []string {
	out := make([]string, 0, len(*s.metadata))
	for key := range *s.metadata {
		out = append(out, key)
	}
	return out
}

var client tracing.HelloServiceClient

func main() {
	app := iris.Default()

	tp, err := tracer.TracerProvider("http://localhost:14268/api/traces")
	if err != nil {
		panic(err)
	}

	app.Use(ClientInterceptor(tp))

	app.Get("/ping", Ping)

	cc, err := grpc.Dial(
		"localhost:50051",
		grpc.WithInsecure(),
	)

	client = tracing.NewHelloServiceClient(cc)

	app.Run(iris.Addr(":3030"))
}

// Ping -
func Ping(ctx iris.Context) {

	_, err := client.Pin(ctx.Request().Context(), &tracing.Request{Id: "123Adam"})
	if err != nil {
		panic(err)
	}

	//go func(c context.Context) {
	////tpp, err := tracer.TracerProvider("http://localhost:14268/api/traces")
	////if err != nil {
	////panic(err)
	////}

	//tr := otel.Tracer("ex.com/webserver")

	//newCtx, span := tr.Start(c, "Sub operation...")
	//ctx.ResetRequest(ctx.Request().WithContext(newCtx))

	//defer span.End()

	//span.AddEvent("Sub span event adkjasklfjsdklfjl;ajfl;ak")
	//}(ctx.Request().Context())

	//time.Sleep(time.Second * 1)

	ctx.JSON(iris.Map{"response": "pong"})

}

// ClientInterceptor -
func ClientInterceptor(tp *tracesdk.TracerProvider) func(ctx iris.Context) {
	return func(ctx iris.Context) {
		req := ctx.Request()

		//requestMetadata, _ := metadata.FromOutgoingContext(req.Context())
		//metadataCopy := requestMetadata.Copy()

		tr := otel.Tracer("ex.com/webserver")
		newCtx, span := tr.Start(
			req.Context(),
			req.Host,
			trace.WithSpanKind(trace.SpanKindClient),
		)

		//req = req.WithContext(newCtx)
		ctx.ResetRequest(ctx.Request().WithContext(newCtx))

		span.AddEvent("Nice operation!", trace.WithAttributes(attribute.Int("bogons", 100)))

		defer span.End()

		span.SetAttributes([]attribute.KeyValue{
			{
				Key:   attribute.Key("url"),
				Value: attribute.StringValue(ctx.RemoteAddr()),
			},
			{
				Key:   attribute.Key("method"),
				Value: attribute.StringValue(req.Method),
			},
			{
				Key:   attribute.Key("TraceID"),
				Value: attribute.StringValue(span.SpanContext().TraceID().String()),
			},
			{
				Key:   attribute.Key("statusCode"),
				Value: attribute.IntValue(ctx.GetStatusCode()),
			},
		}...)

		//Inject(req.Context(), &metadataCopy)
		//newCtx = metadata.NewOutgoingContext(req.Context(), metadataCopy)

		//ctx.ResetRequest(ctx.Request().WithContext(newCtx))

		//func(ctx context.Context) {
		//tpp, err := tracer.TracerProvider("http://localhost:14268/api/traces")
		//if err != nil {
		//panic(err)
		//}

		//tr := tpp.Tracer("helloAdam123456")
		//var span trace.Span
		//ctx, span = tr.Start(ctx, "Sub operation...")
		//defer span.End()

		//span.AddEvent("Sub span event adkjasklfjsdklfjl;ajfl;ak")
		//}(ctx.Request().Context())

		ctx.Next()
	}
}

type Option func(*config)

type config struct {
	TracerProvider trace.TracerProvider
	Propagators    propagation.TextMapPropagator

	Tracer trace.Tracer
}

func Inject(ctx context.Context, metadata *metadata.MD, opts ...Option) {
	c := newConfig(opts...)
	c.Propagators.Inject(ctx, &metadataSupplier{
		metadata: metadata,
	})
}

func newConfig(opts ...Option) config {
	cfg := config{
		Propagators:    otel.GetTextMapPropagator(),
		TracerProvider: otel.GetTracerProvider(),
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	cfg.Tracer = cfg.TracerProvider.Tracer(
		defaultTracerName,
		trace.WithInstrumentationVersion(contrib.SemVersion()),
	)

	return cfg
}
