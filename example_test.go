package cachecontrol_test

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	cachecontrol "github.com/himkit/gin-cache-control"
)

func ExamplePublic() {
	router := gin.New()

	router.GET("/products",
		cachecontrol.Public(5*time.Minute),
		func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"items": []string{"book", "pen"}})
		},
	)

	_ = router
}

func ExamplePublic_cloudflare() {
	router := gin.New()

	router.GET("/products/:id",
		cachecontrol.Public(1*time.Minute,
			cachecontrol.WithCloudflareDirectives(cachecontrol.Directives{
				Public:       true,
				MaxAge:       10 * time.Minute,
				StaleIfError: 5 * time.Minute,
			}),
			cachecontrol.WithCloudflareTags("products"),
			cachecontrol.WithCloudflareTagFunc(func(c *gin.Context) []string {
				return []string{"product:" + c.Param("id")}
			}),
		),
		func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"id": c.Param("id")})
		},
	)

	_ = router
}

func ExampleNew_policy() {
	router := gin.New()

	router.GET("/feed",
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
		})),
		func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"ok": true})
		},
	)

	_ = router
}
