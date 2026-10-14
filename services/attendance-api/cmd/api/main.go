// Package main is the entry point for the Attendance API.
//
// @title Restaurant Attendance API
// @version 1.0
// @description Attendance, Employee Management, and Overtime modules
// @host localhost:9003
// @BasePath /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Use Bearer followed by the Auth API access token.
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
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "restaurant-api/docs"
	"restaurant-api/internal/attendance"
	"restaurant-api/internal/authn"
	"restaurant-api/internal/employee"
	"restaurant-api/internal/overtime"
)

const defaultPort = "9003"

const (
	CtxUserID   = authn.UserIDKey
	CtxUserRole = authn.RoleKey
	CtxBranchID = authn.BranchIDKey
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("service stopped", "service", "attendance-api", "error", err)
		os.Exit(1)
	}
}

func run() error {
	_ = godotenv.Load()
	secret := []byte(os.Getenv("JWT_SECRET"))
	if len(secret) < 32 {
		return fmt.Errorf("JWT_SECRET must be set and at least 32 characters long")
	}
	dsn, err := databaseURL()
	if err != nil {
		return err
	}
	startupCtx, startupCancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer startupCancel()
	db, err := sqlx.ConnectContext(startupCtx, "postgres", dsn)
	if err != nil {
		return fmt.Errorf("database is unavailable")
	}
	defer db.Close()
	db.SetMaxOpenConns(envInt("DB_MAX_OPEN_CONNS", 5))
	db.SetMaxIdleConns(envInt("DB_MAX_IDLE_CONNS", 2))
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	employeeRepo := employee.NewRepository(db)
	employeeSvc := employee.NewService(db, employeeRepo)
	employeeHandler := employee.NewHandler(employeeSvc)
	attendanceRepo := attendance.NewRepository(db)
	attendanceSvc := attendance.NewService(db, attendanceRepo, employeeReaderAdapter{svc: employeeSvc}, branchReaderAdapter{db: db})
	attendanceHandler := attendance.NewHandler(attendanceSvc)
	overtimeRepo := overtime.NewRepository(db)
	overtimeSvc := overtime.NewService(db, overtimeRepo)
	overtimeHandler := overtime.NewHandler(overtimeSvc)
	overtimeHandler.SetEmployeeBranchLookup(func(ctx context.Context, employeeID string) (string, error) {
		emp, err := employeeRepo.GetByID(ctx, employeeID)
		if err != nil {
			return "", err
		}
		if emp == nil {
			return "", overtime.ErrEmployeeNotFound
		}
		return emp.BranchID, nil
	})

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery(), requestLogging("attendance-api"))
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	router.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "attendance-api"}) })
	api := router.Group("/api/v1")
	api.Use(authn.Middleware(secret), authn.RequireRoles("CEO", "BRANCH_MANAGER"))
	attendanceHandler.RegisterRoutes(api)
	employeeHandler.RegisterRoutes(api)
	overtimeHandler.RegisterRoutes(api)

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}
	srv := &http.Server{Addr: ":" + port, Handler: router, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	serverErr := make(chan error, 1)
	go func() { serverErr <- srv.ListenAndServe() }()
	slog.Info("http server started", "service", "attendance-api", "port", port)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-serverErr:
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("http server failed")
		}
		return nil
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("http server shutdown failed")
		}
		return nil
	}
}

func databaseURL() (string, error) {
	if dsn := strings.TrimSpace(os.Getenv("DATABASE_URL")); dsn != "" {
		return dsn, nil
	}
	host, port, user, password, name := os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_NAME")
	if host == "" || port == "" || user == "" || password == "" || name == "" {
		return "", fmt.Errorf("set DATABASE_URL or all DB_HOST/DB_PORT/DB_USER/DB_PASSWORD/DB_NAME values")
	}
	u := url.URL{Scheme: "postgres", User: url.UserPassword(user, password), Host: net.JoinHostPort(host, port), Path: name}
	q := u.Query()
	ssl := os.Getenv("DB_SSLMODE")
	if ssl == "" {
		ssl = "disable"
	}
	q.Set("sslmode", ssl)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	var n int
	if _, err := fmt.Sscan(value, &n); err != nil || n < 1 {
		return fallback
	}
	return n
}

func requestLogging(service string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.GetHeader("X-Request-ID"))
		if id == "" || len(id) > 128 {
			id = uuid.NewString()
		} else {
			for _, r := range id {
				if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.') {
					id = uuid.NewString()
					break
				}
			}
		}
		c.Set("request_id", id)
		c.Header("X-Request-ID", id)
		start := time.Now()
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		slog.Info("http request", "service", service, "request_id", id, "method", c.Request.Method, "route", route, "status", c.Writer.Status(), "duration_ms", time.Since(start).Milliseconds())
	}
}
