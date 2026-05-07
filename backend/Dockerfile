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
    --mount=type=cache,target=/root/.cache/go-build,id=backend-build-${TARGETARCH} \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server && \
    go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate && \
    go build -trimpath -ldflags="-s -w" -o /out/seed ./cmd/seed

FROM gcr.io/distroless/static-debian12:nonroot AS final
WORKDIR /app
COPY --from=builder /out/server /app/server
COPY --from=builder /out/migrate /app/migrate
COPY --from=builder /out/seed /app/seed
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/server"]
