# Suggestions for README Enhancements

## Key Design Decision: JMESPath Everywhere ✅

**Orchid uses JMESPath as the single expression language for all dynamic behavior:**

- Conditions (`while`, `abort_when`, `retry_when`, `break_when`)
- Parameter and header extraction
- Request body generation
- Sub-step iteration (`iterate_over`)
- Context setting and access
- Auth token extraction
- Dynamic rate limiting
- Conditional sub-step execution

This provides consistency, reduces learning curve, and simplifies implementation. All suggestions below assume JMESPath usage.

---

## High Priority - Critical for Flexibility

### 1. **Condition/Expression Language** ⭐ RECOMMENDED: JMESPath

**Current Gap**: Conditions (`while`, `abort_when`, `retry_when`, `break_when`) are mentioned but the syntax/language is undefined.

**Recommendation**: Use **JMESPath** as the primary expression language with helper functions for common patterns.

**Why JMESPath:**

- ✅ Simple, readable syntax (more user-friendly than Jsonata)
- ✅ Excellent for data extraction and conditions
- ✅ Good Go library support (`github.com/jmespath/go-jmespath`)
- ✅ Handles nested data and arrays well
- ✅ Supports basic transformations
- ✅ Widely adopted (AWS CLI uses it)

**Alternative Consideration**: If you need advanced transformations later, you could add Jsonata as an opt-in feature for power users, but start with JMESPath for 90% of use cases.

**Example Addition for README**:

```
### Expression Language

Orchid uses JMESPath for evaluating conditions and extracting data. JMESPath provides a simple, readable syntax for querying JSON data.

**Condition Examples:**
- `response.status_code == 200`
- `response.body.next_page_token != null`
- `response.headers['x-rate-limit-remaining'] < 10`
- `length(response.body.items) > 0`
- `contains(response.body.errors[*].code, 'RATE_LIMIT')`

**Data Extraction Examples:**
- `response.body.users[0].id` - Get first user's ID
- `response.body.users[*].id` - Get all user IDs as array
- `parent.body.pagination.next` - Get next page from parent
- `context.last_user_id` - Get value from context
- `config.base_url` - Get value from config

**Available Variables in Expressions:**
- `response` - Current API response (status_code, headers, body)
- `prev` - Previous response in loop (headers, body)
- `parent` - Parent step response (headers, body)
- `context` - Plan context metadata
- `config` - User-defined configuration
- `auth` - Authentication result
- `item` - Current item when iterating (in sub-steps)

**Helper Functions:**
- `length(array)` - Get array length
- `contains(array, value)` - Check if array contains value
- `starts_with(string, prefix)` - String prefix check
- `ends_with(string, suffix)` - String suffix check
- `to_string(value)` - Convert to string
- `to_number(value)` - Convert to number
```

### 2. **Sub-Step Fanout/Iteration** ⭐ RECOMMENDED APPROACH

**Current Gap**: Sub-steps are mentioned but it's unclear how to iterate over arrays in responses (e.g., fetch settings for each user).

**Recommendation**: Use JMESPath expression for iteration path, pass items as `item` variable, with configurable concurrency.

**Key Design Decisions:**

1. **Use JMESPath** (consistent with expression language) - `iterate_over` should be a JMESPath expression
2. **Pass item as `item` variable** - Simple, clear, and works well with JMESPath
3. **Concurrent execution by default** - Maximize throughput, but allow configuration
4. **Per-item error handling** - Failures in one item don't stop others
5. **Support nested fanout** - Sub-steps can have their own sub-steps that iterate

**Example Addition for README**:

```
### Sub-Step Fanout/Iteration

Sub-steps can iterate over arrays in the parent response to execute the sub-step for each item. This enables fanout patterns where you fetch details for each item returned by the parent.

**Configuration:**
- `iterate_over`: JMESPath expression that evaluates to an array in the parent response
- `concurrency`: Maximum number of sub-steps to execute concurrently (default: 10, configurable per sub-step)
- `stop_on_error`: Whether to stop all sub-steps if one fails (default: false)

**How it works:**
1. Parent step executes and receives response
2. `iterate_over` JMESPath expression is evaluated against parent response
3. Each item in the resulting array is passed to the sub-step as the `item` variable
4. Sub-steps execute concurrently (up to concurrency limit)
5. Each sub-step can access:
   - `item` - Current item from the array
   - `parent` - Full parent response (headers, body)
   - `context` - Plan context
   - `config` - Configuration
   - `auth` - Auth result

**Example:**
Parent response: `{"users": [{"id": 1, "name": "Alice"}, {"id": 2, "name": "Bob"}]}`

Sub-step configuration:
- `iterate_over`: `parent.body.users`
- `url_schema`: `{base_url}/api/users/{user_id}/settings`
- `params`: `{"user_id": "item.id"}`

Result: Sub-step executes twice:
1. First execution: `item = {"id": 1, "name": "Alice"}`, calls `/api/users/1/settings`
2. Second execution: `item = {"id": 2, "name": "Bob"}`, calls `/api/users/2/settings`

**Nested Fanout:**
Sub-steps can have their own sub-steps with `iterate_over`, enabling multi-level fanout:
- Parent gets users → Sub-step gets each user's devices → Sub-sub-step gets each device's metrics

**Error Handling:**
- By default, if one item's sub-step fails, others continue
- Set `stop_on_error: true` to stop all remaining sub-steps on first failure
- Failed items are logged and can be retried independently
- Statistics track success/failure counts per sub-step

**Concurrency Control:**
- Each sub-step can define its own `concurrency` limit
- Limits apply per parent execution (not globally)
- Rate limiting still applies to individual API calls
- System respects both sub-step concurrency and rate limits
```

### 3. **Dynamic Parameter Extraction** ⭐ UPDATED: Use JMESPath

**Current Gap**: Params can reference context/config, but unclear if they can extract from responses.

**Recommendation**: Use JMESPath expressions for dynamic parameter extraction (consistent with conditions and iteration).

**Example Addition for README**:

```
### Dynamic Parameters and Headers

Parameters and headers can be static values or JMESPath expressions that extract data dynamically:

**Available Sources:**
- `response.body.user_id` - Extract from current response body
- `response.headers['x-next-page']` - Extract from current response headers
- `parent.body.users[0].id` - Extract from parent step response
- `parent.headers['x-total-count']` - Extract from parent headers
- `context.last_user_id` - Get value from saved context
- `config.base_url` - Get value from configuration
- `auth.token` - Get value from auth result
- `item.id` - Get value from current item (when iterating in sub-steps)

**Examples:**
- Static: `{"user_id": "123"}`
- Dynamic: `{"user_id": "response.body.users[0].id"}`
- From parent: `{"page": "parent.body.pagination.next"}`
- From context: `{"since": "context.last_sync_time"}`
- From item: `{"user_id": "item.id"}` (in sub-steps with iterate_over)

**How it works:**
1. If a param/header value is a string that looks like a JMESPath expression, it's evaluated
2. The result is converted to string if needed
3. If evaluation fails, the original value is used (allows mixing static and dynamic)
4. Expressions are evaluated against the current execution context (response, parent, context, etc.)

**URL Schema Parameters:**
Path parameters in URL schemas are automatically replaced with param values:
- URL: `{base_url}/api/users/{user_id}`
- Params: `{"user_id": "response.body.id"}`
- Result: `/api/users/123` (where 123 is the extracted ID)

**Query Parameters:**
Params that don't match URL schema placeholders become query parameters:
- URL: `{base_url}/api/users`
- Params: `{"page": "context.current_page", "limit": "50"}`
- Result: `/api/users?page=2&limit=50`
```

### 4. **Response Parsing for Conditions**

**Current Gap**: You emit raw data, but conditions need to evaluate response data. How is this handled?

**Suggestion**: Clarify that minimal parsing is needed for condition evaluation and sub-step iteration, even though raw data is emitted.

**Example Addition**:

```
While Orchid emits raw response data to Kafka, it performs minimal parsing (JSON, XML, etc.) to:
- Evaluate conditions (`while`, `abort_when`, `retry_when`, `break_when`)
- Extract data for sub-step iteration
- Resolve dynamic parameters and headers
- Access response metadata (status codes, headers)

The parsed structure is available in plan arguments but the original raw response is what gets emitted.
```

### 5. **Request Body Generation** ⭐ Use JMESPath

**Current Gap**: Request body mentioned but format/syntax undefined.

**Recommendation**: Use JMESPath expressions for dynamic values in request bodies (consistent with everything else).

**Example Addition for README**:

````
### Request Bodies

Request bodies can be static JSON/XML/text or use JMESPath expressions for dynamic values:

**Static JSON:**
```json
{"user_id": "123", "action": "sync"}
````

**Dynamic with JMESPath:**

```json
{
  "user_id": "response.body.id",
  "action": "config.action",
  "timestamp": "context.current_time"
}
```

The system evaluates JMESPath expressions and injects the results. Supports JSON, form-data, XML, and raw text formats. Expression evaluation works the same as for parameters and headers.

```

### 6. **Context Persistence** ⭐ Use JMESPath

**Current Gap**: Context can store custom fields, but when/how is unclear.

**Recommendation**: Use JMESPath expressions to set context values.

**Example Addition for README**:

```

### Context Management

Context fields can be set during plan execution using `set_context` with JMESPath expressions. Context is persisted:

- After each successful step execution
- Before entering wait periods
- After while loop iterations

**Setting Context:**

```yaml
set_context:
  last_page_token: "response.body.next_token"
  last_user_id: "response.body.users[-1].id"
  sync_timestamp: "context.current_time"
```

**Accessing Context:**
Use JMESPath expressions: `context.last_page_token`, `context.last_user_id`, etc.

Context is shared across all executions of the same plan/config combination.

```

---

## Medium Priority - Important for Scalability

### 7. **Queue Message Structure**

**Current Gap**: Queue-based execution mentioned but message format undefined.

**Suggestion**: Specify the structure of queue messages.

**Example Addition**:

```

Queue messages contain:

- Plan/step ID and version
- Config ID
- Execution context (for resuming)
- Parent step data (for sub-steps)
- Retry count
- Scheduled execution time
- Priority level

Messages are acknowledged after successful execution or moved to dead-letter queue after max retries.

```

### 8. **Plan Scheduling Strategy**

**Current Gap**: `wait` is mentioned but unclear if scheduling is time-based or event-based.

**Suggestion**: Clarify scheduling approach.

**Example Addition**:

```

Plans are scheduled based on their `wait` interval:

- After successful execution, plan waits for `wait` duration before next execution
- After failures/retries, wait period still applies before next attempt
- Plans can be triggered immediately via API (for testing/manual runs)
- Multiple configs for same plan run independently with their own schedules

```

### 9. **Auth Flow Definition** ⭐ Use JMESPath

**Current Gap**: Auth mechanisms mentioned but definition language unclear.

**Recommendation**: Auth flows are special plans that use JMESPath to extract tokens/credentials from responses.

**Example Addition for README**:

```

### Authentication Flows

Auth flows are defined as special plans that execute before main plan execution. They use JMESPath expressions to extract authentication tokens/credentials from the auth API response.

**Configuration:**

- `token_path`: JMESPath expression to extract token (e.g., `response.body.access_token`)
- `header_name`: Header name to use for token (e.g., `Authorization`)
- `header_format`: Format string with token (e.g., `Bearer {token}`)
- `refresh_path`: JMESPath expression for refresh token (optional)
- `expires_in_path`: JMESPath expression for expiration time (optional)
- `ttl`: Cache TTL in seconds
- `skew`: Seconds before expiry to refresh (default: 60)

**Example:**

```yaml
token_path: "response.body.access_token"
header_name: "Authorization"
header_format: "Bearer {token}"
expires_in_path: "response.body.expires_in"
ttl: 3600
skew: 300
```

Auth result is available as `auth.token`, `auth.headers`, etc. in plan arguments. Can be shared across multiple plans/configs.

```

### 10. **Rate Limit Bucket Strategy & Dynamic Rate Limiting** ⭐ UPDATED

**Current Gap**: Rate limits mentioned but bucket/key strategy unclear. Also missing dynamic rate limiting from response headers.

**✅ Clarified**: README now mentions dynamic rate limiting from response headers.

**Suggestion**: Expand with detailed specification.

**Example Addition**:

```

Rate limits are organized into buckets that can be:

- Per endpoint (e.g., `/api/users`)
- Per endpoint group (e.g., all `/api/users/*` endpoints)
- Per config (all endpoints for a specific config)
- Global (all endpoints for an integration)

Rate limit state is shared across instances via Redis. Buckets use sliding window or token bucket algorithms.

**Dynamic Rate Limiting:**
Many APIs provide rate limit information in response headers. Users can configure dynamic rate limiting using JMESPath expressions to extract:

- Remaining requests: `response.headers['X-RateLimit-Remaining']`
- Reset time: `response.headers['X-RateLimit-Reset']` (Unix timestamp)
- Retry after: `response.headers['Retry-After']` (seconds)
- Window size: `response.headers['X-RateLimit-Limit']`

Examples:

- Static: 100 requests per 60 seconds
- Dynamic: Use `response.headers['X-RateLimit-Remaining']` to track remaining requests
- Dynamic with reset: Use `response.headers['X-RateLimit-Reset']` to know when window resets
- Hybrid: Start with static limits, adjust based on API headers when available

The system will:

1. Check static rate limits first
2. If dynamic rate limit headers are present, use those values
3. Update rate limit state in Redis for all instances
4. Automatically throttle requests when limits are approached
5. Respect `Retry-After` headers for 429 responses

```

---

## Lower Priority - Nice to Have

### 11. **Plan Dependencies** ⭐ Not Needed - Use Sub-Steps

**Decision**: Plan dependencies are not needed as a separate feature. Instead, create a parent plan with sub-steps that execute sequentially.

**Rationale**:
- Simpler architecture - no need for dependency tracking
- More flexible - sub-steps can share data via `parent` variable
- Consistent model - everything is a plan/step hierarchy
- Better observability - single execution context

**Example Pattern**:
Instead of "Plan B depends on Plan A", create:
- Parent plan with two sub-steps
- Sub-step 1: Execute what Plan A would do
- Sub-step 2: Execute what Plan B would do (can access `parent.body` from step 1)

This pattern naturally handles dependencies while keeping the model simple.

### 12. **Conditional Sub-Step Execution** ⭐ Use JMESPath

**Suggestion**: Allow sub-steps to execute conditionally using JMESPath conditions.

**Example**: Add `execute_if` field with JMESPath condition:
- `execute_if: "parent.body.user_type == 'premium'"` - Only execute for premium users
- `execute_if: "item.status == 'active'"` - Only execute for active items when iterating

### 13. **Response Format Support**

**Suggestion**: Explicitly list supported response formats for parsing.

**Example**: "Supports JSON, XML, CSV, and plain text. Binary responses are base64 encoded in Kafka messages."

### 14. **Execution Timeouts**

**Suggestion**: Specify maximum execution time per plan/step.

**Example**: "Plans timeout after X minutes. Timeout is configurable per plan."

### 15. **Plan Versioning**

**Suggestion**: Specify how plan changes are versioned and rolled out.

**Example**: "Plans are versioned. New versions can be deployed while old versions complete. Rollback supported."

### 16. **Statistics API**

**Suggestion**: Specify how statistics are exposed.

**Example**: "Statistics available via REST API and Prometheus metrics. Includes per-plan and aggregate metrics."

### 17. **Dead Letter Queue**

**Suggestion**: Specify handling of permanently failed executions.

**Example**: "After max retries, failed executions are moved to dead-letter queue for manual inspection/replay."

### 18. **Plan Testing/Dry-Run**

**Suggestion**: Allow testing plans without full execution.

**Example**: "Plans can be tested in dry-run mode that validates syntax and shows execution plan without making API calls."

---

## Implementation Considerations

### Expression Language: JMESPath Everywhere ✅

**Decision**: Use **JMESPath** as the single expression language for all dynamic behavior:

- ✅ Conditions (`while`, `abort_when`, `retry_when`, `break_when`)
- ✅ Parameter extraction (params, headers)
- ✅ Request body generation
- ✅ Sub-step iteration (`iterate_over`)
- ✅ Context setting and access
- ✅ Auth token extraction
- ✅ Dynamic rate limiting
- ✅ Conditional sub-step execution

**Benefits:**
- Single language to learn
- Consistent syntax throughout
- Powerful enough for complex use cases
- Good Go library support
- Well-documented and widely used

### Queue System Decision ✅

**Decision**: **Redis Streams for job queue, Kafka for data emission**

- **Redis Streams**: Used for plan/step execution job queue
  - Supports consumer groups for horizontal scaling
  - Built-in acknowledgment and retry mechanisms
  - Good for job distribution patterns
  - Also used for locks, rate limit counters, and state coordination

- **Kafka**: Used for emitting raw API response data
  - High throughput for data streaming
  - Persistent message storage
  - Consumer groups for downstream processing

### State Storage Decision ✅

- **PostgreSQL (Citus)**: For persistent state (context, statistics, plan definitions)
- **Redis**: For ephemeral state (locks, rate limit counters, active executions, job queue)
- **Kafka**: For data emission only (raw API responses)

### Operational Limits Decision ✅

- **Response Formats**: Start with JSON, convert XML to JSON for parsing
- **Sub-Step Concurrency**: Default 50, configurable per sub-step
- **Context Size**: 64KB per field, 1MB total
- **Response Size**: 10MB max (Kafka message size limit)
- **Request Body Size**: 5MB max, configurable per plan/step
- **Execution Timeout**: 5 minutes default per step, configurable per plan/step
- **Nesting Depth**: 5 levels max, configurable via env var
- **Concurrency**: No hard limit, scales naturally with horizontal scaling
- **Retry Backoff**: Fibonacci sequence (1s, 1s, 2s, 3s, 5s, 8s, 13s...) with jitter, max backoff 60s (configurable)

---

## Questions to Resolve

1. **✅ Expression Language**: **JMESPath** - Decision made, use everywhere
2. **✅ Plan Dependencies**: **Not needed** - Use parent plan with sub-steps instead
3. **✅ Response Formats**: **Start with JSON**. XML can be converted to JSON for parsing. Other formats (CSV, binary) handled as needed.
4. **✅ Queue Choice**: **Redis Streams** for job queue, Kafka for data emission
5. **✅ Fanout Limits**: **Default 50** concurrent sub-steps per parent, configurable per sub-step
6. **✅ Context Size**: **64KB per field, 1MB total** context size
7. **✅ Response Size**: **10MB max** (limited by Kafka message size)
8. **✅ Plan Complexity**: **5 levels max nesting**, easily adjustable via environment variable
9. **✅ Concurrency**: **No hard limit** - scales naturally with horizontal scaling and Go's concurrency model
10. **✅ Request Body Size**: **5MB default, configurable** per plan/step
11. **✅ Timeout Defaults**: **5 minutes default per step, configurable** per plan/step
12. **✅ Retry Backoff**: **Fibonacci backoff with jitter and max backoff**. Fibonacci grows more slowly than exponential (1s, 1s, 2s, 3s, 5s, 8s, 13s...) which is less aggressive for APIs. Max backoff prevents excessive wait times (default 60s, configurable). Jitter adds randomness to prevent thundering herd problems.

**Why Fibonacci over Exponential:**
- Slower initial growth (1s, 1s, 2s vs 1s, 2s, 4s)
- Less aggressive on APIs
- Still provides good retry spacing
- Already used in codebase (startup.go uses Fibonacci)
```
