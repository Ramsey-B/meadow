This project is a microservice within a larger data engineering platform. This project is responsible for api polling.

Orchid is responsible for allowing users to define plans for extracting data from a 3rd party api. It's intended to be robust and reliable. It will expose endpoints for tracking statuses and managing and updating the plans and secrets.

Data will be polled out of the external api and emitted onto a kafka queue for the rest of the system. Orchid emits raw data as received from the APIs - data transformations are handled by the sister project "Lotus".

## Configuration

Most APIs will require that the specific fields and secrets be provided. Users will need to define a config schema. When enabling, the users will also need to provide values for these configs. In theory the users can provide multiple configs for the same integration.

## Auth

Many apis require an auth flow for things like getting tokens. Users will need the ability to define auth mechanisms for the API to generate and potentially refresh the auth. We will also need the ability to optionally define a cache and ttl management for the auth and likely a skew mechanism.

## api plans

This is the meat of Orchid. Users will define plans on HOW to get the data out of the 3rd party api. Each integration will have a set of Plans, these plans will run independently, meaning a plan that gets Users won't be impacted by the plan that gets Devices. Each plan can have a set of sub-steps like get Users may have sub steps where each Users settings are fetched. Sub-steps may also have their own sub-steps

Users will define a `wait` which will determine how long we will wait between executions of the plan.

Users may define a `repeat_count` which will define how many times the plan will be executed. If not set it will be run infinitely.

Users may enable/disable the plans at anytime.

Users will define the url schema like `{base_url}/api/users/{user_id}`

Users may define the params which will replace the path parameters in the url schema. If the param name does not match a value in the url schema then it is treated as a query parameter. Param values can be static strings or JMESPath expressions that extract data from `response`, `parent`, `context`, `config`, `auth`, or `item` (when iterating in sub-steps).

Users may define headers. Header values can be static strings or JMESPath expressions, similar to params.

Users may define a request body.

Users will define the request method (GET, POST, PUT, etc)

Users may optionally define sub-steps which will have the same functions as the plan but without things like `repeat_count`, or `wait`. Sub-steps can iterate over arrays in the parent response using an `iterate_over` field that specifies a JMESPath expression. Each item in the array is passed to the sub-step as `item`, allowing the sub-step to execute for each item concurrently. Users can configure concurrency limits per sub-step to control fanout behavior.

Users may define a `while`. This is a condition that will cause the plan/step to immediately rerun if true. This can be useful for things like pagination. If not provided the plan is called once then enters its wait.

Users may define a `abort_when`. A condition which causes the plan to stop execution and not rerun. This is intended to handle conditions like 403 errors that are unrecoverable.

Users may define a `retry_when`. A condition which causes the plan to immediately re attempt the request. This is intended to help with transient errors. Users can define retry events (conditions that trigger retries) and maximum retry counts per plan/step.

Users may define a `break_when`. A condition which causes the plan to exit the current while when true.

We should maximize concurrency especially with things like fanout but we may want to run the plans across multiple Orchid instances. To enable horizontal scaling, plan/step execution will be enqueued using Redis Streams for job distribution. Multiple Orchid instances consume work from the queue, ensuring plans are executed reliably even if individual instances fail. Redis is also used for sharing locks, coordinating state, and rate limit counters across instances. Data is emitted to Kafka for the rest of the system.

We should record useful statistics for the plans, things like last execution, last failure, last success, number of api calls, etc.

### plan arguments

plans and their sub-steps will be provided with the following arguments they can access:

- `config`: This is the config the user defined. This is important for accessing values like secret keys and base urls
- `auth`: This is the result of an auth pre-step that can be executed prior to the plan/step execution
- `context`: This will include metadata about the plan. Such as the last successful execution time, current time, custom context fields saved by previous executions. Can also include an execution count while in a while loop.
- `response.status_code` & `response.headers` & `response.body`: After the api call is made this is populated with the response data.
- `prev.headers` & `prev.body`: If the plan/step is executed in a loop, this is the previous api response headers/body. So if this plan is paginated page 1 this value is null but page 2 this will be the headers/body from page 1s response.
- `parent.headers` & `parent.body`: This is set of sub-steps to access the parent plan/step information.
- `item`: When a sub-step uses `iterate_over` to fanout over an array, each item from the array is available as `item`. For example, if iterating over `parent.body.users`, each sub-step execution receives one user object as `item`.

### Rate limiting

Many apis define rate limits, throttle, or apply concurrency limits. We need to gracefully handle these things for the users. These values should be configurable down to the api endpoint, groups of endpoints, or even for the entire set of api endpoints.

- We should allow for configuring requests per time window
- We should allow for prioritizing api calls so if an endpoint shares a rate limit with other endpoints, the users can define the endpoints that should get priority
- We should allow for concurrency limitations. Some endpoints only allow a certain number of in flight calls at a time.
- We should support dynamic rate limiting based on API response headers. Users can define JMESPath expressions to extract rate limit information from response headers (e.g., `X-RateLimit-Remaining`, `X-RateLimit-Reset`, `Retry-After`). The system will automatically adjust rate limits based on these values, allowing it to adapt to APIs that provide real-time rate limit information.

**Dynamic Rate Limiting Examples:**

- Extract remaining requests: `response.headers['X-RateLimit-Remaining']`
- Extract reset timestamp: `response.headers['X-RateLimit-Reset']`
- Extract retry delay: `response.headers['Retry-After']`
- Extract limit per window: `response.headers['X-RateLimit-Limit']`

The system will use these values to dynamically adjust rate limiting, falling back to static configuration when headers are not present. Rate limit state is shared across all Orchid instances via Redis to ensure consistent behavior in a distributed environment.

### Operational Limits and Constraints

- **Response Formats**: JSON is the primary supported format. XML responses are converted to JSON for parsing. Other formats (CSV, binary) are handled as needed, with binary data base64 encoded for Kafka emission.
- **Sub-Step Concurrency**: Default of 50 concurrent sub-steps per parent execution, configurable per sub-step.
- **Context Size**: Maximum 64KB per context field, 1MB total context size per plan/config.
- **Response Size**: Maximum 10MB response body size (limited by Kafka message size constraints).
- **Plan Nesting Depth**: Maximum 5 levels of sub-step nesting, configurable via environment variable.
- **Horizontal Scaling**: Plan/step execution is enqueued using Redis Streams for job distribution. Multiple Orchid instances consume from the queue, with no hard limit on concurrent plans per instance - scaling is handled naturally through horizontal scaling and Go's concurrency model.
- **Request Body Size**: Maximum 5MB request body size, configurable per plan/step.
- **Execution Timeout**: Default 5 minutes per step, configurable per plan/step.
- **Retry Backoff**: Fibonacci backoff sequence (1s, 1s, 2s, 3s, 5s, 8s, 13s...) with jitter and maximum backoff limit. Fibonacci grows more slowly than exponential backoff, making it less aggressive on APIs while still providing good retry spacing. Maximum backoff prevents excessive wait times (configurable, default 60s).
