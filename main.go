package main

import (
	"context"
	"time"
	"tracing/client"
	"tracing/server"

	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/semconv"

	"go.opentelemetry.io/otel/exporters/stdout"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

const (
	service     = "trace-demo2"
	environment = "production"
	id          = 1
)

func main() {

	go server.RunOpen()

	time.Sleep(time.Second * 1)

	client.ClientOpne()

	select {}

	//tp, err := tracer.TracerProvider("http://localhost:14268/api/traces")
	//if err != nil {
	//log.Fatal(err)
	//}

	//// Register our TracerProvider as the global so any imported
	//// instrumentation in the future will default to using it.
	//otel.SetTracerProvider(tp)

	//ctx, cancel := context.WithCancel(context.Background())
	//defer cancel()

	//// Cleanly shutdown and flush telemetry when the application exits.
	//defer func(ctx context.Context) {
	//// Do not make the application hang when it is shutdown.
	//ctx, cancel = context.WithTimeout(ctx, time.Second*5)
	//defer cancel()
	//if err := tp.Shutdown(ctx); err != nil {
	//log.Fatal(err)
	//}
	//}(ctx)

	//tr := tp.Tracer("component-main")

	//ctx, span := tr.Start(ctx, "foo")
	//defer span.End()

	//bar(ctx)

	//select {}
}

func bar(ctx context.Context) {
	// Use the global TracerProvider.
	tr := otel.Tracer("component-bar")
	_, span := tr.Start(ctx, "bar")
	span.SetAttributes(attribute.Key("testset").String("value"))
	defer span.End()

	pong(ctx)
}

func pong(ctx context.Context) {
	// Use the global TracerProvider.
	tr := otel.Tracer("component-pong")
	_, span := tr.Start(ctx, "pong")
	span.SetAttributes(attribute.Key("testset-pong").String("value-pong"))
	defer span.End()
}

func Init() {
	exporter, err := stdout.NewExporter(stdout.WithPrettyPrint())
	if err != nil {
		log.Fatal(err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSyncer(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.ServiceNameKey.String(service),
			attribute.String("environment", environment),
			attribute.Int64("ID", id),
		)),
	)
	if err != nil {
		log.Fatal(err)
	}
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
}
