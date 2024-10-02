package http

import (
	"context"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"time"

	"github.com/gorilla/mux"
	"github.com/spf13/viper"
	"github.com/stefanprodan/podinfo/pkg/version"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"
	_ "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const (
	instrumentationName = "github.com/stefanprodan/podinfo/pkg/api"
)

func (s *Server) initTracer(ctx context.Context) {
	exporter, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint("http://10.78.226.1:32097/api/traces")))
	if err != nil {
		s.logger.Error("creating OTLP trace exporter", zap.Error(err))
	}

	s.tracerProvider = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter,
			// Default is 5s. Set to 1s for demonstrative purposes.
			sdktrace.WithBatchTimeout(time.Second)),
	)
	otel.SetTracerProvider(s.tracerProvider)

	s.tracer = s.tracerProvider.Tracer(
		instrumentationName,
		trace.WithInstrumentationVersion(version.VERSION),
		trace.WithSchemaURL(semconv.SchemaURL),
	)
}

func NewOpenTelemetryMiddleware() mux.MiddlewareFunc {
	return otelmux.Middleware(viper.GetString("otel-service-name"))
}
