package sdk

import (
	"testing"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
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
