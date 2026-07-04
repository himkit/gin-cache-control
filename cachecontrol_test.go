package cachecontrol

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestBuildCacheControl(t *testing.T) {
	tests := []struct {
		name string
		in   Directives
		want string
	}{
		{
			name: "public max age",
			in:   Directives{Public: true, MaxAge: time.Minute},
			want: "public, max-age=60",
		},
		{
			name: "private max age",
			in:   Directives{Private: true, MaxAge: 30 * time.Second},
			want: "private, max-age=30",
		},
		{
			name: "shared and stale directives",
			in: Directives{
				Public:               true,
				MaxAge:               time.Minute,
				SharedMaxAge:         10 * time.Minute,
				StaleWhileRevalidate: 30 * time.Second,
				StaleIfError:         5 * time.Minute,
				MustRevalidate:       true,
				Immutable:            true,
			},
			want: "public, max-age=60, s-maxage=600, stale-while-revalidate=30, stale-if-error=300, must-revalidate, immutable",
		},
		{
			name: "no store wins",
			in:   Directives{Public: true, MaxAge: time.Minute, NoStore: true},
			want: "no-store",
		},
		{
			name: "private wins over public",
			in:   Directives{Public: true, Private: true, MaxAge: time.Minute},
			want: "private, max-age=60",
		},
		{
			name: "no cache combines with ttl",
			in:   Directives{NoCache: true, MaxAge: time.Minute},
			want: "no-cache, max-age=60",
		},
		{
			name: "empty",
			in:   Directives{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildCacheControl(tt.in); got != tt.want {
				t.Fatalf("buildCacheControl() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildCacheControlSkipsNegativeDurations(t *testing.T) {
	got := buildCacheControl(Directives{
		Public:               true,
		MaxAge:               -time.Second,
		SharedMaxAge:         -time.Second,
		StaleWhileRevalidate: -time.Second,
		StaleIfError:         -time.Second,
	})
	if got != "public" {
		t.Fatalf("buildCacheControl() = %q, want %q", got, "public")
	}
}

func TestMiddlewareDefaultSafetyGuards(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		status     int
		auth       bool
		setCookie  bool
		wantHeader string
	}{
		{name: "get 200 writes", method: http.MethodGet, status: http.StatusOK, wantHeader: "public, max-age=60"},
		{name: "head 200 writes", method: http.MethodHead, status: http.StatusOK, wantHeader: "public, max-age=60"},
		{name: "post 200 skips", method: http.MethodPost, status: http.StatusOK},
		{name: "get 500 skips", method: http.MethodGet, status: http.StatusInternalServerError},
		{name: "authorization skips", method: http.MethodGet, status: http.StatusOK, auth: true},
		{name: "set cookie skips", method: http.MethodGet, status: http.StatusOK, setCookie: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := performRequest(tt.method, "/", Public(time.Minute), func(c *gin.Context) {
				if tt.setCookie {
					c.SetCookie("sid", "1", 60, "/", "", false, true)
				}
				c.Status(tt.status)
			}, func(req *http.Request) {
				if tt.auth {
					req.Header.Set("Authorization", "Bearer token")
				}
			})

			if got := responseHeader(w, "Cache-Control"); got != tt.wantHeader {
				t.Fatalf("Cache-Control = %q, want %q", got, tt.wantHeader)
			}
		})
	}
}

func TestMiddlewareSetsHeadersBeforeJSONWrite(t *testing.T) {
	w := performRequest(http.MethodGet, "/", Public(time.Minute, WithCloudflare()), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	if got := responseHeader(w, "Cache-Control"); got != "public, max-age=60" {
		t.Fatalf("Cache-Control = %q, want %q", got, "public, max-age=60")
	}
	if got := responseHeader(w, "Cloudflare-CDN-Cache-Control"); got != "public, max-age=60" {
		t.Fatalf("Cloudflare-CDN-Cache-Control = %q, want %q", got, "public, max-age=60")
	}
}

func TestMiddlewareSkipsUnsafeJSONWrites(t *testing.T) {
	tests := []struct {
		name    string
		handler gin.HandlerFunc
	}{
		{
			name: "json 500 skips",
			handler: func(c *gin.Context) {
				c.JSON(http.StatusInternalServerError, gin.H{"error": true})
			},
		},
		{
			name: "set cookie then json skips",
			handler: func(c *gin.Context) {
				c.SetCookie("sid", "1", 60, "/", "", false, true)
				c.JSON(http.StatusOK, gin.H{"ok": true})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := performRequest(http.MethodGet, "/", Public(time.Minute), tt.handler)

			if got := responseHeader(w, "Cache-Control"); got != "" {
				t.Fatalf("Cache-Control = %q, want empty", got)
			}
		})
	}
}

func TestMiddlewareAllowsConfiguredMethodsAndStatuses(t *testing.T) {
	w := performRequest(http.MethodPost, "/", New(
		WithNoCache(),
		WithMethods(http.MethodPost),
		WithStatuses(http.StatusInternalServerError),
		WithAllowAuthorization(),
		WithAllowSetCookie(),
	), func(c *gin.Context) {
		c.SetCookie("sid", "1", 60, "/", "", false, true)
		c.Status(http.StatusInternalServerError)
	}, func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer token")
	})

	if got := responseHeader(w, "Cache-Control"); got != "no-cache" {
		t.Fatalf("Cache-Control = %q, want %q", got, "no-cache")
	}
}

func TestNewWithBasicOptions(t *testing.T) {
	w := performRequest(http.MethodGet, "/", New(
		WithPublic(),
		WithMaxAge(time.Minute),
		WithSharedMaxAge(10*time.Minute),
	), okHandler)

	if got := responseHeader(w, "Cache-Control"); got != "public, max-age=60, s-maxage=600" {
		t.Fatalf("Cache-Control = %q, want %q", got, "public, max-age=60, s-maxage=600")
	}
}

func TestMiddlewareDoesNotOverrideCacheControlByDefault(t *testing.T) {
	w := performRequest(http.MethodGet, "/", Public(time.Minute), func(c *gin.Context) {
		c.Header("Cache-Control", "private")
		c.Status(http.StatusOK)
	})

	if got := responseHeader(w, "Cache-Control"); got != "private" {
		t.Fatalf("Cache-Control = %q, want %q", got, "private")
	}
}

func TestMiddlewareOverrideCacheControl(t *testing.T) {
	w := performRequest(http.MethodGet, "/", Public(time.Minute, WithOverride()), func(c *gin.Context) {
		c.Header("Cache-Control", "private")
		c.Status(http.StatusOK)
	})

	if got := responseHeader(w, "Cache-Control"); got != "public, max-age=60" {
		t.Fatalf("Cache-Control = %q, want %q", got, "public, max-age=60")
	}
}

func TestPolicyFunc(t *testing.T) {
	t.Run("false skips headers", func(t *testing.T) {
		w := performRequest(http.MethodGet, "/", New(WithPolicy(func(c *gin.Context) (Config, bool) {
			return Config{}, false
		})), okHandler)

		if got := responseHeader(w, "Cache-Control"); got != "" {
			t.Fatalf("Cache-Control = %q, want empty", got)
		}
	})

	t.Run("returned config replaces original", func(t *testing.T) {
		w := performRequest(http.MethodGet, "/", Public(time.Minute, WithPolicy(func(c *gin.Context) (Config, bool) {
			return Config{Directives: Directives{Private: true, MaxAge: 2 * time.Minute}}, true
		})), okHandler)

		if got := responseHeader(w, "Cache-Control"); got != "private, max-age=120" {
			t.Fatalf("Cache-Control = %q, want %q", got, "private, max-age=120")
		}
	})
}

func TestCloudflareCloneHeader(t *testing.T) {
	w := performRequest(http.MethodGet, "/", Public(time.Minute, WithCloudflare()), okHandler)

	if got := responseHeader(w, "Cache-Control"); got != "public, max-age=60" {
		t.Fatalf("Cache-Control = %q, want %q", got, "public, max-age=60")
	}
	if got := responseHeader(w, "Cloudflare-CDN-Cache-Control"); got != "public, max-age=60" {
		t.Fatalf("Cloudflare-CDN-Cache-Control = %q, want %q", got, "public, max-age=60")
	}
}

func TestCloudflareCustomDirectives(t *testing.T) {
	w := performRequest(http.MethodGet, "/", Public(time.Minute,
		WithCloudflareDirectives(Directives{
			Public:       true,
			MaxAge:       10 * time.Minute,
			StaleIfError: 5 * time.Minute,
		}),
	), okHandler)

	if got := responseHeader(w, "Cache-Control"); got != "public, max-age=60" {
		t.Fatalf("Cache-Control = %q, want %q", got, "public, max-age=60")
	}
	if got := responseHeader(w, "Cloudflare-CDN-Cache-Control"); got != "public, max-age=600, stale-if-error=300" {
		t.Fatalf("Cloudflare-CDN-Cache-Control = %q, want %q", got, "public, max-age=600, stale-if-error=300")
	}
}

func TestCloudflareTags(t *testing.T) {
	w := performRequest(http.MethodGet, "/products/123", Public(time.Minute,
		WithCloudflareTags("products", "", "bad tag", "Product:1", "product:1", "bad,comma"),
		WithCloudflareTagFunc(func(c *gin.Context) []string {
			return []string{"product:" + c.Param("id"), "feed", string([]byte{0x7f})}
		}),
	), okHandler)

	if got := responseHeader(w, "Cache-Tag"); got != "products,Product:1,product:123,feed" {
		t.Fatalf("Cache-Tag = %q, want %q", got, "products,Product:1,product:123,feed")
	}
}

func TestCloudflareOverride(t *testing.T) {
	t.Run("keeps existing cloudflare headers by default", func(t *testing.T) {
		w := performRequest(http.MethodGet, "/", Public(time.Minute,
			WithCloudflare(),
			WithCloudflareTags("new"),
		), func(c *gin.Context) {
			c.Header("Cloudflare-CDN-Cache-Control", "old")
			c.Header("Cache-Tag", "old")
			c.Status(http.StatusOK)
		})

		if got := responseHeader(w, "Cloudflare-CDN-Cache-Control"); got != "old" {
			t.Fatalf("Cloudflare-CDN-Cache-Control = %q, want %q", got, "old")
		}
		if got := responseHeader(w, "Cache-Tag"); got != "old" {
			t.Fatalf("Cache-Tag = %q, want %q", got, "old")
		}
	})

	t.Run("overrides existing cloudflare headers", func(t *testing.T) {
		w := performRequest(http.MethodGet, "/", FromConfig(Config{
			Directives: Directives{Public: true, MaxAge: time.Minute},
			Cloudflare: CloudflareConfig{
				Enabled:  true,
				Tags:     []string{"new"},
				Override: true,
			},
		}), func(c *gin.Context) {
			c.Header("Cloudflare-CDN-Cache-Control", "old")
			c.Header("Cache-Tag", "old")
			c.Status(http.StatusOK)
		})

		if got := responseHeader(w, "Cloudflare-CDN-Cache-Control"); got != "public, max-age=60" {
			t.Fatalf("Cloudflare-CDN-Cache-Control = %q, want %q", got, "public, max-age=60")
		}
		if got := responseHeader(w, "Cache-Tag"); got != "new" {
			t.Fatalf("Cache-Tag = %q, want %q", got, "new")
		}
	})
}

func okHandler(c *gin.Context) {
	c.Status(http.StatusOK)
}

func performRequest(method, path string, middleware gin.HandlerFunc, handler gin.HandlerFunc, opts ...func(*http.Request)) *httptest.ResponseRecorder {
	router := gin.New()
	router.Handle(method, "/products/:id", middleware, handler)
	router.Handle(method, "/", middleware, handler)

	req := httptest.NewRequest(method, path, nil)
	for _, opt := range opts {
		opt(req)
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func responseHeader(w *httptest.ResponseRecorder, name string) string {
	return w.Result().Header.Get(name)
}
