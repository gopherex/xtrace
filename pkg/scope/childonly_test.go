package scope_test

import (
	"context"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/gopherex/xtrace/pkg/scope"
)

func TestChildOnly(t *testing.T) {
	t.Parallel()

	remote := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{1}, SpanID: trace.SpanID{2}, TraceFlags: trace.FlagsSampled, Remote: true,
	})

	cases := []struct {
		name   string
		parent func(trace.Tracer) context.Context
		spans  int
	}{
		{"no parent: no span", func(trace.Tracer) context.Context { return context.Background() }, 0},
		{"local parent: a child", func(parent trace.Tracer) context.Context {
			ctx, _ := parent.Start(context.Background(), "operation")
			return ctx
		}, 1},
		{"remote parent: a child", func(trace.Tracer) context.Context {
			return trace.ContextWithRemoteSpanContext(context.Background(), remote)
		}, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rec := tracetest.NewSpanRecorder()
			tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
			ctx := tc.parent(tp.Tracer("app"))

			got, span := scope.ChildOnly(tp).Tracer("client").Start(ctx, "command")
			span.End()

			children := 0
			for _, s := range rec.Ended() {
				if s.InstrumentationScope().Name != "client" {
					continue
				}

				children++
				if s.Parent().SpanID() != trace.SpanContextFromContext(ctx).SpanID() {
					t.Fatalf("parent %v, want %v", s.Parent().SpanID(), trace.SpanContextFromContext(ctx).SpanID())
				}
			}

			if children != tc.spans {
				t.Fatalf("client spans = %d, want %d", children, tc.spans)
			}

			if tc.spans == 0 && (span.IsRecording() || got != ctx) {
				t.Fatalf("without a parent: recording %v, context changed %v", span.IsRecording(), got != ctx)
			}
		})
	}
}

func BenchmarkChildOnly(b *testing.B) {
	tp := sdktrace.NewTracerProvider()
	tracer := scope.ChildOnly(tp).Tracer("client")
	parent, _ := tp.Tracer("app").Start(context.Background(), "operation")

	for _, bc := range []struct {
		name string
		ctx  context.Context
	}{{"no parent", context.Background()}, {"parent", parent}} {
		b.Run(bc.name, func(b *testing.B) {
			for b.Loop() {
				_, span := tracer.Start(bc.ctx, "command")
				span.End()
			}
		})
	}
}
