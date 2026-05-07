package auth

import (
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	accessTokenTTL  = 15 * time.Minute
	refreshTokenTTL = 7 * 24 * time.Hour
	minSecretLength = 16
)

type UserSummary struct {
	ID      uint
	Name    string
	Profile string
}

type AccessClaims struct {
	Usarname string `json:"usarname"`
	Profile  string `json:"profile"`
	ID       uint   `json:"id"`
	jwt.RegisteredClaims
}

type RefreshClaims struct {
	ID           uint `json:"id"`
	TokenVersion int  `json:"tokenVersion"`
	jwt.RegisteredClaims
}

func CreateAccessToken(user UserSummary, secret []byte) (string, error) {
	now := time.Now()
	claims := AccessClaims{
		Usarname: user.Name,
		Profile:  user.Profile,
		ID:       user.ID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(accessTokenTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

func CreateRefreshToken(userID uint, tokenVersion int, secret []byte) (string, error) {
	now := time.Now()
	claims := RefreshClaims{
		ID:           userID,
		TokenVersion: tokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(refreshTokenTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

func ParseAccessToken(tokenStr string, secret []byte) (*AccessClaims, error) {
	parser := jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	claims := &AccessClaims{}
	_, err := parser.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		return secret, nil
	})
	if err != nil {
		return nil, err
	}
	return claims, nil
}

func ParseRefreshToken(tokenStr string, secret []byte) (*RefreshClaims, error) {
	parser := jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	claims := &RefreshClaims{}
	_, err := parser.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		return secret, nil
	})
	if err != nil {
		return nil, err
	}
	return claims, nil
}

func MustLoadSecrets() (accessSecret, refreshSecret []byte) {
	access := os.Getenv("JWT_SECRET")
	refresh := os.Getenv("JWT_REFRESH_SECRET")
	env := os.Getenv("APP_ENV")
	if env != "test" {
		if len(access) < minSecretLength {
			panic(fmt.Sprintf("JWT_SECRET must be at least %d bytes", minSecretLength))
		}
		if len(refresh) < minSecretLength {
			panic(fmt.Sprintf("JWT_REFRESH_SECRET must be at least %d bytes", minSecretLength))
		}
	}
	if access == "" {
		access = "test-access-secret-development-only"
	}
	if refresh == "" {
		refresh = "test-refresh-secret-development-only"
	}
	return []byte(access), []byte(refresh)
}

