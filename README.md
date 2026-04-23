# gthanks

Go API service for GitHub contribution aggregation.

## Current status

MVP foundation is in place:

- Go module
- `chi` router
- `slog` logger
- SQLite storage
- SQL migrations
- GitHub REST client
- `GET /v1/contributions`
- `/healthz` endpoint
- graceful shutdown

## Run

```bash
export DB_PATH=./gthanks.sqlite3
go run ./cmd/server
```
