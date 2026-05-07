# syntax=docker/dockerfile:1.7

FROM golang:1.25-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker

FROM alpine:3.20 AS prep
RUN mkdir -p /var/lib/worker-go && chown -R 65532:65532 /var/lib/worker-go

FROM gcr.io/distroless/static-debian12:nonroot AS final
WORKDIR /app
COPY --from=builder /out/worker /app/worker
COPY --from=prep --chown=65532:65532 /var/lib/worker-go /var/lib/worker-go
USER nonroot:nonroot
EXPOSE 8081
VOLUME ["/var/lib/worker-go"]
ENTRYPOINT ["/app/worker"]
