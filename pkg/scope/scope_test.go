package scope_test

import (
	"context"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/gopherex/xtrace/pkg/scope"
)

func TestScopeRunUsesProvider(t *testing.T) {
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	s := scope.New("component", scope.WithTracerProvider(tp))

	err := s.Run(context.Background(), "op", func(context.Context, trace.Span) error {
		return nil
	})

	if err != nil {
		t.Fatalf("err = %v", err)
	}
	spans := tracetest.SpanStubsFromReadOnlySpans(rec.Ended())
	if len(spans) != 1 {
		t.Fatalf("ended spans = %d", len(spans))
	}
	if spans[0].InstrumentationScope.Name != "component" {
		t.Fatalf("scope = %q, want component", spans[0].InstrumentationScope.Name)
	}
}
