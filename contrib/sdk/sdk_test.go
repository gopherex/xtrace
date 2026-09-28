package sdk

import (
	"context"
	"os"
	"slices"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
)

func TestResourceBuildMetadataOverridesEnv(t *testing.T) {
	t.Setenv("OTEL_SERVICE_NAME", "env-service")
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "service.version=env-version,service.instance.id=env-instance")

	res, err := newResource(config{
		serviceName:       "build-service",
		serviceVersion:    "build-version",
		serviceInstanceID: "build-instance",
	})
	if err != nil {
		t.Fatalf("newResource error = %v", err)
	}

	attrs := map[attribute.Key]attribute.Value{}
	for _, attr := range res.Attributes() {
		attrs[attr.Key] = attr.Value
	}

	if got := attrs[semconv.ServiceNameKey].AsString(); got != "build-service" {
		t.Fatalf("service.name = %q, want build-service", got)
	}
	if got := attrs[semconv.ServiceVersionKey].AsString(); got != "build-version" {
		t.Fatalf("service.version = %q, want build-version", got)
	}
	if got := attrs[semconv.ServiceInstanceIDKey].AsString(); got != "build-instance" {
		t.Fatalf("service.instance.id = %q, want build-instance", got)
	}
}

func TestSignalsWithoutEndpointAreDisabled(t *testing.T) {
	const (
		generic = "OTEL_EXPORTER_OTLP_ENDPOINT"
		traces  = "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"
		metrics = "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT"
		logs    = "OTEL_EXPORTER_OTLP_LOGS_ENDPOINT"
	)
	type signals struct{ traces, metrics, logs bool }

	tests := []struct {
		name         string
		env          map[string]string
		opts         []Option
		want         signals
		wantDisabled []string
	}{
		{
			name:         "no endpoint",
			want:         signals{},
			wantDisabled: []string{"traces", "metrics", "logs"},
		},
		{
			name: "generic endpoint",
			env:  map[string]string{generic: "http://collector:4318"},
			want: signals{traces: true, metrics: true, logs: true},
		},
		{
			name:         "empty generic endpoint",
			env:          map[string]string{generic: ""},
			want:         signals{},
			wantDisabled: []string{"traces", "metrics", "logs"},
		},
		{
			name:         "traces endpoint only",
			env:          map[string]string{traces: "http://collector:4318/v1/traces"},
			want:         signals{traces: true},
			wantDisabled: []string{"metrics", "logs"},
		},
		{
			name:         "metrics endpoint only",
			env:          map[string]string{metrics: "http://collector:4318/v1/metrics"},
			want:         signals{metrics: true},
			wantDisabled: []string{"traces", "logs"},
		},
		{
			name:         "logs endpoint only",
			env:          map[string]string{logs: "http://collector:4318/v1/logs"},
			want:         signals{logs: true},
			wantDisabled: []string{"traces", "metrics"},
		},
		{
			name: "per-signal endpoints",
			env: map[string]string{
				traces:  "http://collector:4318/v1/traces",
				metrics: "http://collector:4318/v1/metrics",
				logs:    "http://collector:4318/v1/logs",
			},
			want: signals{traces: true, metrics: true, logs: true},
		},
		{
			name: "generic and per-signal endpoints",
			env: map[string]string{
				generic: "http://collector:4318",
				metrics: "http://metrics:4318/v1/metrics",
			},
			want: signals{traces: true, metrics: true, logs: true},
		},
		{
			name: "explicit without wins over endpoint",
			env:  map[string]string{generic: "http://collector:4318"},
			opts: []Option{WithoutTraces(), WithoutMetrics(), WithoutLogs()},
			want: signals{},
		},
		{
			name: "explicit without wins over per-signal endpoint",
			env: map[string]string{
				traces:  "http://collector:4318/v1/traces",
				metrics: "http://collector:4318/v1/metrics",
			},
			opts: []Option{WithoutMetrics()},
			want: signals{traces: true},
			// metrics is disabled explicitly, not for a missing endpoint.
			wantDisabled: []string{"logs"},
		},
		{
			name: "explicit without and no endpoint",
			opts: []Option{WithoutTraces()},
			want: signals{},
			// traces is disabled explicitly, not for a missing endpoint.
			wantDisabled: []string{"metrics", "logs"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, key := range []string{generic, traces, metrics, logs} {
				t.Setenv(key, "")
				os.Unsetenv(key)
			}
			for key, value := range tt.env {
				t.Setenv(key, value)
			}

			cfg := defaultConfig()
			for _, opt := range tt.opts {
				opt(&cfg)
			}
			disabled := disableSignalsWithoutEndpoint(&cfg)

			got := signals{traces: cfg.traces, metrics: cfg.metrics, logs: cfg.logs}
			if got != tt.want {
				t.Fatalf("signals = %+v, want %+v", got, tt.want)
			}
			if !slices.Equal(disabled, tt.wantDisabled) {
				t.Fatalf("disabled = %v, want %v", disabled, tt.wantDisabled)
			}
		})
	}
}

func TestStartHostRuntimeFollowsMetricsSignal(t *testing.T) {
	tests := []struct {
		name        string
		env         map[string]string
		opts        []Option
		wantMetrics bool
	}{
		{
			name:        "no endpoint",
			wantMetrics: false,
		},
		{
			name:        "explicit without metrics",
			env:         map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": "http://127.0.0.1:1"},
			opts:        []Option{WithoutTraces(), WithoutMetrics(), WithoutLogs()},
			wantMetrics: false,
		},
		{
			name:        "metrics endpoint",
			env:         map[string]string{"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT": "http://127.0.0.1:1"},
			wantMetrics: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, key := range []string{
				"OTEL_EXPORTER_OTLP_ENDPOINT",
				"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
				"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT",
				"OTEL_EXPORTER_OTLP_LOGS_ENDPOINT",
			} {
				t.Setenv(key, "")
				os.Unsetenv(key)
			}
			for key, value := range tt.env {
				t.Setenv(key, value)
			}
			prev := otel.GetMeterProvider()
			t.Cleanup(func() { otel.SetMeterProvider(prev) })

			shutdown, err := Setup(context.Background(), tt.opts...)
			if err != nil {
				t.Fatalf("Setup error = %v", err)
			}
			t.Cleanup(func() {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				_ = shutdown(ctx)
			})

			// Observe what StartHostRuntime registers through a local reader.
			reader := sdkmetric.NewManualReader()
			otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)))

			if err := StartHostRuntime(); err != nil {
				t.Fatalf("StartHostRuntime error = %v", err)
			}

			var rm metricdata.ResourceMetrics
			if err := reader.Collect(context.Background(), &rm); err != nil {
				t.Fatalf("Collect error = %v", err)
			}
			if got := len(rm.ScopeMetrics) > 0; got != tt.wantMetrics {
				t.Fatalf("host/runtime metrics registered = %v, want %v", got, tt.wantMetrics)
			}
		})
	}
}
