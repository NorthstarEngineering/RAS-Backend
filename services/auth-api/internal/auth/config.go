package auth

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	JWTSecret         []byte
	JWTIssuer         string
	AccessTokenTTL    time.Duration
	RefreshTokenTTL   time.Duration
	MaxFailedAttempts int
	BaseLockout       time.Duration
	MaxLockout        time.Duration
	BcryptCost        int
	LoginRatePerMin   int // per-IP limit on login/refresh
}

func LoadConfig() (Config, error) {
	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 {
		return Config{}, fmt.Errorf("JWT_SECRET must be set and at least 32 characters long")
	}
	return Config{
		JWTSecret:         []byte(secret),
		JWTIssuer:         "restaurant-backend",
		AccessTokenTTL:    envDuration("AUTH_ACCESS_TTL", 15*time.Minute),
		RefreshTokenTTL:   envDuration("AUTH_REFRESH_TTL", 30*24*time.Hour),
		MaxFailedAttempts: envInt("AUTH_MAX_FAILED_ATTEMPTS", 5),
		BaseLockout:       envDuration("AUTH_BASE_LOCKOUT", 15*time.Minute),
		MaxLockout:        envDuration("AUTH_MAX_LOCKOUT", time.Hour),
		BcryptCost:        envInt("AUTH_BCRYPT_COST", 12),
		LoginRatePerMin:   envInt("AUTH_LOGIN_RATE_PER_MIN", 10),
	}, nil
}

func envDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return def
	}
	return d
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}
