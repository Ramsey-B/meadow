# Orchid Consumer Guide

This document describes Orchid's functionality and output format for downstream consumers.

---

## What is Orchid?

**Orchid** is a horizontally scalable API polling microservice that:

1. **Executes scheduled API polling plans** - Fetches data from external APIs on configurable schedules
2. **Handles authentication** - Manages OAuth tokens, API keys, and custom auth flows
3. **Supports complex workflows** - Pagination, nested requests, conditional logic, and data extraction
4. **Emits raw API responses** - Publishes all API responses to Kafka for downstream processing

---

## Architecture Overview

```
┌─────────────────┐     ┌──────────────┐     ┌─────────────────┐
│  External APIs  │────▶│    Orchid    │────▶│      Kafka      │
└─────────────────┘     └──────────────┘     └─────────────────┘
                              │                      │
                              ▼                      ▼
                        ┌──────────┐          ┌─────────────┐
                        │ Postgres │          │   Lotus     │
                        │ (state)  │          │ (consumer)  │
                        └──────────┘          └─────────────┘
```

---

## Kafka Topic

**Default Topic:** `api-responses`

Configurable via environment variable: `KAFKA_RESPONSE_TOPIC`

---

## Message Format

Each Kafka message contains a JSON payload representing a single API response.

### Message Schema

```json
{
  "tenant_id": "uuid",
  "plan_id": "uuid",
  "config_id": "uuid",
  "execution_id": "uuid",
  "step_path": "string",
  "timestamp": "ISO8601 datetime",
  
  "request_url": "string",
  "request_method": "GET|POST|PUT|DELETE|PATCH",
  "request_headers": { "key": "value" },
  
  "status_code": 200,
  "response_body": { /* raw JSON response */ },
  "response_headers": { "key": "value" },
  "response_size": 12345,
  "duration_ms": 150,
  
  "extracted_data": { /* optional, JMESPath-extracted fields */ },
  
  "trace_id": "string",
  "span_id": "string"
}
```

### Field Descriptions

| Field | Type | Description |
|-------|------|-------------|
| `tenant_id` | UUID | Identifies the tenant/organization |
| `plan_id` | UUID | The polling plan that generated this response |
| `config_id` | UUID | The configuration used (contains API credentials, etc.) |
| `execution_id` | UUID | Unique ID for this execution run |
| `step_path` | string | Identifies which step generated this (e.g., `root`, `root.sub[0]`) |
| `timestamp` | datetime | When the response was received |
| `request_url` | string | The full URL that was called |
| `request_method` | string | HTTP method used |
| `request_headers` | object | Headers sent with the request (sensitive values redacted) |
| `status_code` | int | HTTP status code |
| `response_body` | object | Raw JSON response body |
| `response_headers` | object | Response headers |
| `response_size` | int | Response body size in bytes |
| `duration_ms` | int | Request duration in milliseconds |
| `extracted_data` | object | Optional data extracted via JMESPath expressions |
| `trace_id` | string | OpenTelemetry trace ID for distributed tracing |
| `span_id` | string | OpenTelemetry span ID |

---

## Kafka Message Headers

Each message includes headers for efficient filtering:

| Header | Description |
|--------|-------------|
| `tenant_id` | Tenant identifier |
| `plan_id` | Plan identifier |
| `execution_id` | Execution identifier |
| `traceparent` | W3C Trace Context propagation header |

---

## Message Key

Messages are keyed by: `{tenant_id}:{plan_id}:{execution_id}`

This ensures all messages from the same execution are routed to the same partition.

---

## Example Messages

### Simple GET Response

```json
{
  "tenant_id": "550e8400-e29b-41d4-a716-446655440000",
  "plan_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "config_id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
  "execution_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "step_path": "root",
  "timestamp": "2025-01-15T10:30:00Z",
  "request_url": "https://api.example.com/users",
  "request_method": "GET",
  "request_headers": {
    "Authorization": "[REDACTED]",
    "Content-Type": "application/json"
  },
  "status_code": 200,
  "response_body": {
    "users": [
      {"id": 1, "name": "Alice", "email": "alice@example.com"},
      {"id": 2, "name": "Bob", "email": "bob@example.com"}
    ],
    "total": 2,
    "page": 1
  },
  "response_headers": {
    "Content-Type": "application/json",
    "X-RateLimit-Remaining": "99"
  },
  "response_size": 256,
  "duration_ms": 145,
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "span_id": "00f067aa0ba902b7"
}
```

### Nested/Paginated Response

When Orchid paginates or iterates over items, each sub-request generates its own message:

```json
{
  "tenant_id": "550e8400-e29b-41d4-a716-446655440000",
  "plan_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "config_id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
  "execution_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "step_path": "root.user_details[0]",
  "timestamp": "2025-01-15T10:30:01Z",
  "request_url": "https://api.example.com/users/1/details",
  "request_method": "GET",
  "status_code": 200,
  "response_body": {
    "id": 1,
    "name": "Alice",
    "department": "Engineering",
    "manager": "Carol"
  },
  "duration_ms": 89
}
```

---

## Consumer Implementation Notes

### 1. Idempotency

Use `execution_id` + `step_path` as a unique key to handle duplicate messages.

### 2. Ordering

Messages within the same execution are not guaranteed to arrive in order. Use `timestamp` if order matters.

### 3. Error Responses

Orchid emits messages for both successful and failed API calls. Check `status_code` to identify failures.

### 4. Tracing

To correlate logs/traces, extract the `traceparent` header and propagate it to your observability stack.

### 5. Multi-Tenancy

Always filter by `tenant_id` to ensure proper data isolation.

---

## Orchid Concepts

### Integration
A third-party API definition (e.g., "Salesforce", "GitHub", "Stripe").

### Config Schema
Defines what configuration fields an integration requires (API keys, base URLs, etc.).

### Config
A specific instance of credentials/settings for an integration (belongs to a tenant).

### Plan
A polling workflow definition that specifies:
- Which API endpoints to call
- Request parameters (URL, headers, body)
- Scheduling (cron expression)
- Pagination/iteration logic
- Data extraction expressions

### Execution
A single run of a plan, which may produce multiple Kafka messages (one per API call).

---

## Rate Limiting

Orchid respects rate limits defined in plans and dynamically adjusts based on API response headers:
- `X-RateLimit-Remaining`
- `X-RateLimit-Reset`
- `Retry-After`

---

## Dead Letter Queue

Failed jobs (after max retries) are moved to a Redis-based DLQ. These can be inspected and retried via the Orchid API:

- `GET /api/v1/dlq` - List failed jobs
- `POST /api/v1/dlq/:id/retry` - Retry a failed job
- `DELETE /api/v1/dlq/:id` - Discard a failed job

---

## Observability

Orchid exposes:
- **Prometheus metrics** at `/metrics`
- **Health checks** at `/health`, `/health/live`, `/health/ready`
- **OpenTelemetry traces** (configurable export to Jaeger/OTLP)

---

## Contact

For questions about Orchid's output format or integration assistance, refer to the main project documentation or open an issue in the repository.

