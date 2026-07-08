# syntax=docker/dockerfile:1.7
#
# Build from repo root:
#   docker build -f go/Dockerfile .

FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS build

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

# Copy only module metadata first so dependency downloads stay cached.
COPY --link proto/go.mod proto/go.sum ./proto/
COPY --link go/go.mod go/go.sum ./go/

WORKDIR /src/go
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

WORKDIR /src
COPY proto ./proto
COPY go ./go

WORKDIR /src/go
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/goster-iot .

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=build /out/goster-iot /app/goster-iot
COPY --from=build /src/go/db /app/db

RUN mkdir -p /data

ENV DB_PATH=/data/data.db \
    DB_MIGRATIONS_DIR=/app/db/migrations \
    WEB_HTTP_ADDR=:8080

VOLUME ["/data"]
EXPOSE 8080

CMD ["/app/goster-iot", "serve"]
