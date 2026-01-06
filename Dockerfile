FROM golang:1.24 AS builder

ARG MAIN_PATH=./cmd/api/main.go

WORKDIR /app

COPY go.mod go.sum ./
RUN go env -w GOPROXY=https://goproxy.cn,direct
RUN go mod download

COPY . .

# Build the final static binary for Linux
RUN CGO_ENABLED=0 GOOS=linux go build -o main ${MAIN_PATH}

# --- Final Stage: Use a minimal base image ---
FROM ubuntu:latest

WORKDIR /app

# Install tmux and bash
RUN apt-get update && apt-get install -y tmux bash wget && rm -rf /var/lib/apt/lists/*

# Install Go 1.24.5 manually
RUN wget https://go.dev/dl/go1.24.5.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go1.24.5.linux-amd64.tar.gz && \
    rm go1.24.5.linux-amd64.tar.gz

ENV PATH="/usr/local/go/bin:$PATH"

# Copy the compiled Go binary from the 'builder' stage
COPY --from=builder /app/main .
# Copy config files if necessary. Assuming config is loaded from env or default paths.
# If there are static files, copy them too.
# COPY --from=builder /app/config ./config

EXPOSE 8080

# Default entrypoint (can be overridden by k8s manifest)
CMD ["./main"]