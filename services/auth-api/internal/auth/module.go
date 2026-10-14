// Package auth wires the authentication module together. Business code lives
// in the sub-packages (model, errors, interfaces, repository, service, dto,
// handler, security); cmd/api only talks to this file.
package auth

import (
	"database/sql"
	"time"

	"restaurant-backend/internal/auth/handler"
	"restaurant-backend/internal/auth/interfaces"
	"restaurant-backend/internal/auth/repository"
	"restaurant-backend/internal/auth/security"
	"restaurant-backend/internal/auth/service"
)

type ModuleDeps struct {
	DB       *sql.DB
	Branches interfaces.BranchChecker
	Clock    interfaces.Clock // optional, defaults to the system clock
	Config   Config
}

type Module struct {
	Service    *service.Service
	Handler    *handler.Handler
	Middleware *handler.Middleware
}

func NewModule(d ModuleDeps) *Module {
	if d.Clock == nil {
		d.Clock = service.SystemClock{}
	}
	svc := service.New(service.Dependencies{
		Users:         repository.NewUserRepository(d.DB),
		RefreshTokens: repository.NewRefreshTokenRepository(d.DB),
		Branches:      d.Branches,
		Hasher:        security.NewBcryptHasher(d.Config.BcryptCost),
		Tokens:        security.NewJWTManager(d.Config.JWTSecret, d.Config.JWTIssuer, d.Config.AccessTokenTTL),
		Clock:         d.Clock,
	}, service.Settings{
		RefreshTokenTTL:   d.Config.RefreshTokenTTL,
		MaxFailedAttempts: d.Config.MaxFailedAttempts,
		BaseLockout:       d.Config.BaseLockout,
		MaxLockout:        d.Config.MaxLockout,
	})

	mw := handler.NewMiddleware(svc)
	limiter := handler.NewRateLimiter(d.Config.LoginRatePerMin, time.Minute, d.Clock)
	return &Module{Service: svc, Handler: handler.New(svc, mw, limiter), Middleware: mw}
}
