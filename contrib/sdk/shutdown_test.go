package sdk

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	logglobal "go.opentelemetry.io/otel/log/global"
)

// hung is an OTLP collector that takes every export and never answers;
// it records the paths it was sent to.
type hung struct {
	mu      sync.Mutex
	paths   map[string]bool
	release chan struct{} // closed when the test ends
}

func (h *hung) ServeHTTP(_ http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	h.paths[r.URL.Path] = true
	h.mu.Unlock()

	select {
	case <-r.Context().Done():
	case <-h.release:
	}
}

func (h *hung) seen() map[string]bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	out := make(map[string]bool, len(h.paths))
	for p := range h.paths {
		out[p] = true
	}

	return out
}

// The signals flush at once: with a hung collector every signal tries
// its export within the one shutdown budget, and shutdown ends with it.
// Flushed one after another, the first would hold the budget and the
// others would never try.
func TestShutdownFlushesSignalsConcurrently(t *testing.T) {
	collector := &hung{paths: map[string]bool{}, release: make(chan struct{})}
	srv := httptest.NewServer(collector)
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(collector.release) }) // before Close: it waits for the handlers

	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", srv.URL)

	prevTP, prevMP, prevLP := otel.GetTracerProvider(), otel.GetMeterProvider(), logglobal.GetLoggerProvider()
	t.Cleanup(func() {
		otel.SetTracerProvider(prevTP)
		otel.SetMeterProvider(prevMP)
		logglobal.SetLoggerProvider(prevLP)
	})

	shutdown, err := Setup(context.Background(), WithService("shutdown-test"))
	if err != nil {
		t.Fatal(err)
	}

	// Something to export on every signal.
	_, span := otel.Tracer("test").Start(context.Background(), "op")
	span.End()

	counter, err := otel.Meter("test").Int64Counter("ops")
	if err != nil {
		t.Fatal(err)
	}

	counter.Add(context.Background(), 1)

	var record log.Record
	record.SetBody(attribute.StringValue("bye"))
	logglobal.GetLoggerProvider().Logger("test").Emit(context.Background(), record)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	start := time.Now()
	err = shutdown(ctx)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("a hung collector must fail the flush")
	}

	if elapsed > 3*time.Second {
		t.Fatalf("shutdown took %v past its 1s budget", elapsed)
	}

	seen := collector.seen()
	for _, path := range []string{"/v1/traces", "/v1/metrics", "/v1/logs"} {
		if !seen[path] {
			t.Errorf("no export to %s within the budget (seen %v)", path, seen)
		}
	}
}
