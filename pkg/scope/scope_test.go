package scope_test

import (
	"context"
	"errors"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/gopherex/xtrace/pkg/scope"
	"github.com/gopherex/xtrace/pkg/span"
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

// The scope's renderer applies to every Run; a Run option overrides it.
func TestScopeErrorRenderer(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		opts []span.Option
		want string
	}{
		{name: "scope renderer", want: "scope"},
		{name: "run override", opts: []span.Option{span.WithErrorRenderer(func(error) string { return "run" })}, want: "run"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec := tracetest.NewSpanRecorder()
			tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
			s := scope.New("c", scope.WithTracerProvider(tp), scope.WithErrorRenderer(func(error) string { return "scope" }))
			_ = s.Run(context.Background(), "op", func(context.Context, trace.Span) error {
				return errors.New("token=secret")
			}, tc.opts...)
			if got := tracetest.SpanStubsFromReadOnlySpans(rec.Ended())[0].Status.Description; got != tc.want {
				t.Fatalf("status description %q, want %q", got, tc.want)
			}
			if s.ErrorRenderer() == nil || (*scope.Scope)(nil).ErrorRenderer() != nil {
				t.Fatal("ErrorRenderer accessor")
			}
		})
	}
}
