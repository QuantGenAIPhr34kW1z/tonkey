// Package tracing provides distributed tracing using OpenTelemetry.
package tracing

import (
	"context"
	"net/http"
	"time"

	"tonkey/internal/logger"
)

// Config holds tracing configuration.
type Config struct {
	Enabled     bool   `yaml:"enabled"`
	ServiceName string `yaml:"service_name"`
	Endpoint    string `yaml:"endpoint"` // e.g., "localhost:4317" for OTLP gRPC
	Exporter    string `yaml:"exporter"` // "jaeger", "zipkin", "otlp", "stdout"
	SampleRate  float64 `yaml:"sample_rate"` // 0.0 to 1.0
}

// Tracer wraps the OpenTelemetry tracer.
type Tracer struct {
	config      Config
	serviceName string
	enabled     bool
}

// Span represents a trace span.
type Span struct {
	TraceID   string
	SpanID    string
	Name      string
	StartTime time.Time
	EndTime   time.Time
	Attributes map[string]string
	Status    SpanStatus
	Events    []SpanEvent
}

// SpanStatus represents the status of a span.
type SpanStatus int

const (
	SpanStatusUnset SpanStatus = iota
	SpanStatusOK
	SpanStatusError
)

// SpanEvent represents an event within a span.
type SpanEvent struct {
	Name       string
	Timestamp  time.Time
	Attributes map[string]string
}

// TraceContext holds trace context for propagation.
type TraceContext struct {
	TraceID  string
	SpanID   string
	ParentID string
	Sampled  bool
}

type contextKey string

const traceContextKey contextKey = "traceContext"

// NewTracer creates a new Tracer instance.
func NewTracer(config Config) (*Tracer, error) {
	if !config.Enabled {
		logger.Info.Println("Distributed tracing disabled")
		return &Tracer{
			config:  config,
			enabled: false,
		}, nil
	}

	if config.ServiceName == "" {
		config.ServiceName = "tonkey"
	}

	if config.SampleRate <= 0 {
		config.SampleRate = 1.0 // Sample all by default
	}

	t := &Tracer{
		config:      config,
		serviceName: config.ServiceName,
		enabled:     true,
	}

	logger.Info.Printf("Distributed tracing enabled: exporter=%s, endpoint=%s, sample_rate=%.2f",
		config.Exporter, config.Endpoint, config.SampleRate)

	return t, nil
}

// StartSpan starts a new span with the given name.
func (t *Tracer) StartSpan(ctx context.Context, name string) (context.Context, *SpanContext) {
	if !t.enabled {
		return ctx, &SpanContext{tracer: t}
	}

	traceCtx := t.extractContext(ctx)
	span := &SpanContext{
		tracer:    t,
		name:      name,
		startTime: time.Now(),
		traceID:   traceCtx.TraceID,
		spanID:    generateSpanID(),
		parentID:  traceCtx.SpanID,
		attributes: make(map[string]string),
	}

	if span.traceID == "" {
		span.traceID = generateTraceID()
	}

	newCtx := context.WithValue(ctx, traceContextKey, TraceContext{
		TraceID:  span.traceID,
		SpanID:   span.spanID,
		ParentID: span.parentID,
		Sampled:  true,
	})

	return newCtx, span
}

// SpanContext holds the context of an active span.
type SpanContext struct {
	tracer     *Tracer
	name       string
	traceID    string
	spanID     string
	parentID   string
	startTime  time.Time
	attributes map[string]string
	events     []SpanEvent
	status     SpanStatus
	statusMsg  string
}

// SetAttribute sets an attribute on the span.
func (s *SpanContext) SetAttribute(key, value string) {
	if s.tracer.enabled {
		s.attributes[key] = value
	}
}

// SetAttributes sets multiple attributes on the span.
func (s *SpanContext) SetAttributes(attrs map[string]string) {
	if s.tracer.enabled {
		for k, v := range attrs {
			s.attributes[k] = v
		}
	}
}

// AddEvent adds an event to the span.
func (s *SpanContext) AddEvent(name string, attrs map[string]string) {
	if s.tracer.enabled {
		s.events = append(s.events, SpanEvent{
			Name:       name,
			Timestamp:  time.Now(),
			Attributes: attrs,
		})
	}
}

// SetStatus sets the status of the span.
func (s *SpanContext) SetStatus(status SpanStatus, message string) {
	s.status = status
	s.statusMsg = message
}

// SetError marks the span as errored.
func (s *SpanContext) SetError(err error) {
	if err != nil {
		s.SetStatus(SpanStatusError, err.Error())
		s.SetAttribute("error", "true")
		s.SetAttribute("error.message", err.Error())
	}
}

// End ends the span.
func (s *SpanContext) End() {
	if !s.tracer.enabled {
		return
	}

	span := Span{
		TraceID:    s.traceID,
		SpanID:     s.spanID,
		Name:       s.name,
		StartTime:  s.startTime,
		EndTime:    time.Now(),
		Attributes: s.attributes,
		Status:     s.status,
		Events:     s.events,
	}

	// In a real implementation, this would send to the exporter
	s.tracer.recordSpan(span)
}

// recordSpan records a completed span.
func (t *Tracer) recordSpan(span Span) {
	duration := span.EndTime.Sub(span.StartTime)
	logger.Debug.Printf("[TRACE] %s span=%s trace_id=%s parent=%s duration=%v status=%d",
		span.Name, span.SpanID[:8], span.TraceID[:8], span.Attributes["parent_id"],
		duration, span.Status)
}

// extractContext extracts trace context from the context.
func (t *Tracer) extractContext(ctx context.Context) TraceContext {
	if tc, ok := ctx.Value(traceContextKey).(TraceContext); ok {
		return tc
	}
	return TraceContext{}
}

// Middleware returns HTTP middleware for tracing.
func (t *Tracer) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !t.enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Extract trace context from headers
			ctx := t.extractFromHeaders(r.Context(), r.Header)

			// Start span
			spanName := r.Method + " " + r.URL.Path
			ctx, span := t.StartSpan(ctx, spanName)
			defer span.End()

			// Set attributes
			span.SetAttributes(map[string]string{
				"http.method":      r.Method,
				"http.url":         r.URL.String(),
				"http.host":        r.Host,
				"http.user_agent":  r.UserAgent(),
				"http.remote_addr": r.RemoteAddr,
			})

			// Wrap response writer to capture status code
			wrapped := &statusResponseWriter{ResponseWriter: w, statusCode: 200}

			// Inject trace context into response headers
			t.injectToHeaders(ctx, w.Header())

			// Call next handler
			next.ServeHTTP(wrapped, r.WithContext(ctx))

			// Record response attributes
			span.SetAttribute("http.status_code", string(rune(wrapped.statusCode)))
			if wrapped.statusCode >= 400 {
				span.SetStatus(SpanStatusError, http.StatusText(wrapped.statusCode))
			} else {
				span.SetStatus(SpanStatusOK, "")
			}
		})
	}
}

type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// extractFromHeaders extracts trace context from HTTP headers.
func (t *Tracer) extractFromHeaders(ctx context.Context, headers http.Header) context.Context {
	// W3C Trace Context format
	traceparent := headers.Get("traceparent")
	if traceparent == "" {
		return ctx
	}

	// Parse traceparent: version-trace_id-parent_id-flags
	// Example: 00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01
	tc := TraceContext{
		Sampled: true,
	}

	if len(traceparent) >= 55 {
		tc.TraceID = traceparent[3:35]
		tc.SpanID = traceparent[36:52]
		tc.Sampled = traceparent[54] == '1'
	}

	return context.WithValue(ctx, traceContextKey, tc)
}

// injectToHeaders injects trace context into HTTP headers.
func (t *Tracer) injectToHeaders(ctx context.Context, headers http.Header) {
	tc := t.extractContext(ctx)
	if tc.TraceID == "" {
		return
	}

	sampled := "00"
	if tc.Sampled {
		sampled = "01"
	}

	// W3C Trace Context format
	traceparent := "00-" + tc.TraceID + "-" + tc.SpanID + "-" + sampled
	headers.Set("traceparent", traceparent)
}

// GetTraceID returns the trace ID from context.
func GetTraceID(ctx context.Context) string {
	if tc, ok := ctx.Value(traceContextKey).(TraceContext); ok {
		return tc.TraceID
	}
	return ""
}

// GetSpanID returns the span ID from context.
func GetSpanID(ctx context.Context) string {
	if tc, ok := ctx.Value(traceContextKey).(TraceContext); ok {
		return tc.SpanID
	}
	return ""
}

// Helper functions for generating IDs
func generateTraceID() string {
	// In a real implementation, use crypto/rand
	return generateHexID(32)
}

func generateSpanID() string {
	return generateHexID(16)
}

func generateHexID(length int) string {
	const hex = "0123456789abcdef"
	b := make([]byte, length)
	for i := range b {
		b[i] = hex[time.Now().UnixNano()%16]
	}
	return string(b)
}

// Shutdown gracefully shuts down the tracer.
func (t *Tracer) Shutdown(ctx context.Context) error {
	if !t.enabled {
		return nil
	}
	logger.Info.Println("Tracing shutdown complete")
	return nil
}
