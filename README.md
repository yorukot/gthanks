<p align="center">
  <img src="assets/logo.png" alt="GThanks logo" width="180">
</p>

# GThanks

GThanks is a Go API service that aggregates GitHub repository contributor data and can return either JSON or a transparent PNG avatar grid.

The hosted API is available at:

```text
https://gthanks.yorukot.me
```

Example image:

<p align="center">
  <img src="https://gthanks.yorukot.me/image?target=yorukot" alt="GThanks contributors example for yorukot" width="720">
</p>

## What It Does

GThanks accepts a GitHub target in one of two forms:

- `owner`, for a GitHub user or organization, such as `yorukot`
- `owner/repo`, for a single repository, such as `yorukot/gthanks`

For user and organization targets, GThanks lists repositories, fetches contributor data for each repository, and aggregates contributor totals. For single repository targets, it fetches contributor data for that repository only.

## Query Flags

### Shared Flags

These flags work on both `/json` and `/image`.

| Flag | Type | Default | Required | Description |
| --- | --- | --- | --- | --- |
| `target` | string | none | yes | GitHub user, organization, or repository. Use `owner` or `owner/repo`. |
| `refresh` | boolean | `false` | no | When `true`, bypasses fresh cache and fetches live GitHub data. |
| `include_forks` | boolean | `false` | no | Includes forked repositories for user or organization targets. |
| `include_bots` | boolean | `true` | no | Includes contributors whose GitHub type is `Bot`. |

### JSON Flags

These flags only apply to `/json`.

| Flag | Type | Default | Required | Description |
| --- | --- | --- | --- | --- |
| `summary` | boolean | `true` | no | Includes the aggregated contributor summary when `true`. |

### Image Flags

These flags only apply to `/image`.

| Flag | Type | Default | Range | Description |
| --- | --- | --- | --- | --- |
| `per_row` | integer | `12` | `1` to `20` | Number of avatars per row. |
| `width` | integer | `1920` | `100` to `4000` | Output image width in pixels. |
| `shape` | string | `circle` | `circle`, `square` | Avatar mask shape. |
| `limit` | integer | `144` | `1` to max integer | Maximum number of contributor avatars to render. |
| `padding` | integer | `0` | `0` to `500` | Outer image padding in pixels. |
| `space` | integer | `12` | `0` to `500` | Spacing between avatars in pixels. |

The image endpoint validates that `width` is large enough for the selected `per_row`, `padding`, and `space` values.

## Cache Behavior

GThanks stores query responses, generated images, repository snapshots, and downloaded avatars in SQLite.

| Cache | Default TTL | Notes |
| --- | --- | --- |
| Single repository query/image | `1h` | Configured with `CACHE_TTL_SINGLE_REPO`. |
| User or organization query/image | `3h` | Configured with `CACHE_TTL_USER_ORG`. |
| Avatar downloads | `24h` | Used internally by the image renderer. |

When `refresh=true`, GThanks skips fresh query and image cache entries and requests live GitHub data. If GitHub refresh fails and stale cached data exists, the JSON endpoint can return stale data with an error entry, and the image endpoint can return a stale image.
