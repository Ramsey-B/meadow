// Package metrics provides Prometheus metrics for the Orchid service.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// PlanExecutionsTotal tracks total plan executions by status
	PlanExecutionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "orchid",
			Subsystem: "execution",
			Name:      "plans_total",
			Help:      "Total number of plan executions by status",
		},
		[]string{"tenant_id", "integration_id", "status"},
	)

	// PlanExecutionDuration tracks plan execution duration in seconds
	PlanExecutionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "orchid",
			Subsystem: "execution",
			Name:      "plan_duration_seconds",
			Help:      "Duration of plan executions in seconds",
			Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 120, 300},
		},
		[]string{"tenant_id", "integration_id"},
	)

	// HTTPRequestsTotal tracks outbound HTTP requests
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "orchid",
			Subsystem: "http_client",
			Name:      "requests_total",
			Help:      "Total number of outbound HTTP requests",
		},
		[]string{"method", "status_code"},
	)

	// HTTPRequestDuration tracks outbound HTTP request duration
	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "orchid",
			Subsystem: "http_client",
			Name:      "request_duration_seconds",
			Help:      "Duration of outbound HTTP requests in seconds",
			Buckets:   []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method"},
	)

	// QueueJobsProcessed tracks jobs processed from the queue
	QueueJobsProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "orchid",
			Subsystem: "queue",
			Name:      "jobs_processed_total",
			Help:      "Total number of jobs processed from the queue",
		},
		[]string{"status"},
	)

	// QueueJobsInFlight tracks jobs currently being processed
	QueueJobsInFlight = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "orchid",
			Subsystem: "queue",
			Name:      "jobs_in_flight",
			Help:      "Number of jobs currently being processed",
		},
	)

	// DLQJobsTotal tracks jobs sent to the dead letter queue
	DLQJobsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "orchid",
			Subsystem: "dlq",
			Name:      "jobs_total",
			Help:      "Total number of jobs sent to dead letter queue",
		},
		[]string{"tenant_id", "reason"},
	)

	// SchedulerPlansScheduled tracks plans scheduled
	SchedulerPlansScheduled = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "orchid",
			Subsystem: "scheduler",
			Name:      "plans_scheduled_total",
			Help:      "Total number of plans scheduled",
		},
	)

	// RateLimitHits tracks rate limit hits
	RateLimitHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "orchid",
			Subsystem: "ratelimit",
			Name:      "hits_total",
			Help:      "Total number of rate limit hits",
		},
		[]string{"tenant_id", "limit_name"},
	)

	// RateLimitWaitTime tracks time spent waiting for rate limits
	RateLimitWaitTime = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "orchid",
			Subsystem: "ratelimit",
			Name:      "wait_seconds",
			Help:      "Time spent waiting for rate limits in seconds",
			Buckets:   []float64{0.01, 0.1, 0.5, 1, 2, 5, 10, 30},
		},
		[]string{"tenant_id", "limit_name"},
	)

	// KafkaMessagesPublished tracks Kafka messages published
	KafkaMessagesPublished = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "orchid",
			Subsystem: "kafka",
			Name:      "messages_published_total",
			Help:      "Total number of messages published to Kafka",
		},
		[]string{"topic", "status"},
	)

	// KafkaPublishDuration tracks Kafka publish duration
	KafkaPublishDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "orchid",
			Subsystem: "kafka",
			Name:      "publish_duration_seconds",
			Help:      "Duration of Kafka publish operations in seconds",
			Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5},
		},
	)

	// AuthTokenRefreshes tracks auth token refresh operations
	AuthTokenRefreshes = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "orchid",
			Subsystem: "auth",
			Name:      "token_refreshes_total",
			Help:      "Total number of auth token refresh operations",
		},
		[]string{"tenant_id", "status"},
	)

	// DatabaseQueryDuration tracks database query duration
	DatabaseQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "orchid",
			Subsystem: "database",
			Name:      "query_duration_seconds",
			Help:      "Duration of database queries in seconds",
			Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		},
		[]string{"operation"},
	)

	// RedisOperationDuration tracks Redis operation duration
	RedisOperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "orchid",
			Subsystem: "redis",
			Name:      "operation_duration_seconds",
			Help:      "Duration of Redis operations in seconds",
			Buckets:   []float64{0.0001, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1},
		},
		[]string{"operation"},
	)
)

// RecordPlanExecution records a plan execution metric
func RecordPlanExecution(tenantID, integrationID, status string, durationSeconds float64) {
	PlanExecutionsTotal.WithLabelValues(tenantID, integrationID, status).Inc()
	PlanExecutionDuration.WithLabelValues(tenantID, integrationID).Observe(durationSeconds)
}

// RecordHTTPRequest records an outbound HTTP request metric
func RecordHTTPRequest(method, statusCode string, durationSeconds float64) {
	HTTPRequestsTotal.WithLabelValues(method, statusCode).Inc()
	HTTPRequestDuration.WithLabelValues(method).Observe(durationSeconds)
}

// RecordQueueJob records a queue job processing metric
func RecordQueueJob(status string) {
	QueueJobsProcessed.WithLabelValues(status).Inc()
}

// RecordDLQJob records a dead letter queue job
func RecordDLQJob(tenantID, reason string) {
	DLQJobsTotal.WithLabelValues(tenantID, reason).Inc()
}

// RecordKafkaPublish records a Kafka publish operation
func RecordKafkaPublish(topic, status string, durationSeconds float64) {
	KafkaMessagesPublished.WithLabelValues(topic, status).Inc()
	KafkaPublishDuration.Observe(durationSeconds)
}

