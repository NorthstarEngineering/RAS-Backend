// @title           Sales API
// @version         1.0
// @description     Sales, Shifts, and Discounts API for Restaurant Branch Management.
// @host            localhost:9006
// @BasePath        /api/v1
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	_ "github.com/NorthstarEngineering/RAS-Backend/services/sales-api/docs"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/NorthstarEngineering/RAS-Backend/services/sales-api/internal/sales"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func main() {
	// قراءة إعدادات الداتابيز من البيئة (متوافقة مع Docker)
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbName := getEnv("DB_NAME", "restaurant_db")
	dbUser := getEnv("DB_USER", "postgres")
	dbPass := getEnv("DB_PASSWORD", "Postgres@123")

	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", dbUser, dbPass, dbHost, dbPort, dbName)

	db, err := sqlx.Connect("postgres", dbURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()
	log.Println("✅ Connected to PostgreSQL")

	// Setup
	repo := sales.NewRepository(db)
	svc := sales.NewService(db, repo)
	handler := sales.NewHandler(svc)

	// Gin Router
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "sales-api"})
	})

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	port := getEnv("PORT", "9006")
	log.Printf("🚀 Sales API listening on :%s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
