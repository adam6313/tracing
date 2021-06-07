package tracer

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/trace/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/semconv"
)

const (
	service     = "demo-service-adam"
	environment = "production"
)

func TracerProvider(url string) (*tracesdk.TracerProvider, error) {
	// Create the Jaeger exporter
	exp, err := jaeger.NewRawExporter(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(url)))
	if err != nil {
		return nil, err
	}

	tp := tracesdk.NewTracerProvider(
		// Always be sure to batch in production.
		tracesdk.WithBatcher(exp),
		// Record information about this application in an Resource.
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.ServiceNameKey.String(service),
			attribute.String("environment", environment),
		)),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	return tp, nil
}

// NewTracing -
//func NewTracer() {
//exporter, err := stdout.NewExporter(stdout.WithPrettyPrint())
//if err != nil {
//log.Fatal(err)
//}

//tp := tracesdk.NewTracerProvider(
//tracesdk.WithSampler(tracesdk.AlwaysSample()),
//tracesdk.WithSyncer(exporter),
//tracesdk.WithResource(resource.NewWithAttributes(
//semconv.ServiceNameKey.String(service),
//attribute.String("environment", environment),
//)),
//)
//if err != nil {
//log.Fatal(err)
//}
//otel.SetTracerProvider(tp)
//otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
//}
