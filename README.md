# xtrace

Small OpenTelemetry tracing helpers for Go services.

`xtrace` keeps the core package thin: span helpers and instrumentation scopes
depend only on the OpenTelemetry API. Exporter setup and logging integrations
live in separate `contrib` modules.

OpenTelemetry API/SDK minimum: v1.45.0. Logging integrations require the Logs
API/SDK v0.21.0 or newer; the SDK uses semantic conventions v1.43.0.

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

A technical failure with the stack captured where it was made, and an
expected refusal that leaves the span unmarked:

```go
err := scope.Run(ctx, "charge", run,
	xtrace.WithErrorFilter(func(err error) bool { return !fault.IsExpected(err) }),
	xtrace.WithStackTrace(fault.Stack)) // the error's own stack as exception.stacktrace
```

Error texts can carry what must not reach a trace (a `*url.Error` holds the
full signed URL, a PostgreSQL error row values). Render them once per scope
with the same renderer as the logger:

```go
render := xlog.NewErrorRenderer(xlog.RedactURL, xlogpgx.RedactPgError)
scope := xtrace.New("billing.worker", xtrace.WithErrorRenderer(render))
```

Every `scope.Run` then records `exception.message` and the status description
as `render(err)`; the exception type and stack stay, and the returned error is
untouched. `WithSpanErrorRenderer` does the same for one `Run`/`Call`.

`WithStackTrace(extract)` records what `extract` returns as
`exception.stacktrace`; an empty result records none (a stack taken at the
span's end would show only the helper's frames).

For return values:

```go
user, err := xtrace.Call(ctx, tracer, "user.load",
	func(ctx context.Context, span trace.Span) (User, error) {
		return users.Load(ctx, id)
	})
```

## Instrumented clients: spans only under an operation

```go
client, err := valkeyotel.NewClient(opts, valkeyotel.WithTracerProvider(scope.ChildOnly(otel.GetTracerProvider())))
```

`scope.ChildOnly(provider)` starts a span only when the context already
carries one (local or remote). A client's background work (polling, pings,
sweeps) creates no root traces; its commands still appear as children of the
operations that issue them. Metrics are not affected.

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

A signal is exported only when an OTLP endpoint is configured for it:
`OTEL_EXPORTER_OTLP_<SIGNAL>_ENDPOINT` (`TRACES`, `METRICS`, `LOGS`) or the
shared `OTEL_EXPORTER_OTLP_ENDPOINT`. With no endpoint, `Setup` disables that
signal exactly as `WithoutTraces`, `WithoutMetrics`, or `WithoutLogs` would and
logs one line (via `slog`) naming the disabled signals; it no longer falls back
to the OpenTelemetry default of `localhost:4317`/`4318`. To export to a local
collector, set the endpoint explicitly, e.g.
`OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318`. An explicit `Without*`
option always wins over a configured endpoint. `StartHostRuntime` is a no-op
when metrics are disabled.

Data that must never reach telemetry (client addresses, for example) is
removed at export, whenever instrumentation set it:

```go
ip := []attribute.Key{"client.address", "client.port", "network.peer.address", "network.peer.port"}
shutdown, err := sdk.Setup(ctx, sdk.WithService("api"),
	sdk.WithoutSpanAttributes(ip...), // span, event and link attributes
	sdk.WithoutLogAttributes(ip...),  // log record attributes
)
```

Spans are filtered in a wrapper around the exporter, so an attribute added
after the span started (otelgrpc sets `network.peer.*` in `HandleRPC`) is
dropped too; log records are filtered before the batch processor. Metrics
attributes are not touched.

Build-time metadata passed through `WithService`, `WithVersion`, and
`WithInstanceID` overrides matching values from `OTEL_SERVICE_NAME` and
`OTEL_RESOURCE_ATTRIBUTES`.
