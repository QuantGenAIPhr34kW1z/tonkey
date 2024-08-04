package api

import (
	"context"
	"net/http"
	"strings"

	"tonkey/internal/auth"
)

type AuthCtx struct {
	User   string
	UserID int64
}

type ctxKey string

const authKey ctxKey = "auth"

func WithAuth(secret []byte, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			return
		}
		token := strings.TrimPrefix(h, "Bearer ")
		claims, err := auth.Verify(secret, token)
		if err != nil {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		r = r.WithContext(withAuth(r.Context(), &AuthCtx{User: claims.User}))
		next.ServeHTTP(w, r)
	})
}

func withAuth(ctx context.Context, a *AuthCtx) context.Context {
	return context.WithValue(ctx, authKey, a)
}

func fromAuth(ctx context.Context) *AuthCtx {
	if v := ctx.Value(authKey); v != nil {
		if a, ok := v.(*AuthCtx); ok {
			return a
		}
	}
	return nil
}

// GetUser returns authenticated user info
func (a *AuthCtx) GetUser() string {
	if a == nil {
		return ""
	}
	return a.User
}
