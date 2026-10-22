// @title           Cost API
// @version         1.0
// @description     Cost & Profitability API.
// @host            localhost:9007
// @BasePath        /api/v1
package main

import (
	"log"
	"net/http"
	"os"

	_ "github.com/NorthstarEngineering/RAS-Backend/services/cost-api/docs"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/NorthstarEngineering/RAS-Backend/services/cost-api/internal/cost"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
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

	repo := cost.NewRepository(db)
	svc := cost.NewService(db, repo)
	handler := cost.NewHandler(svc)

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "cost-api"})
	})

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	log.Println("🚀 Cost API listening on :9007")
	if err := router.Run(":9007"); err != nil {
		log.Fatalf("server error: %v", err)
	}
}