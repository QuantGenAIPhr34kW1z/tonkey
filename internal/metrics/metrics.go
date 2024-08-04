package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTP metrics
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tonkey_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tonkey_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	// Authentication metrics
	AuthAttempts = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tonkey_auth_attempts_total",
			Help: "Total authentication attempts",
		},
		[]string{"result"}, // success, failure
	)

	JWTTokensIssued = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "tonkey_jwt_tokens_issued_total",
			Help: "Total number of JWT tokens issued",
		},
	)

	// TON provider metrics
	ProviderCalls = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tonkey_provider_calls_total",
			Help: "Total calls to TON provider",
		},
		[]string{"method", "result"}, // Balance/Send/TxStatus, success/error
	)

	ProviderCallDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tonkey_provider_call_duration_seconds",
			Help:    "TON provider call duration",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"method"},
	)

	// Database metrics
	DBQueriesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tonkey_db_queries_total",
			Help: "Total database queries",
		},
		[]string{"operation", "result"}, // select/insert/update/delete, success/error
	)

	DBQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tonkey_db_query_duration_seconds",
			Help:    "Database query duration",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
		[]string{"operation"},
	)

	// Transaction metrics
	TransactionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tonkey_transactions_total",
			Help: "Total transactions processed",
		},
		[]string{"status"}, // submitted, confirmed, failed
	)

	TransactionAmount = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "tonkey_transaction_amount_nanoton",
			Help:    "Transaction amounts in nanoTON",
			Buckets: prometheus.ExponentialBuckets(1000, 10, 10),
		},
	)

	// WebSocket metrics
	WebSocketConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "tonkey_websocket_connections",
			Help: "Current WebSocket connections",
		},
	)

	WebSocketMessagesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tonkey_websocket_messages_total",
			Help: "Total WebSocket messages",
		},
		[]string{"direction", "type"}, // in/out, subscribe/unsubscribe/data
	)

	// Webhook metrics
	WebhookDeliveries = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tonkey_webhook_deliveries_total",
			Help: "Total webhook deliveries",
		},
		[]string{"result"}, // success, failure
	)

	WebhookDeliveryDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "tonkey_webhook_delivery_duration_seconds",
			Help:    "Webhook delivery duration",
			Buckets: []float64{.1, .25, .5, 1, 2.5, 5, 10, 30},
		},
	)

	// Rate limiting metrics
	RateLimitHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tonkey_rate_limit_hits_total",
			Help: "Total rate limit hits",
		},
		[]string{"key_type"}, // ip, user
	)

	// System metrics
	ActiveUsers = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "tonkey_active_users",
			Help: "Number of active users (logged in last 24h)",
		},
	)

	ActiveOrganizations = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "tonkey_active_organizations",
			Help: "Number of active organizations",
		},
	)
)

// RecordHTTPRequest records an HTTP request with timing
func RecordHTTPRequest(method, endpoint string, statusCode int, duration time.Duration) {
	status := statusCodeToString(statusCode)
	HTTPRequestsTotal.WithLabelValues(method, endpoint, status).Inc()
	HTTPRequestDuration.WithLabelValues(method, endpoint).Observe(duration.Seconds())
}

// RecordAuthAttempt records an authentication attempt
func RecordAuthAttempt(success bool) {
	result := "failure"
	if success {
		result = "success"
	}
	AuthAttempts.WithLabelValues(result).Inc()
}

// RecordProviderCall records a TON provider call with timing
func RecordProviderCall(method string, err error, duration time.Duration) {
	result := "success"
	if err != nil {
		result = "error"
	}
	ProviderCalls.WithLabelValues(method, result).Inc()
	ProviderCallDuration.WithLabelValues(method).Observe(duration.Seconds())
}

// RecordDBQuery records a database query with timing
func RecordDBQuery(operation string, err error, duration time.Duration) {
	result := "success"
	if err != nil {
		result = "error"
	}
	DBQueriesTotal.WithLabelValues(operation, result).Inc()
	DBQueryDuration.WithLabelValues(operation).Observe(duration.Seconds())
}

// RecordTransaction records a transaction
func RecordTransaction(status string, amount int64) {
	TransactionsTotal.WithLabelValues(status).Inc()
	TransactionAmount.Observe(float64(amount))
}

// RecordWebhookDelivery records a webhook delivery attempt
func RecordWebhookDelivery(success bool, duration time.Duration) {
	result := "failure"
	if success {
		result = "success"
	}
	WebhookDeliveries.WithLabelValues(result).Inc()
	WebhookDeliveryDuration.Observe(duration.Seconds())
}

// statusCodeToString converts HTTP status code to a string category
func statusCodeToString(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "2xx"
	case code >= 300 && code < 400:
		return "3xx"
	case code >= 400 && code < 500:
		return "4xx"
	case code >= 500:
		return "5xx"
	default:
		return "unknown"
	}
}
