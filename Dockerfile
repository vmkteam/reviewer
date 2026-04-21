# Build frontend
FROM node:20-alpine AS frontend
WORKDIR /frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ .
RUN npm run build:all

# Build backend
FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git
# VERSION is passed via `docker build --build-arg VERSION=...` (GH Actions sends
# github.ref_name on release). Falls back to `git describe` when not provided.
ARG VERSION=
COPY . /build
COPY --from=frontend /frontend/dist /build/frontend/dist
COPY --from=frontend /frontend/dist-vt /build/frontend/dist-vt
RUN cd /build \
    && VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}" \
    && echo "building version=$VERSION" \
    && go build -mod=vendor -ldflags "-s -w -X main.version=$VERSION" -o /go/bin/reviewsrv ./cmd/reviewsrv \
    && CGO_ENABLED=0 go build -mod=vendor -ldflags "-s -w -X main.version=$VERSION" -o /go/bin/reviewctl ./cmd/reviewctl

# Final image
FROM alpine:latest

ENV TZ=Europe/Moscow
RUN apk --no-cache add ca-certificates tzdata && cp -r -f /usr/share/zoneinfo/$TZ /etc/localtime

COPY --from=builder /go/bin/reviewsrv .
COPY --from=builder /go/bin/reviewctl .
COPY docs/patches/*.sql /patches/

ENTRYPOINT ["/reviewsrv"]
EXPOSE 8075
