# gthanks

Go API service for GitHub contribution aggregation.

## What It Does

This API accepts either:

- a GitHub user or org, such as `yorukot`
- a single repository, such as `yorukot/superfile`

It returns contribution data based on the GitHub REST contributors endpoint.

## Current Status

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
cp .env.example .env
go run ./cmd/server
```

Default server address:

```text
http://localhost:8080
```

## Configuration

Important variables in `.env`:

- `DB_PATH`: SQLite database path
- `GITHUB_TOKEN`: optional, but recommended to avoid strict public rate limits
- `PORT`: HTTP port
- `GITHUB_MAX_CONCURRENCY`: max concurrent GitHub contributor requests for user/org targets

Example:

```env
DB_PATH=./gthanks.sqlite3
GITHUB_TOKEN=ghp_xxx
PORT=8080
GITHUB_MAX_CONCURRENCY=1
```

## API

### Health Check

```bash
curl http://localhost:8080/healthz
```

Example response:

```json
{
  "status": "ok",
  "time": "2026-04-23T04:00:00Z"
}
```

### Get Contributions

Endpoint:

```text
GET /v1/contributions?target={target}
```

Supported targets:

- `yorukot`
- `yorukot/superfile`

Query parameters:

- `target`: required
- `refresh`: optional, `true` to bypass fresh cache and fetch from GitHub again
- `summary`: optional, default `true`; set `false` to skip cross-repo summary
- `include_forks`: optional, default `false`; in user/org mode, forked repositories are excluded unless enabled

Avatar grid image endpoint:

```text
GET /v1/contributions/image?target={target}&per_row={n}&width={px}&shape={circle|square}&padding={px}&space={px}
```

Image query parameters:

- `target`: required
- `per_row`: optional, default `12`
- `width`: optional, default `1920`
- `shape`: optional, `circle` or `square`, default `circle`
- `limit`: optional, default `1000`
- `padding`: optional, default `0`
- `space`: optional, default `12`
- `include_forks`: optional, default `false`
- `refresh`: optional, same behavior as JSON API

The generated PNG uses a transparent background by default.
`limit` currently accepts values from `1` to Go's `math.MaxInt`.

## Examples

User or org mode:

```bash
curl "http://localhost:8080/v1/contributions?target=yorukot"
```

Single repo mode:

```bash
curl "http://localhost:8080/v1/contributions?target=yorukot/superfile"
```

Generate avatar grid PNG:

```bash
curl "http://localhost:8080/v1/contributions/image?target=yorukot&per_row=12&width=1920&shape=circle&limit=1000&padding=0&space=12" --output contributors.png
```

The image endpoint is also cached in SQLite. The cache key includes `target`, `per_row`, `width`, `shape`, `limit`, `padding`, `space`, and `include_forks`.
Avatar downloads are also cached in SQLite so future image renders do not need to refetch the same avatar bytes every time.

Force refresh:

```bash
curl "http://localhost:8080/v1/contributions?target=yorukot/superfile&refresh=true"
```

Include forked repositories in user/org mode:

```bash
curl "http://localhost:8080/v1/contributions?target=yorukot&include_forks=true"
```

Disable summary:

```bash
curl "http://localhost:8080/v1/contributions?target=yorukot&summary=false"
```

## Response Shape

Top-level response fields:

- `metadata`
- `cache`
- `summary`
- `repos`
- `errors`

Example:

```json
{
  "metadata": {
    "input": "yorukot/superfile",
    "normalized_target": "yorukot/superfile",
    "mode": "single_repo",
    "status": "success",
    "generated_at": "2026-04-23T04:00:00Z"
  },
  "cache": {
    "status": "miss",
    "expires_at": "2026-04-23T05:00:00Z"
  },
  "summary": [
    {
      "identity_key": "github_user:123",
      "login": "yorukot",
      "avatar_url": "https://avatars.githubusercontent.com/u/123?v=4",
      "html_url": "https://github.com/yorukot",
      "total_contributions": 42,
      "repo_count": 1,
      "repos": [
        {
          "full_name": "yorukot/superfile",
          "html_url": "https://github.com/yorukot/superfile",
          "contributions": 42
        }
      ]
    }
  ],
  "repos": [
    {
      "full_name": "yorukot/superfile",
      "owner": "yorukot",
      "name": "superfile",
      "html_url": "https://github.com/yorukot/superfile",
      "private": false,
      "archived": false,
      "fork": false,
      "fetched_at": "2026-04-23T04:00:00Z",
      "contributors": [
        {
          "identity_key": "github_user:123",
          "login": "yorukot",
          "github_user_id": 123,
          "avatar_url": "https://avatars.githubusercontent.com/u/123?v=4",
          "html_url": "https://github.com/yorukot",
          "type": "User",
          "contributions": 42
        }
      ]
    }
  ],
  "errors": []
}
```

## Cache Behavior

- Fresh cache returns immediately
- Cache miss fetches from GitHub, stores SQLite snapshots, then returns JSON
- If refresh fails and stale cache exists, the API returns stale data with `cache.status = "stale"`

Response headers:

- `X-Cache-Status`
- `X-Image-Cache-Status`
- `X-GitHub-Requests`

For user/org targets, each `summary` item also includes a `repos` array so you can see which repositories that contributor appeared in.

## Common Status Codes

- `200`: success, including stale fallback and partial success
- `400`: invalid target or invalid query parameter
- `404`: target not found on GitHub and no stale cache available
- `429`: GitHub rate limit hit and no stale cache available
- `502`: upstream GitHub failure
- `500`: internal application error
