// Package cachecontrol provides Gin middleware for writing HTTP cache headers.
//
// The package only writes response headers. It does not cache response bodies.
// Attach it per route or route group:
//
//	router.GET("/products", cachecontrol.Public(5*time.Minute), listProducts)
//
// By default, headers are written only for GET and HEAD responses with 2xx or
// 3xx status codes, no Authorization request header, no Set-Cookie response
// header, and no existing Cache-Control response header.
//
// Use WithCondition for request-level opt-in or opt-out before the policy and
// default guards run.
//
// Cloudflare support is opt-in. Use WithCloudflare to mirror Cache-Control into
// Cloudflare-CDN-Cache-Control, or WithCloudflareDirectives to set a separate
// Cloudflare policy. Use WithCloudflareTags and WithCloudflareTagFunc to emit
// Cache-Tag for Cloudflare purge-by-tag workflows.
package cachecontrol
