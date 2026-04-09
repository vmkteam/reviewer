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
COPY . /build
COPY --from=frontend /frontend/dist /build/frontend/dist
COPY --from=frontend /frontend/dist-vt /build/frontend/dist-vt
RUN cd /build && go install -mod=vendor ./cmd/reviewsrv
RUN cd /build && CGO_ENABLED=0 go build -mod=vendor -ldflags "-s -w" -o /go/bin/reviewctl ./cmd/reviewctl

# Final image
FROM alpine:latest

ENV TZ=Europe/Moscow
RUN apk --no-cache add ca-certificates tzdata && cp -r -f /usr/share/zoneinfo/$TZ /etc/localtime

COPY --from=builder /go/bin/reviewsrv .
COPY --from=builder /go/bin/reviewctl .

ENTRYPOINT ["/reviewsrv"]
EXPOSE 8075
