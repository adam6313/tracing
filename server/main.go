package main

import (
	"context"
	"fmt"
	"log"
	"net"
	tracing "tracing/proto"
	"tracing/tracer"

	"github.com/davecgh/go-spew/spew"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/codes"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	grpc_codes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"encoding/json"
	"strings"

	"google.golang.org/grpc/peer"

	"go.opentelemetry.io/contrib"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/semconv"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

func main() {
	fmt.Println("starting gRPC server...")

	tp, err := tracer.TracerProvider("http://localhost:14268/api/traces")
	if err != nil {
		panic(err)
	}

	lis, err := net.Listen("tcp", "localhost:50051")
	if err != nil {
		log.Fatalf("failed to listen: %v \n", err)
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(UnaryServerInterceptor(tp)),
	)

	tracing.RegisterHelloServiceServer(grpcServer, new(server))

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v \n", err)
	}
}

type server struct{}

func (s *server) Pin(ctx context.Context, in *tracing.Request) (*tracing.Response, error) {
	spew.Dump(in.GetId())

	return &tracing.Response{
		Id: in.GetId(),
	}, nil
}

func UnaryServerInterceptor(tp *tracesdk.TracerProvider, opts ...Option) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		requestMetadata, _ := metadata.FromOutgoingContext(ctx)
		metadataCopy := requestMetadata.Copy()

		entries, spanCtx := Extract(ctx, &metadataCopy, opts...)
		ctx = baggage.ContextWithValues(ctx, entries...)

		name, attr := spanInfo(info.FullMethod, peerFromCtx(ctx), req)

		tr := otel.Tracer("ex.com/webserver")
		ctx, span := tr.Start(
			trace.ContextWithRemoteSpanContext(ctx, spanCtx),
			name,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(attr...),
		)
		defer span.End()

		resp, err := handler(ctx, req)
		if err != nil {
			s, _ := status.FromError(err)
			span.SetStatus(codes.Error, s.Message())
			span.SetAttributes(statusCodeAttr(s.Code()))
		} else {
			span.SetAttributes(statusCodeAttr(grpc_codes.OK))
		}

		return resp, err
	}
}

const (
	defaultTracerName = "go.opentelemetry.io/contrib/instrumentation/github.com/Shopify/sarama/otelsarama"

	kafkaPartitionKey = attribute.Key("messaging.kafka.partition")
)

func UnaryClientInterceptor(tp *tracesdk.TracerProvider, opts ...Option) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, resp interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, callOpts ...grpc.CallOption) error {
		requestMetadata, _ := metadata.FromOutgoingContext(ctx)
		metadataCopy := requestMetadata.Copy()

		name, attr := spanInfo(method, cc.Target(), req)
		tr := otel.Tracer("ex.com/webserver")
		var span trace.Span
		ctx, span = tr.Start(
			ctx,
			name,
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithAttributes(attr...),
		)

		// set TraceID
		span.SetAttributes(attribute.KeyValue{
			Key:   attribute.Key("TraceID"),
			Value: attribute.StringValue(span.SpanContext().TraceID().String()),
		})
		defer span.End()

		Inject(ctx, &metadataCopy, opts...)
		ctx = metadata.NewOutgoingContext(ctx, metadataCopy)

		err := invoker(ctx, method, req, resp, cc, callOpts...)

		if err != nil {
			s, _ := status.FromError(err)
			span.SetStatus(codes.Error, s.Message())
			span.SetAttributes(statusCodeAttr(s.Code()))

			return err
		}

		span.SetAttributes(statusCodeAttr(grpc_codes.OK))

		return nil
	}
}

// UnaryServerInterceptor -
//func UnaryServerInterceptor(tp *tracesdk.TracerProvider, opts ...Option) grpc.UnaryServerInterceptor {
//return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {

//requestMetadata, _ := metadata.FromIncomingContext(ctx)
//metadataCopy := requestMetadata.Copy()

//entries, spanCtx := Extract(ctx, &metadataCopy, opts...)
//ctx = baggage.ContextWithValues(ctx, entries...)

//tr := tp.Tracer(info.FullMethod)
//name, attr := spanInfo(info.FullMethod, peerFromCtx(ctx), req)

//ctx, span := tr.Start(
//trace.ContextWithRemoteSpanContext(ctx, spanCtx),
//name,
//trace.WithSpanKind(trace.SpanKindServer),
//trace.WithAttributes(attr...),
//)
//defer span.End()

//resp, err := handler(ctx, req)
//if err != nil {
//s, _ := status.FromError(err)
//span.SetStatus(codes.Error, s.Message())
//span.SetAttributes(statusCodeAttr(s.Code()))
//} else {
//span.SetAttributes(statusCodeAttr(grpc_codes.OK))
//}

//return resp, err

//}
//}

func spanInfo(fullMethod, peerAddress string, req interface{}) (string, []attribute.KeyValue) {
	attrs := []attribute.KeyValue{semconv.RPCSystemGRPC}
	name, mAttrs := parseFullMethod(fullMethod)
	attrs = append(attrs, mAttrs...)
	attrs = append(attrs, peerAttr(peerAddress)...)

	// requestMetadata
	j, _ := json.Marshal(req)
	attrs = append(attrs, attribute.KeyValue{
		Key:   attribute.Key("request"),
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

func peerFromCtx(ctx context.Context) string {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return ""
	}
	return p.Addr.String()
}
