package scope

import (
	"context"

	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/embedded"
)

// ChildOnly wraps provider so its tracers start a span only under a parent:
// a context carrying a valid span context, local or remote. Without one they
// start nothing and return the context unchanged with its non-recording
// span. Given to an instrumented client (a database, a cache), it adds spans
// to the operations that use the client and never turns the client's own
// background work (polling, pings, sweeps) into root traces.
func ChildOnly(provider trace.TracerProvider) trace.TracerProvider {
	return childOnly{provider: provider}
}

type childOnly struct {
	embedded.TracerProvider

	provider trace.TracerProvider
}

func (c childOnly) Tracer(name string, opts ...trace.TracerOption) trace.Tracer {
	return childTracer{tracer: c.provider.Tracer(name, opts...)}
}

type childTracer struct {
	embedded.Tracer

	tracer trace.Tracer
}

func (t childTracer) Start(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return ctx, trace.SpanFromContext(ctx)
	}

	return t.tracer.Start(ctx, name, opts...)
}
