# xtrace

Small OpenTelemetry tracing helpers for Go services.

`xtrace` keeps the core package thin: span helpers and instrumentation scopes
depend only on the OpenTelemetry API. Exporter setup and logging integrations
live in separate `contrib` modules.

## Install

```bash
go get github.com/gopherex/xtrace@latest
```

Optional integrations:

```bash
go get github.com/gopherex/xtrace/contrib/libs/xlog@latest
go get github.com/gopherex/xtrace/contrib/sdk@latest
```

## Span Helpers

```go
scope := xtrace.New("billing.worker")

err := scope.Run(ctx, "charge", func(ctx context.Context, span trace.Span) error {
	return charge(ctx)
}, xtrace.WithAttrs(attribute.String("tenant", tenantID)))
```

Returned errors are recorded on the span and set span status to `Error` by
default. Use `WithErrorFilter`, `WithoutRecordError`, or `WithoutErrorStatus`
to tune that behavior.

For return values:

```go
user, err := xtrace.Call(ctx, tracer, "user.load",
	func(ctx context.Context, span trace.Span) (User, error) {
		return users.Load(ctx, id)
	})
```

## xlog

`contrib/libs/xlog` wraps the existing `github.com/gopherex/xlog/contrib/libs/otel`
adapter:

```go
logger := xlog.NewJSON(xlogtrace.Options()...)
logger.Ctx().Info(ctx, "request handled")
```

It adds trace/span IDs to context-aware logs and records error logs onto the
active span.

## SDK Bootstrap

```go
shutdown, err := sdk.Setup(ctx,
	sdk.WithService("api"),
	sdk.WithVersion(version),
	sdk.WithInstanceID(instanceID),
)
if err != nil {
	return err
}
defer shutdown(context.Background())
```

The SDK package exists for the code part of OpenTelemetry setup: creating and
installing global providers, wiring shutdown, and stamping build-time service
metadata into the resource. Exporter endpoint, protocol, headers, and timeouts
still use the standard `OTEL_*` environment variables.

Build-time metadata passed through `WithService`, `WithVersion`, and
`WithInstanceID` overrides matching values from `OTEL_SERVICE_NAME` and
`OTEL_RESOURCE_ATTRIBUTES`.
