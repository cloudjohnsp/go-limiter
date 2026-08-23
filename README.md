# Go Redis Rate Limiter

A small HTTP API that applies a Redis-backed token-bucket rate limit to each API key. The limiter uses a Lua script, so refilling and consuming a token happen atomically even when multiple API instances share the same Redis server.

## Requirements

- Go 1.26 or later
- Redis 6 or later, reachable from the API process

## Run locally

Start Redis. For example, if Docker is installed:

```sh
docker run --rm --name rate-limiter-redis -p 6379:6379 redis:7-alpine
```

In another terminal, start the API:

```powershell
cd app
$env:REDIS_ADDR = "localhost:6379"
go run .\cmd\api\main.go
```

The API listens on `http://localhost:8080`.

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `REDIS_ADDR` | `localhost:6379` | Redis host and port. |

The current rate-limit policy is configured in `app/cmd/api/main.go`:

- Capacity: 30 tokens (maximum initial burst)
- Refill: 1 token per second
- Identity: `X-API-KEY` request header

## API

### `GET /healthz`

Returns `200 OK` with an `OK` response body. This endpoint does not require an API key.

```sh
curl -i http://localhost:8080/healthz
```

### `GET /api/`

Requires an `X-API-KEY` header.

```sh
curl -i -H "X-API-KEY: demo-key" http://localhost:8080/api/
```

| Status | Meaning |
| --- | --- |
| `200 OK` | A token was available and the request was accepted. |
| `400 Bad Request` | The `X-API-KEY` header was missing. |
| `429 Too Many Requests` | The bucket is empty. The response includes `Retry-After: 1`. |
| `503 Service Unavailable` | Redis is unavailable or did not answer within the request's 300 ms limiter timeout. |

## Verify rate limiting

This sends 31 requests with the same API key. With a fresh bucket, 30 should return `200` and one should return `429`.

```powershell
1..31 | ForEach-Object {
  (curl.exe -s -o NUL -w "%{http_code}`n" -H "X-API-KEY: demo-key" http://localhost:8080/api/)
}
```

Wait approximately one second, then send another request; it should be accepted as one token has refilled.

## Test and build

```powershell
cd app
go test ./...
go build ./cmd/api
```

The repository also includes a shell load-check script at `scripts/test-rate-limit.sh`. It targets an nginx/compose setup, which is not included in this repository; use it only when that surrounding setup is available.

## Container image

Build the API image from the `app` directory:

```sh
docker build -t go-redis-rate-limiter .
docker run --rm -p 8080:8080 -e REDIS_ADDR=host.docker.internal:6379 go-redis-rate-limiter
```

On Linux, replace `host.docker.internal` with a Redis hostname reachable from the container (for example, the Redis service name on a Docker network).

## Implementation notes

Each API key maps to a Redis hash named `rl:apikey:<key>`. It stores the remaining token count and the last update time. Keys expire once a full bucket could have been replenished, preventing inactive keys from accumulating indefinitely.

Avoid using sensitive API-key values directly in production logs or Redis key names; prefer a stable hash or opaque client identifier.

