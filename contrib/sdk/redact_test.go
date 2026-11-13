package sdk

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	otellog "go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

var clientIP = []attribute.Key{"client.address", "network.peer.address", "client.port"}

func keysOf(kvs []attribute.KeyValue) map[attribute.Key]string {
	out := map[attribute.Key]string{}
	for _, kv := range kvs {
		out[kv.Key] = kv.Value.Emit()
	}
	return out
}

// Keys are removed from the exported span whenever they were set: at start,
// after start (as otelgrpc does), on events and on links. Other attributes
// survive.
func TestRedactingExporter(t *testing.T) {
	t.Parallel()
	mem := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(newRedactingExporter(mem, clientIP)))
	defer func() { _ = tp.Shutdown(context.Background()) }()

	linked := trace.NewSpanContext(trace.SpanContextConfig{TraceID: trace.TraceID{1}, SpanID: trace.SpanID{1}})
	_, span := tp.Tracer("t").Start(context.Background(), "rpc",
		trace.WithAttributes(attribute.String("client.address", "10.0.0.1"), attribute.String("rpc.method", "Get")),
		trace.WithLinks(trace.Link{SpanContext: linked, Attributes: []attribute.KeyValue{
			attribute.String("network.peer.address", "10.0.0.2"), attribute.String("kept", "yes"),
		}}))
	span.SetAttributes(attribute.String("network.peer.address", "10.0.0.3"), attribute.Int("client.port", 443))
	span.AddEvent("msg", trace.WithAttributes(attribute.String("client.address", "10.0.0.4"), attribute.Int("size", 1)))
	span.End()

	got := mem.GetSpans()[0]
	if attrs := keysOf(got.Attributes); len(attrs) != 1 || attrs["rpc.method"] != "Get" {
		t.Fatalf("span attributes %v, want only rpc.method", attrs)
	}
	if attrs := keysOf(got.Events[0].Attributes); len(attrs) != 1 || attrs["size"] != "1" {
		t.Fatalf("event attributes %v, want only size", attrs)
	}
	if attrs := keysOf(got.Links[0].Attributes); len(attrs) != 1 || attrs["kept"] != "yes" {
		t.Fatalf("link attributes %v, want only kept", attrs)
	}
	if got.Name != "rpc" || got.SpanContext.SpanID() != span.SpanContext().SpanID() {
		t.Fatalf("span identity lost: %v", got)
	}
}

// recordProcessor keeps what it receives.
type recordProcessor struct{ records []sdklog.Record }

func (p *recordProcessor) OnEmit(_ context.Context, r *sdklog.Record) error {
	p.records = append(p.records, r.Clone())
	return nil
}
func (p *recordProcessor) Enabled(context.Context, sdklog.EnabledParameters) bool { return true }
func (p *recordProcessor) Shutdown(context.Context) error                         { return nil }
func (p *recordProcessor) ForceFlush(context.Context) error                       { return nil }

func TestRedactingLogProcessor(t *testing.T) {
	t.Parallel()
	inner := &recordProcessor{}
	lp := sdklog.NewLoggerProvider(sdklog.WithProcessor(newRedactingProcessor(inner, clientIP)))
	defer func() { _ = lp.Shutdown(context.Background()) }()

	var rec otellog.Record
	rec.SetBody(attribute.StringValue("request"))
	rec.AddAttributes(attribute.String("client.address", "10.0.0.1"), attribute.String("route", "/x"),
		attribute.Int("client.port", 443))
	lp.Logger("t").Emit(context.Background(), rec)

	got := map[attribute.Key]string{}
	inner.records[0].WalkAttributes(func(kv attribute.KeyValue) bool {
		got[kv.Key] = kv.Value.Emit()
		return true
	})
	if len(got) != 1 || got["route"] != "/x" {
		t.Fatalf("log attributes %v, want only route", got)
	}
}

func BenchmarkRedactSpan(b *testing.B) {
	mem := tracetest.NewInMemoryExporter()
	exp := newRedactingExporter(mem, clientIP)
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	_, span := tp.Tracer("t").Start(context.Background(), "rpc",
		trace.WithAttributes(attribute.String("client.address", "10.0.0.1"), attribute.String("rpc.method", "Get")))
	span.End()
	spans := mem.GetSpans().Snapshots()
	for b.Loop() {
		_ = exp.ExportSpans(context.Background(), spans)
		mem.Reset()
	}
}
