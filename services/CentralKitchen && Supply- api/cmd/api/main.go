// @title           Inventory API
// @version         1.0
// @description     Inventory management for Central Kitchen & Branches.
// @host            localhost:9004
// @BasePath        /api/v1
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	_ "github.com/NorthstarEngineering/RAS-Backend/services/inventory-api/docs"
	"github.com/NorthstarEngineering/RAS-Backend/services/inventory-api/internal/inventory"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
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

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}
	log.Println("✅ Connected to PostgreSQL")

	invRepo := inventory.NewRepository(db)
	invSvc := inventory.NewService(db, invRepo)
	invHandler := inventory.NewHandler(invSvc)

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "inventory-api",
		})
	})

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL("/swagger/doc.json")))

	api := router.Group("/api/v1")
	invHandler.RegisterRoutes(api)

	port := getEnv("PORT", "9004")
	log.Printf("🚀 Inventory API listening on :%s", port)
	log.Printf("   ❤️  Health:   http://localhost:%s/health", port)

	if err := router.Run(":" + port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
