// Package span provides small helpers for running functions inside
// OpenTelemetry spans.
package span

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var ErrNilFunc = errors.New("xtrace: nil span function")

// Func is a function executed with a started span.
type Func func(ctx context.Context, span trace.Span) error

// Func1 is a function executed with a started span and one return value.
type Func1[T any] func(ctx context.Context, span trace.Span) (T, error)

// Func2 is a function executed with a started span and two return values.
type Func2[T, U any] func(ctx context.Context, span trace.Span) (T, U, error)

type options struct {
	startOptions  []trace.SpanStartOption
	errorFilter   func(error) bool
	recordError   bool
	errorStatus   bool
	recoverPanics bool
}

// Option configures span helper behavior.
type Option func(*options)

func newOptions(opts ...Option) options {
	o := options{
		errorFilter: func(error) bool { return true },
		recordError: true,
		errorStatus: true,
	}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// WithSpanOptions appends OpenTelemetry span start options.
func WithSpanOptions(opts ...trace.SpanStartOption) Option {
	return func(o *options) {
		o.startOptions = append(o.startOptions, opts...)
	}
}

// WithAttrs starts the span with the provided attributes.
func WithAttrs(attrs ...attribute.KeyValue) Option {
	return WithSpanOptions(trace.WithAttributes(attrs...))
}

// WithErrorFilter decides which returned errors should be recorded on the span.
func WithErrorFilter(filter func(error) bool) Option {
	return func(o *options) {
		if filter != nil {
			o.errorFilter = filter
		}
	}
}

// WithoutRecordError prevents RecordError calls for returned errors.
func WithoutRecordError() Option {
	return func(o *options) {
		o.recordError = false
	}
}

// WithoutErrorStatus prevents setting span status to Error for returned errors.
func WithoutErrorStatus() Option {
	return func(o *options) {
		o.errorStatus = false
	}
}

// WithRecoverPanics converts a panic into a PanicError return value after
// recording it on the span. Without this option panics are recorded and
// re-panicked.
func WithRecoverPanics() Option {
	return func(o *options) {
		o.recoverPanics = true
	}
}

// PanicError is returned when WithRecoverPanics catches a panic.
type PanicError struct {
	Value any
}

func (e PanicError) Error() string {
	return fmt.Sprintf("xtrace: panic: %v", e.Value)
}

// Run starts name on tracer, executes f, records returned errors, and ends the
// span. A nil tracer falls back to the global OpenTelemetry tracer.
func Run(ctx context.Context, tracer trace.Tracer, name string, f Func, opts ...Option) (err error) {
	if f == nil {
		return ErrNilFunc
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if tracer == nil {
		tracer = otel.Tracer("")
	}
	o := newOptions(opts...)
	spanCtx, sp := tracer.Start(ctx, name, o.startOptions...)
	defer finish(sp, o, &err)

	err = f(spanCtx, sp)
	return err
}

// Call is Run with one return value.
func Call[T any](ctx context.Context, tracer trace.Tracer, name string, f Func1[T], opts ...Option) (ret T, err error) {
	if f == nil {
		return ret, ErrNilFunc
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if tracer == nil {
		tracer = otel.Tracer("")
	}
	o := newOptions(opts...)
	spanCtx, sp := tracer.Start(ctx, name, o.startOptions...)
	defer finish(sp, o, &err)

	ret, err = f(spanCtx, sp)
	return ret, err
}

// Call2 is Run with two return values.
func Call2[T, U any](ctx context.Context, tracer trace.Tracer, name string, f Func2[T, U], opts ...Option) (ret T, ret2 U, err error) {
	if f == nil {
		return ret, ret2, ErrNilFunc
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if tracer == nil {
		tracer = otel.Tracer("")
	}
	o := newOptions(opts...)
	spanCtx, sp := tracer.Start(ctx, name, o.startOptions...)
	defer finish(sp, o, &err)

	ret, ret2, err = f(spanCtx, sp)
	return ret, ret2, err
}

func finish(sp trace.Span, o options, err *error) {
	if recovered := recover(); recovered != nil {
		panicErr := PanicError{Value: recovered}
		handleError(sp, o, panicErr)
		sp.End()
		if o.recoverPanics {
			*err = panicErr
			return
		}
		panic(recovered)
	}
	if *err != nil && o.errorFilter(*err) {
		handleError(sp, o, *err)
	}
	sp.End()
}

func handleError(sp trace.Span, o options, err error) {
	if o.recordError {
		sp.RecordError(err)
	}
	if o.errorStatus {
		sp.SetStatus(codes.Error, err.Error())
	}
}
