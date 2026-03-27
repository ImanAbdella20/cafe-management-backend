package main

import (
	"log"
	"os"

	"backend/internal/database"
	"backend/internal/server"
	"backend/internal/user"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	db, err := database.Connect()
	if err != nil {
		log.Fatal(err)
	}

	if err := db.AutoMigrate(&user.User{}); err != nil {
		log.Fatal(err)
	}
	log.Println("auto migration completed")

	if err := user.CreateAdminIfNotExists(db); err != nil {
		log.Fatal(err)
	}
	log.Println("admin seed completed")

	if sqlDB, err := db.DB(); err == nil {
		if err := sqlDB.Ping(); err != nil {
			log.Fatal(err)
		}
		log.Println("database connected successfully")
		defer func() { _ = sqlDB.Close() }()
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	addr := ":" + port
	log.Printf("server running on %s", addr)
	if err := server.Run(addr, db); err != nil {
		log.Fatal(err)
	}
}
