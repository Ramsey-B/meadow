# Use a recent Alpine-based Go image for linting and testing
FROM golang:1.24-alpine AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

COPY db/pg /app/db/pg

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o app ./cmd

FROM gcr.io/distroless/static-debian11

# Copy in the compiled binary
COPY --from=build /app/app /usr/local/bin/app

COPY --from=build /app/db/pg /db/pg

# Use an unprivileged user (distroless static runs as non-root by default)
ENTRYPOINT ["/usr/local/bin/app"]
