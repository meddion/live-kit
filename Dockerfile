FROM golang:1.27-alpine3.24 AS builder

WORKDIR /go/src

# Download dependencies first so this layer is cached until go.mod/go.sum change.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -ldflags '-extldflags "-static"' -o /build/main /go/src

FROM alpine:3.24
WORKDIR /app
COPY --from=builder /build/main /app/
COPY frontend/ /app/frontend/
ENTRYPOINT ["/app/main"]
