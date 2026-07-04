package cachecontrol

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMethodAllowed(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		allowed []string
		want    bool
	}{
		{name: "default allows GET", method: http.MethodGet, want: true},
		{name: "default allows HEAD", method: http.MethodHead, want: true},
		{name: "default rejects POST", method: http.MethodPost, want: false},
		{name: "custom list replaces default", method: http.MethodGet, allowed: []string{http.MethodPost}, want: false},
		{name: "custom list matches case insensitive", method: "post", allowed: []string{http.MethodPost}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := methodAllowed(tt.method, tt.allowed); got != tt.want {
				t.Fatalf("methodAllowed(%q, %v) = %v, want %v", tt.method, tt.allowed, got, tt.want)
			}
		})
	}
}

func TestStatusAllowed(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		statuses []int
		want     bool
	}{
		{name: "default allows 200", status: http.StatusOK, want: true},
		{name: "default allows 302", status: http.StatusFound, want: true},
		{name: "default rejects 199", status: 199, want: false},
		{name: "default rejects 400", status: http.StatusBadRequest, want: false},
		{name: "custom list replaces default", status: http.StatusOK, statuses: []int{http.StatusNotFound}, want: false},
		{name: "custom list matches exact status", status: http.StatusNotFound, statuses: []int{http.StatusNotFound}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := statusAllowed(tt.status, tt.statuses); got != tt.want {
				t.Fatalf("statusAllowed(%d, %v) = %v, want %v", tt.status, tt.statuses, got, tt.want)
			}
		})
	}
}

func TestValidCacheTag(t *testing.T) {
	tests := []struct {
		tag  string
		want bool
	}{
		{tag: "products", want: true},
		{tag: "product:123", want: true},
		{tag: "", want: false},
		{tag: "bad tag", want: false},
		{tag: "bad,tag", want: false},
		{tag: "bad\ntag", want: false},
		{tag: string([]byte{0x7f}), want: false},
		{tag: "sản-phẩm", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			if got := validCacheTag(tt.tag); got != tt.want {
				t.Fatalf("validCacheTag(%q) = %v, want %v", tt.tag, got, tt.want)
			}
		})
	}
}

func TestBuildCacheTags(t *testing.T) {
	got := buildCacheTags(nil, CloudflareConfig{
		Tags: []string{"products", "Products", "", "bad tag", "product:1"},
		TagFunc: func(_ *gin.Context) []string {
			return []string{"product:2", "PRODUCT:1", "feed"}
		},
	})

	if got != "products,product:1,product:2,feed" {
		t.Fatalf("buildCacheTags() = %q, want %q", got, "products,product:1,product:2,feed")
	}
}

func TestBuildCacheTagsHonorsHeaderLimit(t *testing.T) {
	full := strings.Repeat("a", maxCacheTagHeaderLen)
	got := buildCacheTags(nil, CloudflareConfig{Tags: []string{full, "b"}})

	if got != full {
		t.Fatalf("buildCacheTags() length = %d, want %d", len(got), len(full))
	}
}
