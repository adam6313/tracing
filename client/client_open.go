package client

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	tracing "tracing/proto"
	"tracing/tracer"

	"go.opentelemetry.io/contrib"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/semconv"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/otel/attribute"
	grpc_codes "google.golang.org/grpc/codes"
)

const (
	defaultTracerName = "go.opentelemetry.io/contrib/instrumentation/github.com/Shopify/sarama/otelsarama"

	kafkaPartitionKey = attribute.Key("messaging.kafka.partition")
)

type metadataSupplier struct {
	metadata *metadata.MD
}

// assert that metadataSupplier implements the TextMapCarrier interface
var _ propagation.TextMapCarrier = &metadataSupplier{}

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

// ClientOpne -
func ClientOpne() {

	tp, err := tracer.TracerProvider("http://localhost:14268/api/traces")
	if err != nil {
		panic(err)
	}

	cc, err := grpc.Dial(
		"localhost:50051",
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(
			UnaryServerInterceptor(tp),
		),
	)

	if err != nil {
		log.Fatalf("Error connecting: %v", err)
	}
	defer cc.Close()

	c := tracing.NewHelloServiceClient(cc)

	for i := 0; i < 9999; i++ {
		time.Sleep(time.Second * 1)
		c.Pin(context.Background(), &tracing.Request{Id: "123"})
	}

}

func UnaryServerInterceptor(tp *tracesdk.TracerProvider, opts ...Option) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, resp interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		callOpts ...grpc.CallOption) error {

		requestMetadata, _ := metadata.FromOutgoingContext(ctx)
		metadataCopy := requestMetadata.Copy()

		name, attr := spanInfo(method, cc.Target(), req)
		tr := tp.Tracer(method)
		var span trace.Span
		ctx, span = tr.Start(
			ctx,
			name,
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithAttributes(attr...),
		)
		defer span.End()

		Inject(ctx, &metadataCopy, opts...)
		ctx = metadata.NewOutgoingContext(ctx, metadataCopy)

		err := invoker(ctx, method, req, resp, cc, callOpts...)

		if err != nil {
			s, _ := status.FromError(err)
			span.SetStatus(codes.Error, s.Message())
			span.SetAttributes(statusCodeAttr(s.Code()))
		} else {
			span.SetAttributes(statusCodeAttr(grpc_codes.OK))
		}

		return err
	}
}

func spanInfo(fullMethod, peerAddress string, req interface{}) (string, []attribute.KeyValue) {
	attrs := []attribute.KeyValue{semconv.RPCSystemGRPC}
	name, mAttrs := parseFullMethod(fullMethod)
	attrs = append(attrs, mAttrs...)
	attrs = append(attrs, peerAttr(peerAddress)...)

	// req
	j, _ := json.Marshal(req)
	attrs = append(attrs, attribute.KeyValue{
		Key:   attribute.Key("require"),
		Value: attribute.StringValue(string(j)),
	})

	return name, attrs
}

func parseFullMethod(fullMethod string) (string, []attribute.KeyValue) {
	name := strings.TrimLeft(fullMethod, "/")
	parts := strings.SplitN(name, "/", 2)
	if len(parts) != 2 {
		// Invalid format, does not follow `/package.service/method`.
		return name, []attribute.KeyValue(nil)
	}

	var attrs []attribute.KeyValue
	if service := parts[0]; service != "" {
		attrs = append(attrs, semconv.RPCServiceKey.String(service))
	}
	if method := parts[1]; method != "" {
		attrs = append(attrs, semconv.RPCMethodKey.String(method))
	}
	return name, attrs
}

func peerAttr(addr string) []attribute.KeyValue {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return []attribute.KeyValue(nil)
	}

	if host == "" {
		host = "127.0.0.1"
	}

	return []attribute.KeyValue{
		semconv.NetPeerIPKey.String(host),
		semconv.NetPeerPortKey.String(port),
	}
}

func Inject(ctx context.Context, metadata *metadata.MD, opts ...Option) {
	c := newConfig(opts...)
	c.Propagators.Inject(ctx, &metadataSupplier{
		metadata: metadata,
	})
}

type config struct {
	TracerProvider trace.TracerProvider
	Propagators    propagation.TextMapPropagator

	Tracer trace.Tracer
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

// Option specifies instrumentation configuration options.
type Option func(*config)

// WithTracerProvider specifies a tracer provider to use for creating a tracer.
// If none is specified, the global provider is used.
func WithTracerProvider(provider trace.TracerProvider) Option {
	return func(cfg *config) {
		cfg.TracerProvider = provider
	}
}

// WithPropagators specifies propagators to use for extracting
// information from the HTTP requests. If none are specified, global
// ones will be used.
func WithPropagators(propagators propagation.TextMapPropagator) Option {
	return func(cfg *config) {
		cfg.Propagators = propagators
	}
}

func Extract(ctx context.Context, metadata *metadata.MD, opts ...Option) ([]attribute.KeyValue, trace.SpanContext) {
	c := newConfig(opts...)
	ctx = c.Propagators.Extract(ctx, &metadataSupplier{
		metadata: metadata,
	})

	attributeSet := baggage.Set(ctx)

	return (&attributeSet).ToSlice(), trace.SpanContextFromContext(ctx)
}

func statusCodeAttr(c grpc_codes.Code) attribute.KeyValue {
	return attribute.KeyValue{
		Key:   attribute.Key("statusCode"),
		Value: attribute.Int64Value(int64(c)),
	}
}
