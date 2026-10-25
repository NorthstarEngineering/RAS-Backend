// @title           Menu API
// @version         1.0
// @description     Master Menu & Pricing API for Restaurant Branch Management.
// @host            localhost:9005
// @BasePath        /api/v1
package main

import (
	"log"
	"net/http"
	"os"

	_ "github.com/NorthstarEngineering/RAS-Backend/services/menu-api/docs"
	"github.com/NorthstarEngineering/RAS-Backend/services/menu-api/internal/menu"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

const (
	defaultPort = "9005"
)

func main() {
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

	// Setup
	repo := menu.NewRepository(db)
	svc := menu.NewService(db, repo)
	handler := menu.NewHandler(svc)

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "menu-api"})
	})

	// 4. API v1
	// api := router.Group("/api/v1") // هنسجل الـ routes هنا بعدين
	api := router.Group("/api/v1")
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL("/swagger/doc.json")))
	handler.RegisterRoutes(api)

	log.Printf("🚀 Menu API listening on :%s", defaultPort)
	if err := router.Run(":" + defaultPort); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
