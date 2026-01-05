# 🌱 Stem

Shared Go packages for the flower-themed microservices ecosystem.

## Packages

| Package | Description |
|---------|-------------|
| `pkg/database` | Database utilities, JSONB, migrations, SQL builders |
| `pkg/tracing` | OpenTelemetry tracing helpers and exporters |
| `pkg/middleware` | Echo middleware (context, error handling, logging) |
| `pkg/context` | Request context utilities (tenant, user, request ID) |
| `pkg/startup` | Dependency-based service startup with backoff |

## Usage

### In other projects (Orchid, Lotus)

```go
import (
    "github.com/Ramsey-B/stem/pkg/database"
    "github.com/Ramsey-B/stem/pkg/tracing"
    "github.com/Ramsey-B/stem/pkg/middleware"
)
```

### Local Development with Go Workspaces

Create a `go.work` file in the parent directory:

```go
go 1.23.0

use (
    ./stem
    ./orchid
    ./lotus
)
```

This allows you to develop all projects together without publishing stem.

## Architecture

```
┌─────────────────────────────────────────┐
│              Applications               │
├──────────────────┬──────────────────────┤
│     Orchid 🌸    │      Lotus 🪷        │
│  (API Polling)   │   (Data Mapping)     │
└────────┬─────────┴──────────┬───────────┘
         │                    │
         ▼                    ▼
┌─────────────────────────────────────────┐
│              Stem 🌱                    │
├─────────────────────────────────────────┤
│  database │ tracing │ middleware │ ...  │
└─────────────────────────────────────────┘
```

## Dependencies

- `github.com/Gobusters/ectologger` - Structured logging
- `github.com/Gobusters/ectoerror` - Error handling
- `github.com/jmoiron/sqlx` - SQL extensions
- `github.com/huandu/go-sqlbuilder` - SQL query builder
- `go.opentelemetry.io/otel` - OpenTelemetry
- `github.com/labstack/echo/v4` - HTTP framework

