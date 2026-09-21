FROM --platform=$BUILDPLATFORM golang:1.26 AS build

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.sum ./

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY plugins ./plugins
COPY gate.go ./

# Automatically provided by the buildkit
ARG TARGETOS TARGETARCH
ARG VERSION=unknown

# Build
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -ldflags="-s -w -X go.minekube.com/gate/pkg/version.Version=${VERSION}" -o gate .

# Managed Bedrock needs zlib on arm64.
FROM debian:bookworm-slim AS runtime-deps
RUN mkdir -p /runtime-libs \
    && cp -L /usr/lib/*-linux-gnu/libz.so.1 /runtime-libs/libz.so.1

# Move binary into final image
# Gate's purego dependency needs the glibc loader even with CGO_ENABLED=0.
FROM gcr.io/distroless/base-debian12 AS app
WORKDIR /app
COPY --from=build /workspace/gate /app/gate
COPY --from=runtime-deps /runtime-libs/libz.so.1 /usr/lib/libz.so.1
COPY config.yml /app/
COPY simplecloud-connection /app/simplecloud-connection
ENV XDG_CACHE_HOME=/var/cache/gate
VOLUME ["/var/cache/gate"]
ENTRYPOINT ["/app/gate"]
