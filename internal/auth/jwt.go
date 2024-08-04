package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	User string `json:"user"`
	jwt.RegisteredClaims
}

func Issue(secret []byte, user string) (string, error) {
	claims := Claims{
		User: user,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "tonkey",
			Subject:   user,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(secret)
}

func Verify(secret []byte, token string) (*Claims, error) {
	t, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (any, error) {
		return secret, nil
	})
	if err != nil {
		return nil, err
	}
	c, ok := t.Claims.(*Claims)
	if !ok || !t.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}
	return c, nil
}
