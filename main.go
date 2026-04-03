package main

import (
	"os"
	"petroapp/db"
	"petroapp/routes"
	"petroapp/server"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func main() {
	database := db.NewInMemoryDB()
	logger := logrus.New()

	srv := &server.Server{
		Database: database,
		Logger:   logger,
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	routes.RegisterRoutes(r, srv)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	logger.Infof("server starting on :%s", port)
	if err := r.Run(":" + port); err != nil {
		logger.Fatal(err)
	}
}
