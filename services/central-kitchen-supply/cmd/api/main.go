// @title           Inventory API
// @version         1.0
// @description     Inventory management for Central Kitchen & Branches.
// @host            localhost:9004
// @BasePath        /api/v1
package main

import (
	"log"
	"net/http"
	"os"

	_ "github.com/NorthstarEngineering/RAS-Backend/services/inventory-api/docs" // ← ضفنا ده عشان Swagger
	"github.com/NorthstarEngineering/RAS-Backend/services/inventory-api/internal/inventory"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

const (
	defaultPort = "9004"
)

func main() {
	// ============================================================
	// 1. الاتصال بقاعدة البيانات
	// ============================================================
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}
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

	// ============================================================
	// 2. إعداد الـ Router
	// ============================================================
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// Health check (public)
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "inventory-api",
		})
	})

	// Swagger UI (تم تعديل المسار هنا)
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL("/swagger/doc.json")))

	// API v1
	api := router.Group("/api/v1")
	invHandler.RegisterRoutes(api)

	// ============================================================
	// 3. تشغيل السيرفر
	// ============================================================
	log.Printf("🚀 Inventory API listening on :%s", defaultPort)
	log.Printf("   ❤️  Health:   http://localhost:%s/health", defaultPort)

	if err := router.Run(":" + defaultPort); err != nil {
		log.Fatalf("server error: %v", err)
	}
}