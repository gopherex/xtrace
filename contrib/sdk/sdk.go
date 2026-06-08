// Package sdk configures global OpenTelemetry SDK providers for xtrace-based
// services.
package sdk

import (
	"context"
	"errors"
	"os"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/host"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	logexport "go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	metricexport "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	traceexport "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	logglobal "go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/semconv/v1.40.0"
)

const (
	DefaultMetricInterval = 30 * time.Second
	DefaultMetricTimeout  = 10 * time.Second
)

// Shutdown stops all providers configured by Setup.
type Shutdown func(context.Context) error

type config struct {
	serviceName       string
	serviceVersion    string
	serviceInstanceID string
	attrs             []attribute.KeyValue
	propagator        propagation.TextMapPropagator
	traces            bool
	metrics           bool
	logs              bool
	metricInterval    time.Duration
	metricTimeout     time.Duration
}

// Option configures Setup.
type Option func(*config)

func defaultConfig() config {
	return config{
		propagator:     propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}),
		traces:         true,
		metrics:        true,
		logs:           true,
		metricInterval: DefaultMetricInterval,
		metricTimeout:  DefaultMetricTimeout,
	}
}

// WithService sets service.name.
func WithService(name string) Option {
	return func(c *config) {
		c.serviceName = name
	}
}

// WithVersion sets service.version.
func WithVersion(version string) Option {
	return func(c *config) {
		c.serviceVersion = version
	}
}

// WithInstanceID sets service.instance.id.
func WithInstanceID(id string) Option {
	return func(c *config) {
		c.serviceInstanceID = id
	}
}

// WithAttributes adds resource attributes.
func WithAttributes(attrs ...attribute.KeyValue) Option {
	return func(c *config) {
		c.attrs = append(c.attrs, attrs...)
	}
}

// WithPropagator overrides the global text map propagator.
func WithPropagator(propagator propagation.TextMapPropagator) Option {
	return func(c *config) {
		if propagator != nil {
			c.propagator = propagator
		}
	}
}

func WithoutTraces() Option  { return func(c *config) { c.traces = false } }
func WithoutMetrics() Option { return func(c *config) { c.metrics = false } }
func WithoutLogs() Option    { return func(c *config) { c.logs = false } }

// WithMetricReaderTiming overrides periodic metric export timing.
func WithMetricReaderTiming(interval, timeout time.Duration) Option {
	return func(c *config) {
		if interval > 0 {
			c.metricInterval = interval
		}
		if timeout > 0 {
			c.metricTimeout = timeout
		}
	}
}

// Setup configures global OpenTelemetry providers and returns a shutdown
// function. OTLP endpoint, headers, timeouts, and protocol are read by the
// exporters from standard OTEL_* environment variables.
func Setup(ctx context.Context, opts ...Option) (Shutdown, error) {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	if ctx == nil {
		ctx = context.Background()
	}

	otel.SetTextMapPropagator(cfg.propagator)

	res, err := newResource(cfg)
	if err != nil {
		return nil, err
	}

	var shutdowns []Shutdown
	shutdown := func(ctx context.Context) error {
		var err error
		for i := len(shutdowns) - 1; i >= 0; i-- {
			err = errors.Join(err, shutdowns[i](ctx))
		}
		shutdowns = nil
		return err
	}

	if cfg.traces {
		tp, err := newTracerProvider(ctx, res)
		if err != nil {
			_ = shutdown(ctx)
			return nil, err
		}
		shutdowns = append(shutdowns, tp.Shutdown)
		otel.SetTracerProvider(tp)
	}

	if cfg.metrics {
		mp, err := newMeterProvider(ctx, res, cfg)
		if err != nil {
			_ = shutdown(ctx)
			return nil, err
		}
		shutdowns = append(shutdowns, mp.Shutdown)
		otel.SetMeterProvider(mp)
	}

	if cfg.logs {
		lp, err := newLoggerProvider(ctx, res)
		if err != nil {
			_ = shutdown(ctx)
			return nil, err
		}
		shutdowns = append(shutdowns, lp.Shutdown)
		logglobal.SetLoggerProvider(lp)
	}

	return shutdown, nil
}

// StartHostRuntime starts host and runtime instrumentation. The OTel contrib
// helpers register observers with the global meter provider and do not expose a
// shutdown handle.
func StartHostRuntime() error {
	return errors.Join(host.Start(), runtime.Start())
}

func newResource(cfg config) (*resource.Resource, error) {
	hostName, _ := os.Hostname()
	attrs := make([]attribute.KeyValue, 0, len(cfg.attrs)+4)
	if cfg.serviceName != "" {
		attrs = append(attrs, semconv.ServiceName(cfg.serviceName))
	}
	if cfg.serviceVersion != "" {
		attrs = append(attrs, semconv.ServiceVersion(cfg.serviceVersion))
	}
	if cfg.serviceInstanceID != "" {
		attrs = append(attrs, semconv.ServiceInstanceID(cfg.serviceInstanceID))
	}
	if hostName != "" {
		attrs = append(attrs, semconv.HostName(hostName))
	}
	attrs = append(attrs, cfg.attrs...)

	return resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL, attrs...),
	)
}

func newTracerProvider(ctx context.Context, res *resource.Resource) (*sdktrace.TracerProvider, error) {
	exp, err := traceexport.New(ctx,
		traceexport.WithRetry(traceexport.RetryConfig{
			Enabled:         true,
			InitialInterval: 5 * time.Second,
			MaxInterval:     30 * time.Second,
			MaxElapsedTime:  time.Minute,
		}),
		traceexport.WithCompression(traceexport.GzipCompression),
	)
	if err != nil {
		return nil, err
	}
	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	), nil
}

func newMeterProvider(ctx context.Context, res *resource.Resource, cfg config) (*sdkmetric.MeterProvider, error) {
	exp, err := metricexport.New(ctx,
		metricexport.WithRetry(metricexport.RetryConfig{
			Enabled:         true,
			InitialInterval: 5 * time.Second,
			MaxInterval:     30 * time.Second,
			MaxElapsedTime:  time.Minute,
		}),
		metricexport.WithCompression(metricexport.GzipCompression),
	)
	if err != nil {
		return nil, err
	}
	return sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(
			exp,
			sdkmetric.WithInterval(cfg.metricInterval),
			sdkmetric.WithTimeout(cfg.metricTimeout),
		)),
		sdkmetric.WithResource(res),
	), nil
}

func newLoggerProvider(ctx context.Context, res *resource.Resource) (*sdklog.LoggerProvider, error) {
	exp, err := logexport.New(ctx,
		logexport.WithRetry(logexport.RetryConfig{
			Enabled:         true,
			InitialInterval: 5 * time.Second,
			MaxInterval:     30 * time.Second,
			MaxElapsedTime:  time.Minute,
		}),
		logexport.WithCompression(logexport.GzipCompression),
	)
	if err != nil {
		return nil, err
	}
	return sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exp)),
		sdklog.WithResource(res),
	), nil
}
