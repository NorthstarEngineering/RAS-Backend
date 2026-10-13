package main

import (
    "context"
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
    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/joho/godotenv"

    "restaurant-backend/internal/authn"
    "restaurant-backend/internal/branch"
)

func main() {
    slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
    if err := run(); err != nil { slog.Error("service stopped", "service", "branch-api", "error", err); os.Exit(1) }
}

func run() error {
    _ = godotenv.Load()
    secret := []byte(os.Getenv("JWT_SECRET"))
    if len(secret) < 32 { return fmt.Errorf("JWT_SECRET must be set and at least 32 characters long") }
    dsn, err := buildDBURL(); if err != nil { return err }
    poolCfg, err := pgxpool.ParseConfig(dsn); if err != nil { return fmt.Errorf("invalid database configuration") }
    poolCfg.MaxConns = int32(envInt("DB_MAX_OPEN_CONNS", 5)); poolCfg.MinConns = 0; poolCfg.MaxConnLifetime = 30*time.Minute; poolCfg.MaxConnIdleTime = 5*time.Minute; poolCfg.HealthCheckPeriod = time.Minute; poolCfg.ConnConfig.ConnectTimeout = 5*time.Second
    startupCtx, startupCancel := context.WithTimeout(context.Background(), 8*time.Second); defer startupCancel()
    pool, err := pgxpool.NewWithConfig(startupCtx, poolCfg); if err != nil { return fmt.Errorf("cannot create database pool") }; defer pool.Close()
    if err := pool.Ping(startupCtx); err != nil { return fmt.Errorf("database is unavailable") }

    service := branch.NewService(branch.NewPostgresRepository(pool), branch.SystemClock{})
    handler := branch.NewHandler(service)
    gin.SetMode(gin.ReleaseMode)
    router := gin.New()
    router.Use(gin.Recovery(), requestLogging("branch-api"))
    router.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status":"ok", "service":"branch-api"}) })
    api := router.Group("/api/v1")
    api.Use(authn.Middleware(secret))
    handler.RegisterRoutes(api, authn.RequireRoles)

    port := os.Getenv("PORT"); if port == "" { port = "9002" }
    srv := &http.Server{Addr:":"+port, Handler:router, ReadHeaderTimeout:5*time.Second, ReadTimeout:15*time.Second, WriteTimeout:30*time.Second, IdleTimeout:60*time.Second}
    serverErr := make(chan error, 1)
    go func(){ serverErr <- srv.ListenAndServe() }()
    slog.Info("http server started", "service", "branch-api", "port", port)
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM); defer stop()
    select {
    case err := <-serverErr:
        if err != nil && err != http.ErrServerClosed { return fmt.Errorf("http server failed") }; return nil
    case <-ctx.Done():
        shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second); defer cancel()
        if err := srv.Shutdown(shutdownCtx); err != nil { return fmt.Errorf("http server shutdown failed") }
        return nil
    }
}

func buildDBURL() (string, error) {
    if value := strings.TrimSpace(os.Getenv("DATABASE_URL")); value != "" { return value, nil }
    host, port, user, password, name := os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_NAME")
    if host == "" || port == "" || user == "" || password == "" || name == "" { return "", fmt.Errorf("set DATABASE_URL or all DB_HOST/DB_PORT/DB_USER/DB_PASSWORD/DB_NAME values") }
    u := url.URL{Scheme:"postgres", User:url.UserPassword(user,password), Host:net.JoinHostPort(host,port), Path:name}
    q := u.Query(); q.Set("sslmode", os.Getenv("DB_SSLMODE")); if q.Get("sslmode") == "" { q.Set("sslmode","disable") }; u.RawQuery=q.Encode()
    return u.String(), nil
}

func requestLogging(service string) gin.HandlerFunc {
    return func(c *gin.Context) {
        id := strings.TrimSpace(c.GetHeader("X-Request-ID"))
        if len(id) == 0 || len(id) > 128 { id = uuid.NewString() } else {
            for _, r := range id { if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.') { id = uuid.NewString(); break } }
        }
        c.Set("request_id", id); c.Header("X-Request-ID", id); start := time.Now(); c.Next()
        route := c.FullPath(); if route == "" { route = "unmatched" }
        slog.Info("http request", "service", service, "request_id", id, "method", c.Request.Method, "route", route, "status", c.Writer.Status(), "duration_ms", time.Since(start).Milliseconds())
    }
}
