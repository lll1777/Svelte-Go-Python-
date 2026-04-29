package main

import (
	"log"

	"agriculture-platform/config"
	"agriculture-platform/database"
	"agriculture-platform/routes"
)

func main() {
	log.Println("Starting Agriculture Platform Backend...")

	cfg := config.LoadConfig()

	database.InitDB(cfg)

	r := routes.SetupRouter(cfg)

	log.Printf("Server starting on port %s", cfg.ServerPort)
	if err := r.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
