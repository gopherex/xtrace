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
	name        string
	tracer      trace.Tracer
	meter       metric.Meter
	renderError func(error) string
}

type options struct {
	tracerProvider trace.TracerProvider
	meterProvider  metric.MeterProvider
	renderError    func(error) string
}

// WithErrorRenderer renders the errors every Run of this scope records (see
// span.WithErrorRenderer). Set it once on the service's scope; a Run option
// still overrides it.
func WithErrorRenderer(render func(error) string) Option {
	return func(o *options) {
		o.renderError = render
	}
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
		name:        name,
		tracer:      o.tracerProvider.Tracer(name),
		meter:       o.meterProvider.Meter(name),
		renderError: o.renderError,
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
	if s != nil && s.renderError != nil {
		opts = append([]span.Option{span.WithErrorRenderer(s.renderError)}, opts...)
	}
	return span.Run(ctx, s.Tracer(), name, f, opts...)
}

// ErrorRenderer returns the scope's error renderer (nil when unset), for
// helpers that record errors outside Run, such as xtrace.Call.
func (s *Scope) ErrorRenderer() func(error) string {
	if s == nil {
		return nil
	}
	return s.renderError
}
