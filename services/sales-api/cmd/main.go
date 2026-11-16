// @title           Sales API
// @version         1.0
// @description     Sales, Shifts, and Discounts API for Restaurant Branch Management.
// @host            localhost:9006
// @BasePath        /api/v1
package main

import (
	"log"
	"net/http"
	"os"

	_ "github.com/NorthstarEngineering/RAS-Backend/services/sales-api/docs"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/NorthstarEngineering/RAS-Backend/services/sales-api/internal/sales"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

const (
	defaultPort = "9006"
)

func main() {
	// 1. الاتصال بقاعدة البيانات
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}
	db, err := sqlx.Connect("postgres", dbURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()
	log.Println("✅ Connected to PostgreSQL")

	// 2. Setup
	repo := sales.NewRepository(db)
	svc := sales.NewService(db, repo)
	handler := sales.NewHandler(svc)

	// 3. Gin Router
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// 4. Health Check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "sales-api"})
	})

	// 5. Swagger UI (لازم يكون بعد تعريف الـ router)
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// 6. API Routes
	api := router.Group("/api/v1", gatewayIdentity)
	handler.RegisterRoutes(api)

	// 7. تشغيل السيرفر
	log.Printf("🚀 Sales API listening on :%s", defaultPort)
	if err := router.Run(":" + defaultPort); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// gatewayIdentity reads authenticated identity from the private API proxy.
func gatewayIdentity(c *gin.Context) {
	actor, err := uuid.Parse(c.GetHeader("X-User-ID"))
	role := c.GetHeader("X-User-Role")
	if err != nil || actor == uuid.Nil || role == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": gin.H{
			"code": "unauthorized", "message": "authenticated user context is required",
		}})
		return
	}
	c.Set("user_id", actor.String())
	c.Set("user_role", role)
	c.Set("branch_id", c.GetHeader("X-Branch-ID"))
	c.Next()
}
