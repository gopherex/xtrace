// Package scope groups OpenTelemetry instruments under one instrumentation
// scope name.
package scope

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/gopherex/xtrace/pkg/span"
)

// Scope owns commonly used OpenTelemetry instruments for a component.
type Scope struct {
	name   string
	tracer trace.Tracer
	meter  metric.Meter
}

type options struct {
	tracerProvider trace.TracerProvider
	meterProvider  metric.MeterProvider
}

// Option configures a Scope.
type Option func(*options)

// WithTracerProvider uses provider to construct the scope tracer.
func WithTracerProvider(provider trace.TracerProvider) Option {
	return func(o *options) {
		o.tracerProvider = provider
	}
}

// WithMeterProvider uses provider to construct the scope meter.
func WithMeterProvider(provider metric.MeterProvider) Option {
	return func(o *options) {
		o.meterProvider = provider
	}
}

// New constructs a Scope for name.
func New(name string, opts ...Option) *Scope {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if o.tracerProvider == nil {
		o.tracerProvider = otel.GetTracerProvider()
	}
	if o.meterProvider == nil {
		o.meterProvider = otel.GetMeterProvider()
	}
	return &Scope{
		name:   name,
		tracer: o.tracerProvider.Tracer(name),
		meter:  o.meterProvider.Meter(name),
	}
}

// Name returns the instrumentation scope name.
func (s *Scope) Name() string {
	if s == nil {
		return ""
	}
	return s.name
}

// Tracer returns the scope tracer.
func (s *Scope) Tracer() trace.Tracer {
	if s == nil || s.tracer == nil {
		return otel.Tracer("")
	}
	return s.tracer
}

// Meter returns the scope meter.
func (s *Scope) Meter() metric.Meter {
	if s == nil || s.meter == nil {
		return otel.Meter("")
	}
	return s.meter
}

// Run starts a span on the scope tracer and executes f.
func (s *Scope) Run(ctx context.Context, name string, f span.Func, opts ...span.Option) error {
	return span.Run(ctx, s.Tracer(), name, f, opts...)
}
