##########################################
#
# Base for build and local development environment
#
FROM golang:1.24-alpine AS base

ARG GIT_COMMIT="<unknown>"
ARG GIT_BRANCH="<unknown>"
ARG BUILD_NUMBER="<unknown>"
ENV VERSION="${BUILD_NUMBER}+${GIT_BRANCH}.${GIT_COMMIT}"
ENV CGO_ENABLED=0

# Install some tools for easier debugging on live container
# while developing.
RUN apk add --no-cache --update \
    ca-certificates \
    bash \
    vim \
    tree \
    && update-ca-certificates

##########################################
#
# Development environment
#
# Targetted from Docker Compose. Notice that the
# source code will be mounted via Docker Compose.
#
FROM base AS dev
WORKDIR /app
EXPOSE 80
ENTRYPOINT go tool air -c go.air.toml

##########################################
#
# Build of production ready binary
#
FROM base AS build
WORKDIR /build
COPY . .
RUN go mod download
RUN go mod verify
RUN go test -cover -timeout 60s ./...
RUN go tool staticcheck ./...
RUN go tool govulncheck ./...
RUN go build \
    -ldflags="-s -w -X 'main.GitBranch=$GIT_BRANCH' -X 'main.GitCommit=$GIT_COMMIT' -X 'main.BuildNumber=$BUILD_NUMBER' -X 'main.Version=$VERSION'" \
    -o /tmp/main

# Note on Go build above:
# Other builds include -a -ldflags '-extldflags "-static"'
# Not sure if this is still valid and recommended. Need to verify before
# just adding it as a true cargo cultist :)

##########################################
#
# Production serve
#
FROM alpine:3.21.3 AS prod
# CA certificates for HTTPS calls to external services
# cURL for HTTP healthcheck
RUN apk add --no-cache --update \
    ca-certificates \
    curl \
    && update-ca-certificates \
    && rm -rf /tmp/* /var/cache/apk/*
WORKDIR /app
COPY --from=build /tmp/main .
EXPOSE 80
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD curl -f http://localhost/healthcheck || exit 1
ENTRYPOINT ["/app/main"]
