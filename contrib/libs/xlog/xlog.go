// Package xlogtrace wires xlog to OpenTelemetry trace correlation.
package xlogtrace

import (
	"context"

	"github.com/gopherex/xlog"
	xlogotel "github.com/gopherex/xlog/contrib/libs/otel"
	otellog "go.opentelemetry.io/otel/log"
)

// TraceFields returns trace_id/span_id fields from ctx.
func TraceFields(ctx context.Context) []xlog.Field {
	return xlogotel.TraceFields(ctx)
}

// WithTraceFields adds trace_id/span_id fields to context-aware xlog calls.
func WithTraceFields() xlog.Option {
	return xlog.WithContextFieldExtractor(xlogotel.TraceFields)
}

// SpanObserver records log events at or above minLevel onto the active span.
func SpanObserver(minLevel ...xlog.Level) xlog.Observer {
	return xlogotel.SpanObserver(minLevel...)
}

// WithSpanObserver installs SpanObserver as an xlog observer.
func WithSpanObserver(minLevel ...xlog.Level) xlog.Option {
	return xlog.WithObserver(SpanObserver(minLevel...))
}

// Core returns an xlog core that emits logs through the OpenTelemetry Logs API.
func Core(logger otellog.Logger) xlog.Core {
	return xlogotel.New(logger)
}

// Options returns the common correlation options for an ordinary xlog logger.
func Options(minLevel ...xlog.Level) []xlog.Option {
	return []xlog.Option{
		WithTraceFields(),
		WithSpanObserver(minLevel...),
	}
}
