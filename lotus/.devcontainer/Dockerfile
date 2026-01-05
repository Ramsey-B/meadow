FROM mcr.microsoft.com/devcontainers/go:1.24

# Install tools
RUN su vscode -c "\
    go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.1.6 && \
    go install golang.org/x/tools/cmd/goimports@latest && \
    go install github.com/go-delve/delve/cmd/dlv@latest && \
    go install github.com/air-verse/air@latest"

# Set working dir
WORKDIR /workspace

# Ensure permissions
RUN chown -R vscode:vscode /workspace