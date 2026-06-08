// Package xtrace provides small OpenTelemetry tracing helpers.
//
// Layout:
//
//	pkg/span   - Run/Call helpers for trace spans
//	pkg/scope  - component instrumentation scopes
//	contrib/libs/xlog      - xlog integration
//	contrib/sdk   - optional OpenTelemetry SDK bootstrap
//
// The root package re-exports the most common identifiers for ergonomic use.
package xtrace

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/gopherex/xtrace/pkg/scope"
	"github.com/gopherex/xtrace/pkg/span"
)

type (
	Scope       = scope.Scope
	ScopeOption = scope.Option

	SpanFunc            = span.Func
	SpanFunc1[T any]    = span.Func1[T]
	SpanFunc2[T, U any] = span.Func2[T, U]
	SpanOption          = span.Option
	PanicError          = span.PanicError
)

var ErrNilFunc = span.ErrNilFunc

func New(name string, opts ...ScopeOption) *Scope { return scope.New(name, opts...) }

func WithTracerProvider(provider trace.TracerProvider) ScopeOption {
	return scope.WithTracerProvider(provider)
}

func WithMeterProvider(provider metric.MeterProvider) ScopeOption {
	return scope.WithMeterProvider(provider)
}

func Run(ctx context.Context, tracer trace.Tracer, name string, f SpanFunc, opts ...SpanOption) error {
	return span.Run(ctx, tracer, name, f, opts...)
}

func Call[T any](ctx context.Context, tracer trace.Tracer, name string, f SpanFunc1[T], opts ...SpanOption) (T, error) {
	return span.Call(ctx, tracer, name, f, opts...)
}

func Call2[T, U any](ctx context.Context, tracer trace.Tracer, name string, f SpanFunc2[T, U], opts ...SpanOption) (T, U, error) {
	return span.Call2(ctx, tracer, name, f, opts...)
}

func WithSpanOptions(opts ...trace.SpanStartOption) SpanOption {
	return span.WithSpanOptions(opts...)
}

func WithAttrs(attrs ...attribute.KeyValue) SpanOption {
	return span.WithAttrs(attrs...)
}

func WithErrorFilter(filter func(error) bool) SpanOption {
	return span.WithErrorFilter(filter)
}

func WithoutRecordError() SpanOption { return span.WithoutRecordError() }
func WithoutErrorStatus() SpanOption { return span.WithoutErrorStatus() }
func WithRecoverPanics() SpanOption  { return span.WithRecoverPanics() }
