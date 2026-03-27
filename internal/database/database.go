package database

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func resolveEnvDatabaseURL() string {
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return ""
	}

	// Ignore deployment template placeholders when running locally.
	if strings.HasPrefix(databaseURL, "${{") && strings.HasSuffix(databaseURL, "}}") {
		return ""
	}

	if strings.HasPrefix(databaseURL, "${") && strings.HasSuffix(databaseURL, "}") {
		return ""
	}

	return databaseURL
}

func resolveDatabaseDSN() string {
	databaseURL := resolveEnvDatabaseURL()
	if databaseURL != "" {
		return databaseURL
	}

	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	name := os.Getenv("DB_NAME")
	sslmode := os.Getenv("DB_SSLMODE")

	if strings.TrimSpace(sslmode) == "" {
		sslmode = "disable"
	}

	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host,
		port,
		user,
		password,
		name,
		sslmode,
	)
}

func resolveDatabaseURL() string {
	databaseURL := resolveEnvDatabaseURL()
	if databaseURL != "" {
		return databaseURL
	}

	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	name := os.Getenv("DB_NAME")
	sslmode := os.Getenv("DB_SSLMODE")

	if strings.TrimSpace(sslmode) == "" {
		sslmode = "disable"
	}

	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		user,
		password,
		host,
		port,
		name,
		sslmode,
	)
}

func Connect() (*gorm.DB, error) {
	return gorm.Open(postgres.Open(resolveDatabaseDSN()), &gorm.Config{})
}

func ConnectPGXPool() (*pgxpool.Pool, error) {
	return pgxpool.New(context.Background(), resolveDatabaseURL())
}
