# gin-cache-control

[![CI](https://github.com/himkit/gin-cache-control/actions/workflows/ci.yml/badge.svg)](https://github.com/himkit/gin-cache-control/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/himkit/gin-cache-control.svg)](https://pkg.go.dev/github.com/himkit/gin-cache-control)

Small Gin middleware for writing `Cache-Control` and Cloudflare cache headers.

- Per-route by default
- Safe defaults for API responses
- Browser TTL and shared-cache TTL support
- Optional `Cloudflare-CDN-Cache-Control`
- Optional Cloudflare `Cache-Tag`
- No response body cache, Redis, storage, or purge client

## Install

Requires Go 1.25+.

```bash
go get github.com/himkit/gin-cache-control
```

## Quick Start

```go
import cachecontrol "github.com/himkit/gin-cache-control"

router.GET("/products",
	cachecontrol.Public(5*time.Minute),
	func(c *gin.Context) {
		c.JSON(200, gin.H{"items": []string{"book", "pen"}})
	},
)
```

Response:

```http
Cache-Control: public, max-age=300
```

## Browser And CDN TTL

Use `max-age` for browsers and `s-maxage` for shared caches.

```go
router.GET("/feed",
	cachecontrol.Public(1*time.Minute,
		cachecontrol.WithSharedMaxAge(10*time.Minute),
		cachecontrol.WithStaleWhileRevalidate(30*time.Second),
	),
	getFeed,
)
```

```http
Cache-Control: public, max-age=60, s-maxage=600, stale-while-revalidate=30
```

## Cloudflare

Mirror `Cache-Control` into `Cloudflare-CDN-Cache-Control`:

```go
cachecontrol.Public(5*time.Minute, cachecontrol.WithCloudflare())
```

Set a separate Cloudflare policy:

```go
cachecontrol.Public(1*time.Minute,
	cachecontrol.WithCloudflareDirectives(cachecontrol.Directives{
		Public:       true,
		MaxAge:       10 * time.Minute,
		StaleIfError: 5 * time.Minute,
	}),
)
```

Add purge tags:

```go
cachecontrol.Public(5*time.Minute,
	cachecontrol.WithCloudflare(),
	cachecontrol.WithCloudflareTags("products"),
	cachecontrol.WithCloudflareTagFunc(func(c *gin.Context) []string {
		return []string{"product:" + c.Param("id")}
	}),
)
```

Origin response:

```http
Cache-Control: public, max-age=300
Cloudflare-CDN-Cache-Control: public, max-age=300
Cache-Tag: products,product:123
```

## Dynamic Policy

Use `WithPolicy` when TTLs or tags depend on the request, status, or Gin context.

```go
cachecontrol.New(cachecontrol.WithPolicy(func(c *gin.Context) (cachecontrol.Config, bool) {
	if c.GetBool("skip_cache") {
		return cachecontrol.Config{}, false
	}

	return cachecontrol.Config{
		Directives: cachecontrol.Directives{
			Public:       true,
			MaxAge:       30 * time.Second,
			SharedMaxAge: 5 * time.Minute,
		},
		Cloudflare: cachecontrol.CloudflareConfig{
			Enabled: true,
			Tags:    []string{"feed"},
		},
	}, true
}))
```

For handlers that write a body, policy runs before the first write. Set context values before `c.JSON`,
`c.String`, or another write method.

## Safety Defaults

Headers are written only when:

- method is `GET` or `HEAD`
- status is `2xx` or `3xx`
- request has no `Authorization`
- response has no `Set-Cookie`
- handler has not already set `Cache-Control`

Override deliberately:

```go
cachecontrol.Public(30*time.Second,
	cachecontrol.WithMethods("GET", "POST"),
	cachecontrol.WithStatuses(200, 404),
	cachecontrol.WithAllowAuthorization(),
	cachecontrol.WithAllowSetCookie(),
	cachecontrol.WithOverride(),
)
```

## API

```go
cachecontrol.Public(ttl, opts...)
cachecontrol.Private(ttl, opts...)
cachecontrol.NoStore(opts...)
cachecontrol.New(opts...)
cachecontrol.FromConfig(config)
```

Common options:

```go
WithMaxAge(ttl)
WithSharedMaxAge(ttl)
WithStaleWhileRevalidate(ttl)
WithStaleIfError(ttl)
WithPublic()
WithPrivate()
WithNoStore()
WithMustRevalidate()
WithImmutable()
WithNoCache()
WithCloudflare()
WithCloudflareDirectives(directives)
WithCloudflareTags(tags...)
WithCloudflareTagFunc(fn)
WithCloudflareOverride()
WithPolicy(fn)
WithMethods(methods...)
WithStatuses(statuses...)
WithOverride()
WithAllowAuthorization()
WithAllowSetCookie()
```

## Development

```bash
make test
make lint
make coverage
```

## License

MIT
