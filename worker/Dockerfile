# syntax=docker/dockerfile:1.7

FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build,id=worker-build-${TARGETARCH} \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker

FROM alpine:3.20 AS prep
RUN mkdir -p /var/lib/worker && chown -R 65532:65532 /var/lib/worker

FROM gcr.io/distroless/static-debian12:nonroot AS final
WORKDIR /app
COPY --from=builder /out/worker /app/worker
COPY --from=prep --chown=65532:65532 /var/lib/worker /var/lib/worker
USER nonroot:nonroot
EXPOSE 8081
VOLUME ["/var/lib/worker"]
ENTRYPOINT ["/app/worker"]
