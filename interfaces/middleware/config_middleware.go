package middleware

import (
	"context"
	"hash-signing-service/config"
	"net/http"
)

func SetConfigInContext(c *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), "config", c)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
