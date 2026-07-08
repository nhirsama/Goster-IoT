# syntax=docker/dockerfile:1.7
#
# Build from repo root:
#   docker build -f protocol-ingress/Dockerfile .

FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS build

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

# Copy only module metadata first so dependency downloads stay cached.
COPY --link proto/go.mod proto/go.sum ./proto/
COPY --link protocol-ingress/go.mod protocol-ingress/go.sum ./protocol-ingress/

WORKDIR /src/protocol-ingress
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

WORKDIR /src
COPY proto ./proto
COPY protocol-ingress ./protocol-ingress

WORKDIR /src/protocol-ingress
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/protocol-ingress ./cmd/protocol-ingress

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=build /out/protocol-ingress /app/protocol-ingress

ENV PROTOCOL_INGRESS_HTTP_ADDR=:8090 \
    PROTOCOL_INGRESS_CUSTOM_TCP_ADDR=:8081 \
    PROTOCOL_INGRESS_MQTT_LISTEN_ADDR=:1883

EXPOSE 8090 8081 1883

CMD ["/app/protocol-ingress"]
