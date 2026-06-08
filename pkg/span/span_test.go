package span_test

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/gopherex/xtrace/pkg/span"
)

func testTracer() (trace.Tracer, func() []tracetest.SpanStub) {
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	return tp.Tracer("test"), func() []tracetest.SpanStub {
		return tracetest.SpanStubsFromReadOnlySpans(rec.Ended())
	}
}

func TestRunRecordsError(t *testing.T) {
	tracer, ended := testTracer()
	want := errors.New("boom")

	err := span.Run(context.Background(), tracer, "op", func(context.Context, trace.Span) error {
		return want
	})

	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
	spans := ended()
	if len(spans) != 1 {
		t.Fatalf("ended spans = %d", len(spans))
	}
	if spans[0].Status.Code != codes.Error {
		t.Fatalf("status = %v, want error", spans[0].Status.Code)
	}
	if len(spans[0].Events) != 1 {
		t.Fatalf("events = %d, want recorded error event", len(spans[0].Events))
	}
}

func TestRunCanFilterErrors(t *testing.T) {
	tracer, ended := testTracer()

	err := span.Run(context.Background(), tracer, "op", func(context.Context, trace.Span) error {
		return errors.New("ignored")
	}, span.WithErrorFilter(func(error) bool { return false }))

	if err == nil {
		t.Fatal("err = nil, want returned error")
	}
	spans := ended()
	if spans[0].Status.Code == codes.Error {
		t.Fatalf("status = %v, want non-error", spans[0].Status.Code)
	}
	if len(spans[0].Events) != 0 {
		t.Fatalf("events = %d, want none", len(spans[0].Events))
	}
}

func TestCallReturnsValue(t *testing.T) {
	tracer, _ := testTracer()

	got, err := span.Call(context.Background(), tracer, "op", func(context.Context, trace.Span) (int, error) {
		return 42, nil
	})

	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got != 42 {
		t.Fatalf("got = %d, want 42", got)
	}
}

func TestRecoverPanics(t *testing.T) {
	tracer, ended := testTracer()

	err := span.Run(context.Background(), tracer, "op", func(context.Context, trace.Span) error {
		panic("boom")
	}, span.WithRecoverPanics())

	var panicErr span.PanicError
	if !errors.As(err, &panicErr) {
		t.Fatalf("err = %T %v, want PanicError", err, err)
	}
	spans := ended()
	if spans[0].Status.Code != codes.Error {
		t.Fatalf("status = %v, want error", spans[0].Status.Code)
	}
}
