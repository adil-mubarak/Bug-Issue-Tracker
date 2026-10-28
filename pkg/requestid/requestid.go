// Package requestid propagates a bounded request ID through HTTP services.
package requestid

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"regexp"
)

type key struct{}

var valid = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,64}$`)

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if !valid.MatchString(id) {
			var bytes [16]byte
			if _, err := rand.Read(bytes[:]); err != nil {
				id = "unknown"
			} else {
				id = hex.EncodeToString(bytes[:])
			}
		}
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), key{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
func FromContext(ctx context.Context) string { id, _ := ctx.Value(key{}).(string); return id }
