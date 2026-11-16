// @title           Menu API
// @version         1.0
// @description     Master Menu & Pricing API for Restaurant Branch Management.
// @host            localhost:9005
// @BasePath        /api/v1
package main

import (
	"log"
	"net"
	"net/http"
	"net/url"
	"os"

	_ "github.com/NorthstarEngineering/RAS-Backend/services/menu-api/docs"
	"github.com/NorthstarEngineering/RAS-Backend/services/menu-api/internal/menu"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
	// قراءة إعدادات الداتابيز من البيئة
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbName := getEnv("DB_NAME", "restaurant_db")
	dbUser := getEnv("DB_USER", "postgres")
	dbPass := getEnv("DB_PASSWORD", "Postgres@123")

	dbURL := (&url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(dbUser, dbPass),
		Host:   net.JoinHostPort(dbHost, dbPort),
		Path:   dbName,
		RawQuery: url.Values{
			"sslmode": {getEnv("DB_SSLMODE", "disable")},
		}.Encode(),
	}).String()

	db, err := sqlx.Connect("postgres", dbURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()
	log.Println("✅ Connected to PostgreSQL")

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

	api := router.Group("/api/v1", gatewayIdentity)
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL("/swagger/doc.json")))
	handler.RegisterRoutes(api)

	port := getEnv("PORT", "9005")
	log.Printf("🚀 Menu API listening on :%s", port)
	if err := router.Run(":" + port); err != nil {
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
