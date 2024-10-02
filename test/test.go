package test

import (
	"context"
	"errors"
	"fmt"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"
)

func Run() (err error) {
	// Handle SIGINT (CTRL+C) gracefully.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Set up OpenTelemetry.
	otelShutdown, err := setupOTelSDK(ctx)
	if err != nil {
		return
	}
	// Handle shutdown properly so nothing leaks.
	defer func() {
		err = errors.Join(err, otelShutdown(context.Background()))
	}()

	// Start HTTP server.
	srv := &http.Server{
		Addr:         ":8080",
		BaseContext:  func(_ net.Listener) context.Context { return ctx },
		ReadTimeout:  time.Second,
		WriteTimeout: 10 * time.Second,
		Handler:      newHTTPHandler(),
	}

	srvErr := make(chan error, 1)
	go func() {
		srvErr <- srv.ListenAndServe()
	}()

	// Wait for interruption.
	select {
	case err = <-srvErr:
		// Error when starting HTTP server.
		return
	case <-ctx.Done():
		// Wait for first CTRL+C.
		// Stop receiving signal notifications as soon as possible.
		stop()
	}

	// When Shutdown is called, ListenAndServe immediately returns ErrServerClosed.
	err = srv.Shutdown(context.Background())
	return
}

func newHTTPHandler() http.Handler {
	mux := http.NewServeMux()

	// handleFunc is a replacement for mux.HandleFunc
	// which enriches the handler's HTTP instrumentation with the pattern as the http.route.
	handleFunc := func(pattern string, handlerFunc func(http.ResponseWriter, *http.Request)) {
		// Configure the "http.route" for the HTTP instrumentation.
		handler := otelhttp.WithRouteTag(pattern, http.HandlerFunc(handlerFunc))
		mux.Handle(pattern, handler)
	}

	// Register handlers.
	handleFunc("/rolldice/", rolldice)
	handleFunc("/api1", api1Handler)
	handleFunc("/api2", api2Handler)
	handleFunc("/rolldice/{player}", rolldice)

	// Add HTTP instrumentation for the whole server.
	handler := otelhttp.NewHandler(mux, "/")
	return handler
}

const name = "go.opentelemetry.io/otel/example/dice"

var (
	tracer = otel.Tracer(name)
	//meter  = otel.Meter(name)
	logger = otelslog.NewLogger(name)
	//rollCnt metric.Int64Counter
)

func init() {
	var err error
	//rollCnt, err = meter.Int64Counter("dice.rolls",
	//	metric.WithDescription("The number of rolls by roll value"),
	//	metric.WithUnit("{roll}"))
	if err != nil {
		panic(err)
	}
}

func rolldice(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "roll")
	defer span.End()

	roll := 1 + rand.Intn(6)

	var msg string
	if player := r.PathValue("player"); player != "" {
		msg = fmt.Sprintf("%s is rolling the dice", player)
	} else {
		msg = "Anonymous player is rolling the dice"
	}
	msg = msg
	logger.InfoContext(ctx, msg, "result", roll)
	//logger.Info("test")
	//rollValueAttr := attribute.Int("roll.value", roll)
	//span.SetAttributes(rollValueAttr)
	//rollCnt.Add(ctx, 1, metric.WithAttributes(rollValueAttr))

	resp := strconv.Itoa(roll) + "\n"
	if _, err := io.WriteString(w, resp); err != nil {
		log.Printf("Write failed: %v\n", err)
	}
}

func api1Handler(w http.ResponseWriter, r *http.Request) {
	tracer := otel.Tracer("api1-tracer")
	ctx, span := tracer.Start(r.Context(), "api1Handler")
	defer span.End()

	span.SetAttributes(attribute.String("event", "api1 called"))

	// Create a new HTTP client with OpenTelemetry instrumentation
	client := http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}

	// Make a request to api2
	req, err := http.NewRequestWithContext(ctx, "GET", "http://localhost:8080/api2", nil)
	if err != nil {
		span.RecordError(err)
		http.Error(w, "Failed to create request to api2", http.StatusInternalServerError)
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		span.RecordError(err)
		http.Error(w, "Failed to call api2", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	span.SetAttributes(attribute.Int("api2.status_code", resp.StatusCode))

	fmt.Fprintf(w, "api1 called api2, response status: %d\n", resp.StatusCode)
}

func api2Handler(w http.ResponseWriter, r *http.Request) {
	tracer := otel.Tracer("api2-tracer")
	_, span := tracer.Start(r.Context(), "api2Handler")
	defer span.End()

	span.SetAttributes(attribute.String("event", "api2 called"))

	// Simulate some work
	time.Sleep(100 * time.Millisecond)

	fmt.Fprintf(w, "Hello from api2!\n")
}
