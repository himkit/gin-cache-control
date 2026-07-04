package cachecontrol

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	headerCacheControl           = "Cache-Control"
	headerCloudflareCacheControl = "Cloudflare-CDN-Cache-Control"
	headerCacheTag               = "Cache-Tag"

	maxCacheTagHeaderLen = 16 * 1024
)

// Option changes middleware configuration.
type Option func(*Config)

// PolicyFunc computes a cache policy before the first response write.
// If the handler does not write a response body, it runs after handlers return.
// Returning false skips all cache headers for the request.
type PolicyFunc func(c *gin.Context) (Config, bool)

// Config is the full middleware configuration.
type Config struct {
	// Directives controls the Cache-Control header value.
	Directives Directives

	// Policy computes a request-specific config before the first response write.
	Policy PolicyFunc

	// Cloudflare controls Cloudflare-CDN-Cache-Control and Cache-Tag headers.
	Cloudflare CloudflareConfig

	// Override allows replacing an existing Cache-Control header.
	Override bool

	// Methods replaces the default GET/HEAD method allowlist.
	Methods []string

	// Statuses replaces the default 2xx/3xx status allowlist.
	Statuses []int

	// AllowAuthorization allows cache headers when the request has Authorization.
	AllowAuthorization bool

	// AllowSetCookie allows cache headers when the response sets cookies.
	AllowSetCookie bool
}

// Directives describes Cache-Control compatible directives.
type Directives struct {
	// MaxAge emits max-age.
	MaxAge time.Duration

	// SharedMaxAge emits s-maxage for shared caches.
	SharedMaxAge time.Duration

	// StaleWhileRevalidate emits stale-while-revalidate.
	StaleWhileRevalidate time.Duration

	// StaleIfError emits stale-if-error.
	StaleIfError time.Duration

	// Public emits public.
	Public bool

	// Private emits private. If Public is also true, Private wins.
	Private bool

	// NoCache emits no-cache.
	NoCache bool

	// NoStore emits no-store and suppresses all other directives.
	NoStore bool

	// MustRevalidate emits must-revalidate.
	MustRevalidate bool

	// Immutable emits immutable.
	Immutable bool
}

// CloudflareConfig controls Cloudflare-specific cache headers.
type CloudflareConfig struct {
	// Enabled enables Cloudflare-CDN-Cache-Control and Cache-Tag handling.
	Enabled bool

	// Directives controls Cloudflare-CDN-Cache-Control. If nil, Cache-Control is copied.
	Directives *Directives

	// Tags are static Cache-Tag values.
	Tags []string

	// TagFunc returns dynamic Cache-Tag values.
	TagFunc func(c *gin.Context) []string

	// Override allows replacing existing Cloudflare-CDN-Cache-Control and Cache-Tag headers.
	Override bool
}

// Public returns middleware that writes a public Cache-Control policy.
func Public(ttl time.Duration, opts ...Option) gin.HandlerFunc {
	cfg := Config{Directives: Directives{Public: true, MaxAge: ttl}}
	applyOptions(&cfg, opts)
	return FromConfig(cfg)
}

// Private returns middleware that writes a private Cache-Control policy.
func Private(ttl time.Duration, opts ...Option) gin.HandlerFunc {
	cfg := Config{Directives: Directives{Private: true, MaxAge: ttl}}
	applyOptions(&cfg, opts)
	return FromConfig(cfg)
}

// NoStore returns middleware that writes Cache-Control: no-store.
func NoStore(opts ...Option) gin.HandlerFunc {
	cfg := Config{Directives: Directives{NoStore: true}}
	applyOptions(&cfg, opts)
	return FromConfig(cfg)
}

// New builds middleware from options without applying a preset.
func New(opts ...Option) gin.HandlerFunc {
	var cfg Config
	applyOptions(&cfg, opts)
	return FromConfig(cfg)
}

// FromConfig builds middleware from a complete config.
func FromConfig(cfg Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		writer := &cacheWriter{ResponseWriter: c.Writer}
		applied := false
		writer.beforeWrite = func() {
			if applied {
				return
			}
			applied = true

			active := cfg
			if active.Policy != nil {
				next, ok := active.Policy(c)
				if !ok {
					return
				}
				active = next
			}

			writeHeaders(c, active)
		}

		c.Writer = writer
		c.Next()
		writer.apply()
	}
}

// WithSharedMaxAge adds s-maxage for shared caches.
func WithSharedMaxAge(ttl time.Duration) Option {
	return func(cfg *Config) {
		cfg.Directives.SharedMaxAge = ttl
	}
}

// WithMaxAge adds max-age.
func WithMaxAge(ttl time.Duration) Option {
	return func(cfg *Config) {
		cfg.Directives.MaxAge = ttl
	}
}

// WithPublic adds public.
func WithPublic() Option {
	return func(cfg *Config) {
		cfg.Directives.Public = true
	}
}

// WithPrivate adds private.
func WithPrivate() Option {
	return func(cfg *Config) {
		cfg.Directives.Private = true
	}
}

// WithNoStore adds no-store.
func WithNoStore() Option {
	return func(cfg *Config) {
		cfg.Directives.NoStore = true
	}
}

// WithStaleWhileRevalidate allows shared caches to serve stale content while revalidating.
func WithStaleWhileRevalidate(ttl time.Duration) Option {
	return func(cfg *Config) {
		cfg.Directives.StaleWhileRevalidate = ttl
	}
}

// WithStaleIfError allows caches to serve stale content when the origin errors.
func WithStaleIfError(ttl time.Duration) Option {
	return func(cfg *Config) {
		cfg.Directives.StaleIfError = ttl
	}
}

// WithMustRevalidate adds must-revalidate.
func WithMustRevalidate() Option {
	return func(cfg *Config) {
		cfg.Directives.MustRevalidate = true
	}
}

// WithImmutable adds immutable.
func WithImmutable() Option {
	return func(cfg *Config) {
		cfg.Directives.Immutable = true
	}
}

// WithNoCache adds no-cache.
func WithNoCache() Option {
	return func(cfg *Config) {
		cfg.Directives.NoCache = true
	}
}

// WithCloudflare enables Cloudflare-CDN-Cache-Control cloning.
func WithCloudflare() Option {
	return func(cfg *Config) {
		cfg.Cloudflare.Enabled = true
	}
}

// WithCloudflareDirectives builds Cloudflare-CDN-Cache-Control from separate directives.
func WithCloudflareDirectives(d Directives) Option {
	return func(cfg *Config) {
		cfg.Cloudflare.Enabled = true
		cfg.Cloudflare.Directives = &d
	}
}

// WithCloudflareTags adds static Cache-Tag values for Cloudflare purge-by-tag.
func WithCloudflareTags(tags ...string) Option {
	return func(cfg *Config) {
		cfg.Cloudflare.Enabled = true
		cfg.Cloudflare.Tags = append(cfg.Cloudflare.Tags, tags...)
	}
}

// WithCloudflareTagFunc adds dynamic Cache-Tag values from the final Gin context.
func WithCloudflareTagFunc(fn func(*gin.Context) []string) Option {
	return func(cfg *Config) {
		cfg.Cloudflare.Enabled = true
		cfg.Cloudflare.TagFunc = fn
	}
}

// WithCloudflareOverride allows replacing existing Cloudflare cache headers.
func WithCloudflareOverride() Option {
	return func(cfg *Config) {
		cfg.Cloudflare.Enabled = true
		cfg.Cloudflare.Override = true
	}
}

// WithPolicy computes the config before the first response write.
func WithPolicy(fn PolicyFunc) Option {
	return func(cfg *Config) {
		cfg.Policy = fn
	}
}

// WithMethods replaces the default GET/HEAD method allowlist.
func WithMethods(methods ...string) Option {
	return func(cfg *Config) {
		cfg.Methods = append([]string(nil), methods...)
	}
}

// WithStatuses replaces the default 2xx/3xx status allowlist.
func WithStatuses(statuses ...int) Option {
	return func(cfg *Config) {
		cfg.Statuses = append([]int(nil), statuses...)
	}
}

// WithOverride allows replacing an existing Cache-Control header.
func WithOverride() Option {
	return func(cfg *Config) {
		cfg.Override = true
	}
}

// WithAllowAuthorization allows cache headers on requests with Authorization.
func WithAllowAuthorization() Option {
	return func(cfg *Config) {
		cfg.AllowAuthorization = true
	}
}

// WithAllowSetCookie allows cache headers on responses that set cookies.
func WithAllowSetCookie() Option {
	return func(cfg *Config) {
		cfg.AllowSetCookie = true
	}
}

type cacheWriter struct {
	gin.ResponseWriter
	beforeWrite func()
}

func (w *cacheWriter) WriteHeaderNow() {
	if !w.Written() {
		w.apply()
	}
	w.ResponseWriter.WriteHeaderNow()
}

func (w *cacheWriter) Write(data []byte) (int, error) {
	w.WriteHeaderNow()
	return w.ResponseWriter.Write(data)
}

func (w *cacheWriter) WriteString(s string) (int, error) {
	w.WriteHeaderNow()
	return w.ResponseWriter.WriteString(s)
}

func (w *cacheWriter) Flush() {
	w.WriteHeaderNow()
	w.ResponseWriter.Flush()
}

func (w *cacheWriter) apply() {
	if w.beforeWrite != nil {
		w.beforeWrite()
	}
}

func applyOptions(cfg *Config, opts []Option) {
	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}
}

func writeHeaders(c *gin.Context, cfg Config) {
	if !allowed(c, cfg) {
		return
	}

	headers := c.Writer.Header()
	if headers.Get(headerCacheControl) != "" && !cfg.Override {
		return
	}

	cacheControl := buildCacheControl(cfg.Directives)
	if cacheControl != "" {
		headers.Set(headerCacheControl, cacheControl)
	}

	if cfg.Cloudflare.Enabled {
		writeCloudflareHeaders(c, headers, cfg.Cloudflare, cacheControl)
	}
}

func allowed(c *gin.Context, cfg Config) bool {
	if !methodAllowed(c.Request.Method, cfg.Methods) {
		return false
	}
	if !statusAllowed(c.Writer.Status(), cfg.Statuses) {
		return false
	}
	if !cfg.AllowAuthorization && c.GetHeader("Authorization") != "" {
		return false
	}
	if !cfg.AllowSetCookie && len(c.Writer.Header().Values("Set-Cookie")) > 0 {
		return false
	}
	return true
}

func methodAllowed(method string, methods []string) bool {
	if len(methods) == 0 {
		return method == http.MethodGet || method == http.MethodHead
	}

	for _, allowed := range methods {
		if strings.EqualFold(method, allowed) {
			return true
		}
	}
	return false
}

func statusAllowed(status int, statuses []int) bool {
	if len(statuses) == 0 {
		return status >= 200 && status < 400
	}

	for _, allowed := range statuses {
		if status == allowed {
			return true
		}
	}
	return false
}

func buildCacheControl(d Directives) string {
	if d.NoStore {
		return "no-store"
	}

	var parts []string
	if d.Private {
		parts = append(parts, "private")
	} else if d.Public {
		parts = append(parts, "public")
	}
	if d.NoCache {
		parts = append(parts, "no-cache")
	}

	parts = appendDuration(parts, "max-age", d.MaxAge)
	parts = appendDuration(parts, "s-maxage", d.SharedMaxAge)
	parts = appendDuration(parts, "stale-while-revalidate", d.StaleWhileRevalidate)
	parts = appendDuration(parts, "stale-if-error", d.StaleIfError)

	if d.MustRevalidate {
		parts = append(parts, "must-revalidate")
	}
	if d.Immutable {
		parts = append(parts, "immutable")
	}

	return strings.Join(parts, ", ")
}

func appendDuration(parts []string, name string, ttl time.Duration) []string {
	if ttl <= 0 {
		return parts
	}
	return append(parts, name+"="+strconv.FormatInt(int64(ttl/time.Second), 10))
}

func writeCloudflareHeaders(c *gin.Context, headers http.Header, cfg CloudflareConfig, cacheControl string) {
	cloudflareCacheControl := cacheControl
	if cfg.Directives != nil {
		cloudflareCacheControl = buildCacheControl(*cfg.Directives)
	}
	if cloudflareCacheControl != "" && (cfg.Override || headers.Get(headerCloudflareCacheControl) == "") {
		headers.Set(headerCloudflareCacheControl, cloudflareCacheControl)
	}

	tags := buildCacheTags(c, cfg)
	if tags != "" && (cfg.Override || headers.Get(headerCacheTag) == "") {
		headers.Set(headerCacheTag, tags)
	}
}

func buildCacheTags(c *gin.Context, cfg CloudflareConfig) string {
	raw := append([]string(nil), cfg.Tags...)
	if cfg.TagFunc != nil {
		raw = append(raw, cfg.TagFunc(c)...)
	}

	seen := make(map[string]struct{}, len(raw))
	tags := make([]string, 0, len(raw))
	total := 0
	for _, tag := range raw {
		if !validCacheTag(tag) {
			continue
		}

		key := strings.ToLower(tag)
		if _, ok := seen[key]; ok {
			continue
		}

		extra := len(tag)
		if len(tags) > 0 {
			extra++
		}
		if total+extra > maxCacheTagHeaderLen {
			continue
		}

		seen[key] = struct{}{}
		tags = append(tags, tag)
		total += extra
	}

	return strings.Join(tags, ",")
}

func validCacheTag(tag string) bool {
	if tag == "" {
		return false
	}

	for i := 0; i < len(tag); i++ {
		b := tag[i]
		if b < 33 || b > 126 || b == ',' {
			return false
		}
	}
	return true
}
