package sdk

import (
	"context"
	"slices"

	"go.opentelemetry.io/otel/attribute"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// WithoutSpanAttributes removes these attribute keys from every exported span,
// its events and its links, whenever they were set: at start, later (as
// otelgrpc sets network.peer.* in HandleRPC) or on an event. Use it for data
// that must never reach telemetry, such as client addresses.
func WithoutSpanAttributes(keys ...attribute.Key) Option {
	return func(c *config) { c.dropSpanAttrs = append(c.dropSpanAttrs, keys...) }
}

// WithoutLogAttributes removes these attribute keys from every log record
// before it is exported.
func WithoutLogAttributes(keys ...attribute.Key) Option {
	return func(c *config) { c.dropLogAttrs = append(c.dropLogAttrs, keys...) }
}

// redactingExporter strips keys from spans just before export. A span is
// read-only by then, so the exporter hands on a view of it whose attribute
// lists omit the keys.
type redactingExporter struct {
	next sdktrace.SpanExporter
	drop map[attribute.Key]struct{}
}

func newRedactingExporter(next sdktrace.SpanExporter, keys []attribute.Key) sdktrace.SpanExporter {
	return &redactingExporter{next: next, drop: keySet(keys)}
}

func (e *redactingExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	out := make([]sdktrace.ReadOnlySpan, len(spans))
	for i, s := range spans {
		out[i] = e.redact(s)
	}
	return e.next.ExportSpans(ctx, out)
}

func (e *redactingExporter) Shutdown(ctx context.Context) error { return e.next.Shutdown(ctx) }

func (e *redactingExporter) redact(s sdktrace.ReadOnlySpan) sdktrace.ReadOnlySpan {
	events := s.Events()
	redactedEvents := make([]sdktrace.Event, len(events))
	for i, ev := range events {
		ev.Attributes = e.keep(ev.Attributes)
		redactedEvents[i] = ev
	}
	links := s.Links()
	redactedLinks := make([]sdktrace.Link, len(links))
	for i, l := range links {
		l.Attributes = e.keep(l.Attributes)
		redactedLinks[i] = l
	}
	return redactedSpan{ReadOnlySpan: s, attrs: e.keep(s.Attributes()), events: redactedEvents, links: redactedLinks}
}

func (e *redactingExporter) keep(attrs []attribute.KeyValue) []attribute.KeyValue {
	return slices.DeleteFunc(slices.Clone(attrs), func(kv attribute.KeyValue) bool {
		_, drop := e.drop[kv.Key]
		return drop
	})
}

// redactedSpan is the span with its attribute lists replaced; everything else
// (identity, timing, status, resource) is the original's.
type redactedSpan struct {
	sdktrace.ReadOnlySpan
	attrs  []attribute.KeyValue
	events []sdktrace.Event
	links  []sdktrace.Link
}

func (s redactedSpan) Attributes() []attribute.KeyValue { return s.attrs }
func (s redactedSpan) Events() []sdktrace.Event         { return s.events }
func (s redactedSpan) Links() []sdktrace.Link           { return s.links }

// redactingProcessor strips keys from log records before the next processor
// sees them. Log attributes are fixed at emit, so this is enough.
type redactingProcessor struct {
	next sdklog.Processor
	drop map[attribute.Key]struct{}
}

func newRedactingProcessor(next sdklog.Processor, keys []attribute.Key) sdklog.Processor {
	return &redactingProcessor{next: next, drop: keySet(keys)}
}

func (p *redactingProcessor) OnEmit(ctx context.Context, r *sdklog.Record) error {
	kept := make([]attribute.KeyValue, 0, r.AttributesLen())
	dropped := false
	r.WalkAttributes(func(kv attribute.KeyValue) bool {
		if _, drop := p.drop[kv.Key]; drop {
			dropped = true
		} else {
			kept = append(kept, kv)
		}
		return true
	})
	if dropped {
		r.SetAttributes(kept...)
	}
	return p.next.OnEmit(ctx, r)
}

func (p *redactingProcessor) Enabled(ctx context.Context, param sdklog.EnabledParameters) bool {
	return p.next.Enabled(ctx, param)
}

func (p *redactingProcessor) Shutdown(ctx context.Context) error   { return p.next.Shutdown(ctx) }
func (p *redactingProcessor) ForceFlush(ctx context.Context) error { return p.next.ForceFlush(ctx) }

func keySet(keys []attribute.Key) map[attribute.Key]struct{} {
	set := make(map[attribute.Key]struct{}, len(keys))
	for _, k := range keys {
		set[k] = struct{}{}
	}
	return set
}
