# syntax=docker/dockerfile:1.7

FROM golang:1.25-alpine AS builder

WORKDIR /src

RUN apk add --no-cache build-base sqlite-dev

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
	--mount=type=cache,target=/root/.cache/go-build \
	CGO_ENABLED=1 GOOS=linux GOMAXPROCS=1 \
	go build -p=1 -tags libsqlite3 -v -trimpath -o /out/gthanks ./cmd/server

FROM alpine:3.22

WORKDIR /app

RUN apk add --no-cache ca-certificates sqlite-libs \
	&& addgroup -S app \
	&& adduser -S -G app app \
	&& mkdir -p /data \
	&& chown -R app:app /data /app

COPY --from=builder /out/gthanks /usr/local/bin/gthanks
COPY --from=builder /src/migrations ./migrations

ENV APP_ENV=production
ENV PORT=8080
ENV DB_PATH=/data/gthanks.sqlite3

EXPOSE 8080

VOLUME ["/data"]

USER app

CMD ["gthanks"]
