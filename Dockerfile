FROM golang:1.25-alpine AS builder

WORKDIR /src

RUN apk add --no-cache build-base

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /out/gthanks ./cmd/server

FROM alpine:3.22

WORKDIR /app

RUN apk add --no-cache ca-certificates sqlite-libs \
	&& addgroup -S app \
	&& adduser -S -G app app \
	&& mkdir -p /data \
	&& chown -R app:app /data /app

COPY --from=builder /out/gthanks /usr/local/bin/gthanks

ENV APP_ENV=production
ENV PORT=8080
ENV DB_PATH=/data/gthanks.sqlite3

EXPOSE 8080

VOLUME ["/data"]

USER app

CMD ["gthanks"]
