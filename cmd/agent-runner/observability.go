package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

type agentObservability struct {
	enabled bool
	tracer  trace.Tracer

	shutdown func(context.Context) error

	agentRuns       metric.Int64Counter
	agentRunDurMs   metric.Float64Histogram
	inTok           metric.Int64Counter
	outTok          metric.Int64Counter
	toolInvocations metric.Int64Counter
	skillDurMs      metric.Float64Histogram
}

var obs = &agentObservability{
	tracer:   otel.Tracer("sympozium/agent-runner"),
	shutdown: func(context.Context) error { return nil },
}

func initObservability(ctx context.Context) *agentObservability {
	enabled := strings.EqualFold(getEnv("SYMPOZIUM_OTEL_ENABLED", ""), "true")
	if !enabled {
		return obs
	}

	serviceName := firstNonEmpty(
		getEnv("SYMPOZIUM_OTEL_SERVICE_NAME", ""),
		getEnv("OTEL_SERVICE_NAME", ""),
		"sympozium-agent-runner",
	)
	endpoint := firstNonEmpty(
		getEnv("SYMPOZIUM_OTEL_OTLP_ENDPOINT", ""),
		getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
	)
	protocol := strings.ToLower(firstNonEmpty(
		getEnv("SYMPOZIUM_OTEL_OTLP_PROTOCOL", ""),
		getEnv("OTEL_EXPORTER_OTLP_PROTOCOL", ""),
		"grpc",
	))
	resAttrStr := firstNonEmpty(
		getEnv("SYMPOZIUM_OTEL_RESOURCE_ATTRIBUTES", ""),
		getEnv("OTEL_RESOURCE_ATTRIBUTES", ""),
	)

	if endpoint == "" {
		log.Println("observability enabled but no OTLP endpoint set; skipping OTel bootstrap")
		return obs
	}

	res := buildOTelResource(serviceName, resAttrStr)
	tracerProvider, meterProvider, err := buildProviders(ctx, protocol, endpoint, res)
	if err != nil {
		log.Printf("failed to initialize OTel exporters: %v", err)
		return obs
	}

	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)

	o := &agentObservability{
		enabled: true,
		tracer:  otel.Tracer("sympozium/agent-runner"),
		shutdown: func(ctx context.Context) error {
			var firstErr error
			if err := tracerProvider.Shutdown(ctx); err != nil {
				firstErr = err
			}
			if err := meterProvider.Shutdown(ctx); err != nil && firstErr == nil {
				firstErr = err
			}
			return firstErr
		},
	}
	o.initMetrics()
	obs = o
	return o
}

func buildOTelResource(serviceName, attrsCSV string) *resource.Resource {
	attrs := []attribute.KeyValue{
		semconv.ServiceName(serviceName),
		attribute.String("service.namespace", "sympozium"),
	}
	for k, v := range parseResourceAttributes(attrsCSV) {
		attrs = append(attrs, attribute.String(k, v))
	}
	res, err := resource.New(context.Background(),
		resource.WithAttributes(attrs...),
		resource.WithHost(),
		resource.WithOS(),
		resource.WithProcess(),
	)
	if err != nil {
		log.Printf("failed building OTel resource, using defaults: %v", err)
		return resource.Default()
	}
	return res
}

func buildProviders(
	ctx context.Context,
	protocol string,
	endpoint string,
	res *resource.Resource,
) (*sdktrace.TracerProvider, *sdkmetric.MeterProvider, error) {
	cleanEndpoint, insecure := normalizeEndpoint(endpoint)

	var (
		traceExp sdktrace.SpanExporter
		metricRM sdkmetric.Reader
		err      error
	)

	switch protocol {
	case "http/protobuf":
		traceOpts := []otlptracehttp.Option{otlptracehttp.WithEndpoint(cleanEndpoint)}
		metricOpts := []otlpmetrichttp.Option{otlpmetrichttp.WithEndpoint(cleanEndpoint)}
		if insecure {
			traceOpts = append(traceOpts, otlptracehttp.WithInsecure())
			metricOpts = append(metricOpts, otlpmetrichttp.WithInsecure())
		}
		traceExp, err = otlptracehttp.New(ctx, traceOpts...)
		if err != nil {
			return nil, nil, err
		}
		metricExp, err := otlpmetrichttp.New(ctx, metricOpts...)
		if err != nil {
			return nil, nil, err
		}
		metricRM = sdkmetric.NewPeriodicReader(metricExp)
	default:
		traceOpts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(cleanEndpoint)}
		metricOpts := []otlpmetricgrpc.Option{otlpmetricgrpc.WithEndpoint(cleanEndpoint)}
		if insecure {
			traceOpts = append(traceOpts, otlptracegrpc.WithInsecure())
			metricOpts = append(metricOpts, otlpmetricgrpc.WithInsecure())
		}
		traceExp, err = otlptracegrpc.New(ctx, traceOpts...)
		if err != nil {
			return nil, nil, err
		}
		metricExp, err := otlpmetricgrpc.New(ctx, metricOpts...)
		if err != nil {
			return nil, nil, err
		}
		metricRM = sdkmetric.NewPeriodicReader(metricExp)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithResource(res),
	)
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(metricRM),
		sdkmetric.WithResource(res),
	)
	return tp, mp, nil
}

func normalizeEndpoint(endpoint string) (string, bool) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", true
	}
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		u, err := url.Parse(endpoint)
		if err == nil && u.Host != "" {
			return u.Host, u.Scheme != "https"
		}
	}
	return endpoint, true
}

func parseResourceAttributes(csv string) map[string]string {
	out := map[string]string{}
	if strings.TrimSpace(csv) == "" {
		return out
	}
	parts := strings.Split(csv, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		k := strings.TrimSpace(kv[0])
		v := strings.TrimSpace(kv[1])
		if k == "" || v == "" {
			continue
		}
		out[k] = v
	}
	return out
}

func (o *agentObservability) initMetrics() {
	meter := otel.Meter("sympozium/agent-runner")
	var err error

	o.agentRuns, err = meter.Int64Counter("sympozium.agent.runs")
	if err != nil {
		log.Printf("failed creating metric sympozium.agent.runs: %v", err)
	}
	o.agentRunDurMs, err = meter.Float64Histogram("sympozium.agent.run.duration")
	if err != nil {
		log.Printf("failed creating metric sympozium.agent.run.duration: %v", err)
	}
	o.inTok, err = meter.Int64Counter("gen_ai.usage.input_tokens")
	if err != nil {
		log.Printf("failed creating metric gen_ai.usage.input_tokens: %v", err)
	}
	o.outTok, err = meter.Int64Counter("gen_ai.usage.output_tokens")
	if err != nil {
		log.Printf("failed creating metric gen_ai.usage.output_tokens: %v", err)
	}
	o.toolInvocations, err = meter.Int64Counter("sympozium.tool.invocations")
	if err != nil {
		log.Printf("failed creating metric sympozium.tool.invocations: %v", err)
	}
	o.skillDurMs, err = meter.Float64Histogram("sympozium.skill.duration")
	if err != nil {
		log.Printf("failed creating metric sympozium.skill.duration: %v", err)
	}
}

func (o *agentObservability) startRunSpan(ctx context.Context, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	if o == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
	return o.tracer.Start(ctx, "sympozium.agent.run", trace.WithAttributes(attrs...))
}

func (o *agentObservability) startChatSpan(ctx context.Context, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	if o == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
	return o.tracer.Start(ctx, "gen_ai.chat", trace.WithAttributes(attrs...))
}

func (o *agentObservability) startToolSpan(ctx context.Context, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	if o == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
	return o.tracer.Start(ctx, "gen_ai.execute_tool", trace.WithAttributes(attrs...))
}

func (o *agentObservability) startSkillSpan(ctx context.Context, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	if o == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
	return o.tracer.Start(ctx, "sympozium.skill.exec", trace.WithAttributes(attrs...))
}

func (o *agentObservability) recordRunMetrics(
	ctx context.Context,
	status, instance, model, namespace string,
	durationMs int64,
	inputTokens, outputTokens int,
) {
	if o == nil || !o.enabled {
		return
	}
	attrs := metric.WithAttributes(
		attribute.String("instance", instance),
		attribute.String("status", status),
		attribute.String("namespace", namespace),
		attribute.String("model", model),
	)
	if o.agentRuns != nil {
		o.agentRuns.Add(ctx, 1, attrs)
	}
	if o.agentRunDurMs != nil {
		o.agentRunDurMs.Record(ctx, float64(durationMs), attrs)
	}
	if inputTokens > 0 && o.inTok != nil {
		o.inTok.Add(ctx, int64(inputTokens), metric.WithAttributes(attribute.String("model", model)))
	}
	if outputTokens > 0 && o.outTok != nil {
		o.outTok.Add(ctx, int64(outputTokens), metric.WithAttributes(attribute.String("model", model)))
	}
}

func (o *agentObservability) recordToolInvocation(ctx context.Context, toolName, status string) {
	if o == nil || !o.enabled || o.toolInvocations == nil {
		return
	}
	o.toolInvocations.Add(ctx, 1, metric.WithAttributes(
		attribute.String("tool_name", toolName),
		attribute.String("status", status),
	))
}

func (o *agentObservability) recordSkillDuration(ctx context.Context, skillName string, d time.Duration) {
	if o == nil || !o.enabled || o.skillDurMs == nil {
		return
	}
	o.skillDurMs.Record(ctx, float64(d.Milliseconds()), metric.WithAttributes(
		attribute.String("skill_name", skillName),
	))
}

func markSpanError(span trace.Span, err error) {
	if span == nil || err == nil {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

func writeTraceContextMetadata(ctx context.Context) {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return
	}

	payload := map[string]string{
		"trace_id":      sc.TraceID().String(),
		"span_id":       sc.SpanID().String(),
		"traceparent":   formatTraceparent(sc),
		"agent_run_id":  getEnv("AGENT_RUN_ID", ""),
		"instance_name": getEnv("INSTANCE_NAME", ""),
		"namespace":     getEnv("AGENT_NAMESPACE", ""),
		"model":         getEnv("MODEL_NAME", ""),
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return
	}
	path := "/workspace/.sympozium/trace-context.json"
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, data, 0o644)
}

func traceMetadata(ctx context.Context) map[string]string {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return nil
	}
	return map[string]string{
		"trace_id":    sc.TraceID().String(),
		"span_id":     sc.SpanID().String(),
		"traceparent": formatTraceparent(sc),
	}
}

func formatTraceparent(sc trace.SpanContext) string {
	if !sc.IsValid() {
		return ""
	}
	flags := "00"
	if sc.TraceFlags().IsSampled() {
		flags = "01"
	}
	return fmt.Sprintf("00-%s-%s-%s", sc.TraceID().String(), sc.SpanID().String(), flags)
}

func logWithTrace(ctx context.Context, level, msg string, fields map[string]any) {
	entry := map[string]any{
		"time":  time.Now().UTC().Format(time.RFC3339Nano),
		"level": level,
		"msg":   msg,
	}
	for k, v := range fields {
		entry[k] = v
	}
	if meta := traceMetadata(ctx); meta != nil {
		entry["trace_id"] = meta["trace_id"]
		entry["span_id"] = meta["span_id"]
	}
	line, err := json.Marshal(entry)
	if err != nil {
		return
	}
	log.Println(string(line))
}
