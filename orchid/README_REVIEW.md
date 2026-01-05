# README Review: Issues and Gaps Analysis

## Executive Summary

This document identifies critical issues and gaps in the README requirements for Orchid, a horizontally scalable API polling microservice. The review focuses on areas that could impact scalability, reliability, flexibility, and operational excellence.

---

## 1. Horizontal Scalability & Distributed Execution

### Critical Gaps

**1.1 Plan Distribution & Work Assignment**
- **✅ Clarified**: README now specifies using Redis/Kafka queue for enqueueing plan/step execution
- **Remaining Gaps**: Still need details on:
  - Queue implementation details (Redis Streams, Kafka topics, etc.)
  - How to prevent duplicate execution when multiple instances consume from queue
  - Message acknowledgment and retry strategy for failed queue processing
  - How to handle instance failures and work reassignment
  - Queue partitioning strategy (per-plan, per-config, etc.)

**1.2 State Synchronization**
- **Issue**: Context fields, execution counts, and statistics need to be shared across instances
- **Gap**: No specification for:
  - How `context` fields are persisted and synchronized
  - How execution statistics are aggregated across instances
  - How to handle concurrent updates to shared state
  - Whether Citus (distributed PostgreSQL) is sufficient or if Redis is needed for real-time state

**1.3 Locking & Coordination**
- **Issue**: Mentions Redis for locks but lacks detail
- **Gap**: Missing specification for:
  - What exactly needs to be locked (plan execution, sub-step execution, rate limit buckets?)
  - Lock duration and timeout strategies
  - Deadlock prevention
  - Lock granularity (per-plan, per-config, per-endpoint?)

**1.4 Leader Election**
- **Issue**: No mention of coordination mechanism for distributed operations
- **Gap**: Need to specify:
  - Whether a leader is needed for certain operations (e.g., plan scheduling)
  - How to handle leader failures
  - Whether all instances are equal or if there's a coordinator pattern

---

## 2. Data Consistency & State Management

### Critical Gaps

**2.1 Execution State Persistence**
- **Issue**: Context fields, execution counts, and loop state need persistence
- **Gap**: Missing details on:
  - When context is saved (after each step? after each loop iteration?)
  - How to handle partial executions (crash recovery)
  - Transaction boundaries for multi-step plans
  - How to resume execution after a crash

**2.2 Duplicate Prevention**
- **Issue**: Risk of duplicate API calls or Kafka messages in distributed setup
- **Gap**: Need to specify:
  - Idempotency keys for API calls
  - Deduplication strategy for Kafka messages
  - How to handle retries without duplicates

**2.3 Statistics Aggregation**
- **Issue**: Statistics mentioned but aggregation strategy unclear
- **Gap**: Missing specification for:
  - Real-time vs. eventual consistency for statistics
  - How to aggregate stats across instances
  - Whether stats are stored per-instance or globally

---

## 3. Error Handling & Resilience

### Critical Gaps

**3.1 Retry Strategy Details**
- **✅ Clarified**: Users can define retry events and maximum retry counts per plan/step
- **Remaining Gaps**: Still need details on:
  - Exponential backoff vs. fixed delay
  - Jitter for retry delays
  - Whether retries count against rate limits
  - How to distinguish transient vs. permanent errors
  - Default retry behavior if not specified

**3.2 Failure Recovery**
- **Issue**: No specification for handling worker crashes
- **Gap**: Need to define:
  - How to detect and recover from crashed executions
  - Timeout mechanisms for stuck plans
  - How to handle partial sub-step execution
  - Dead letter queue for unrecoverable failures

**3.3 Kafka Failure Handling**
- **Issue**: No mention of what happens if Kafka is unavailable
- **Gap**: Missing specification for:
  - Buffering strategy when Kafka is down
  - Retry mechanism for failed Kafka publishes
  - Message ordering guarantees
  - At-least-once vs. exactly-once delivery semantics

**3.4 Circuit Breakers**
- **Issue**: No mention of circuit breakers for external APIs
- **Gap**: Should specify:
  - When to open circuit breakers
  - How to handle cascading failures
  - Recovery strategies

---

## 4. Flexibility & API Design Gaps

### Critical Gaps

**4.1 Response Parsing & Transformation**
- **✅ Clarified**: Orchid emits raw data as received from APIs. Data transformations are handled by the sister project "Lotus"
- **Remaining Considerations**: May still need to specify:
  - Support for different response formats (JSON, XML, CSV, binary) - how are these handled in raw form?
  - Response size limits and handling of large responses
  - Whether any minimal parsing is needed to extract data for sub-steps or condition evaluation

**4.2 Conditional Logic**
- **Issue**: Limited conditional logic (while, abort_when, retry_when, break_when)
- **Gap**: Consider adding:
  - Conditional sub-step execution
  - Branching logic (if/else)
  - Data-driven plan selection
  - Dynamic URL construction based on response data

**4.3 Data Enrichment & Joining**
- **Issue**: No mention of combining data from multiple API calls
- **Gap**: Missing specification for:
  - How to join data from parent and sub-steps
  - Data enrichment patterns
  - Aggregating data across multiple calls before emitting

**4.4 Dynamic Parameter Resolution**
- **Issue**: Params are static or from context
- **Gap**: Consider:
  - Extracting params from previous responses
  - Iterating over arrays in responses for sub-steps
  - Dynamic header generation based on auth or config

**4.5 Request Body Generation**
- **Issue**: Request body mentioned but format unclear
- **Gap**: Need to specify:
  - Template language for request bodies
  - How to reference other variables in body
  - Support for different content types (JSON, form-data, etc.)

---

## 5. Rate Limiting & Concurrency

### Critical Gaps

**5.1 Rate Limit Implementation**
- **Issue**: Rate limiting mentioned but implementation unclear
- **Gap**: Missing specification for:
  - Token bucket vs. sliding window algorithm
  - How rate limits are shared across instances (Redis?)
  - Priority queue implementation details
  - How to handle rate limit headers from APIs (429 responses)

**5.2 Concurrency Control**
- **Issue**: Concurrency limits mentioned but details missing
- **Gap**: Need to specify:
  - Per-endpoint vs. per-plan concurrency limits
  - How to queue requests when limit reached
  - Fairness algorithms for concurrent requests
  - How sub-steps affect concurrency limits

**5.3 Fanout Strategy**
- **Issue**: Mentions maximizing concurrency with fanout
- **Gap**: Missing specification for:
  - Maximum fanout depth/width
  - Resource limits per plan
  - How to prevent runaway fanout
  - Backpressure mechanisms

---

## 6. Authentication & Security

### Critical Gaps

**6.1 Auth Flow Details**
- **Issue**: Auth mechanisms mentioned but implementation unclear
- **Gap**: Missing specification for:
  - Supported auth types (OAuth2, API keys, Basic, Custom)
  - How to define custom auth flows
  - Token refresh strategies
  - How auth failures are handled

**6.2 Secret Management**
- **Issue**: Secrets mentioned but storage/access unclear
- **Gap**: Need to specify:
  - Where secrets are stored (encrypted at rest?)
  - Secret rotation mechanisms
  - How secrets are accessed at runtime
  - Audit logging for secret access

**6.3 Auth Caching & Skew**
- **Issue**: Mentions cache and TTL but details missing
- **Gap**: Missing specification for:
  - Cache invalidation strategy
  - How skew mechanism works
  - Cache sharing across instances
  - Handling stale tokens

---

## 7. Observability & Monitoring

### Critical Gaps

**7.1 Metrics & Statistics**
- **Issue**: Statistics mentioned but details sparse
- **Gap**: Need to specify:
  - What metrics are tracked (latency, error rates, throughput?)
  - How metrics are exposed (Prometheus, custom endpoint?)
  - Metric retention and aggregation
  - Per-plan vs. aggregate metrics

**7.2 Logging Strategy**
- **Issue**: No mention of logging requirements
- **Gap**: Missing specification for:
  - Log levels and verbosity
  - Structured logging format
  - Log aggregation strategy
  - PII/sensitive data handling in logs

**7.3 Distributed Tracing**
- **Issue**: No mention of tracing across plan executions
- **Gap**: Should specify:
  - Trace context propagation through plans and sub-steps
  - How to correlate traces across instances
  - Trace sampling strategy

**7.4 Alerting**
- **Issue**: No mention of alerting requirements
- **Gap**: Need to specify:
  - What conditions trigger alerts
  - Alert channels and routing
  - Alert deduplication

---

## 8. Operational Concerns

### Critical Gaps

**8.1 Plan Lifecycle Management**
- **Issue**: Enable/disable mentioned but details missing
- **Gap**: Missing specification for:
  - Graceful shutdown of running plans
  - How to handle in-flight executions when disabling
  - Plan versioning and rollback
  - Plan migration strategies

**8.2 Configuration Management**
- **Issue**: Config schema mentioned but details unclear
- **Gap**: Need to specify:
  - Config validation rules
  - Config versioning
  - Hot-reloading vs. restart required
  - Config rollback mechanisms

**8.3 Resource Management**
- **Issue**: No mention of resource limits
- **Gap**: Missing specification for:
  - Memory limits per plan/instance
  - CPU throttling
  - Network bandwidth limits
  - How to handle resource exhaustion

**8.4 Deployment & Scaling**
- **Issue**: No mention of deployment strategy
- **Gap**: Need to specify:
  - Rolling updates vs. blue-green
  - How to handle plan execution during deployments
  - Auto-scaling triggers
  - Health check endpoints

---

## 9. Data & Message Handling

### Critical Gaps

**9.1 Kafka Integration Details**
- **Issue**: Kafka mentioned but integration details missing
- **Gap**: Missing specification for:
  - Topic naming strategy
  - Partitioning strategy
  - Message format/schema
  - Key selection for partitioning
  - Compression and serialization format

**9.2 Data Validation**
- **Issue**: No mention of data validation
- **Gap**: Should specify:
  - Schema validation for responses
  - Data quality checks
  - How to handle invalid data

**9.3 Message Ordering**
- **Issue**: No mention of ordering guarantees
- **Gap**: Need to specify:
  - Whether messages must be ordered
  - How to maintain order in distributed setup
  - Handling out-of-order messages

---

## 10. Edge Cases & Special Scenarios

### Critical Gaps

**10.1 Timezone Handling**
- **Issue**: No mention of timezone handling
- **Gap**: Need to specify:
  - How timestamps are handled
  - Timezone for wait intervals
  - Daylight saving time handling

**10.2 Large Response Handling**
- **Issue**: No mention of handling large responses
- **Gap**: Missing specification for:
  - Streaming vs. buffering large responses
  - Memory limits for response bodies
  - Chunking strategies

**10.3 Long-Running Plans**
- **Issue**: No mention of execution time limits
- **Gap**: Need to specify:
  - Maximum execution time per plan
  - How to handle plans that exceed limits
  - Checkpointing for long-running plans

**10.4 API Versioning**
- **Issue**: No mention of API versioning support
- **Gap**: Should specify:
  - How to handle API version changes
  - Version negotiation strategies
  - Backward compatibility

---

## 11. Documentation & Clarity Issues

### Typos & Grammar
- Line 1: "plaform" → "platform"
- Line 3: "resposible" → "responsible"
- Line 17: "set up" → "set of"
- Line 17: "Sub-steps make also have" → "Sub-steps may also have"
- Line 41: "condtion" → "condition"
- Line 55: "Added like" → "Added like" (unclear phrasing)
- Line 55: "conext" → "context"

### Clarity Issues
- Line 9: "In theory the users can provide multiple configs" - unclear if this is a requirement or future consideration
- Line 13: "skew mechanism" - needs definition
- Line 35: "without things like `repeat_count`, or `wait`" - should list all excluded features
- Line 45: "That may necessitate" - should be more definitive about when Redis is needed
- Line 55: "Added like" - unclear phrasing, should be "Such as" or "Including"

---

## Recommendations

### High Priority
1. **Define work distribution strategy** - Critical for horizontal scalability
2. **Specify state synchronization mechanism** - Essential for consistency
3. **Detail retry and error handling** - Critical for reliability
4. **Clarify Kafka integration** - Core functionality
5. **Define rate limiting implementation** - Important for API compliance

### Medium Priority
6. **Add response parsing/transformation** - Important for flexibility
7. **Specify observability requirements** - Important for operations
8. **Detail auth flow implementation** - Important for security
9. **Define resource limits** - Important for stability

### Low Priority
10. **Fix typos and improve clarity** - Improves maintainability
11. **Add edge case handling** - Improves robustness

---

## Questions for Clarification

1. What is the expected scale? (number of plans, instances, API calls per second)
2. What are the SLA requirements? (availability, latency, throughput)
3. What is the disaster recovery strategy?
4. How are plans versioned and migrated?
5. What is the expected data volume per plan execution?
6. Are there compliance requirements? (GDPR, SOC2, etc.)
7. What is the expected API diversity? (REST, GraphQL, SOAP, etc.)
8. How should the system handle API deprecations?

