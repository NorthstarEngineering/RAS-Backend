package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"inventory-api/internal/inventory"
	"inventory-api/migrations"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	port := getEnv("PORT", "8080")
	dsn, err := buildDBURL()
	if err != nil {
		logger.Error("database config missing", "error", err)
		os.Exit(1)
	}
	if mode := os.Getenv("GIN_MODE"); mode != "" {
		gin.SetMode(mode)
	}

	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		logger.Error("db connect failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	migCtx, migCancel := context.WithTimeout(context.Background(), 30*time.Second)
	err = migrations.Up(migCtx, db.DB)
	migCancel()
	if err != nil {
		logger.Error("migrations failed", "error", err)
		os.Exit(1)
	}

	repo := inventory.NewRepository(db)
	// TODO: replace the placeholders with real adapters (catalog service, users service).
	svc := inventory.NewService(repo, inventory.NoopCatalog{}, inventory.NoopUserDirectory{}, logger)
	h := inventory.NewHandler(svc, logger)

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/readyz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "db_unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	h.RegisterRoutes(r.Group("/api/v1"))

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("inventory-api listening", "port", port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	logger.Info("server stopped")
}

func getEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func buildDBURL() (string, error) {
	if value := strings.TrimSpace(os.Getenv("DATABASE_URL")); value != "" {
		return value, nil
	}
	host, port, user, password, name := os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_NAME")
	if host == "" || port == "" || user == "" || password == "" || name == "" {
		return "", fmt.Errorf("set DATABASE_URL or all DB_HOST/DB_PORT/DB_USER/DB_PASSWORD/DB_NAME values")
	}
	u := url.URL{Scheme: "postgres", User: url.UserPassword(user, password), Host: net.JoinHostPort(host, port), Path: name}
	q := u.Query()
	q.Set("sslmode", os.Getenv("DB_SSLMODE"))
	if q.Get("sslmode") == "" {
		q.Set("sslmode", "disable")
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}