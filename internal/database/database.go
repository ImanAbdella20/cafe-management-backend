package database

import (
	"context"
	"errors"
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

	// Railway private hostnames are only reachable in Railway runtime.
	if strings.Contains(databaseURL, ".railway.internal") && strings.TrimSpace(os.Getenv("RAILWAY_ENVIRONMENT")) == "" {
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

	if strings.TrimSpace(host) == "" || strings.TrimSpace(port) == "" || strings.TrimSpace(user) == "" || strings.TrimSpace(name) == "" {
		return ""
	}

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

	if strings.TrimSpace(host) == "" || strings.TrimSpace(port) == "" || strings.TrimSpace(user) == "" || strings.TrimSpace(name) == "" {
		return ""
	}

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

func validateResolvedDSN(dsn string) error {
	if strings.TrimSpace(dsn) == "" {
		return errors.New("database configuration missing: set DATABASE_URL or DB_HOST, DB_PORT, DB_USER, DB_NAME")
	}

	return nil
}

func Connect() (*gorm.DB, error) {
	dsn := resolveDatabaseDSN()
	if err := validateResolvedDSN(dsn); err != nil {
		return nil, err
	}

	return gorm.Open(postgres.Open(dsn), &gorm.Config{})
}

func ConnectPGXPool() (*pgxpool.Pool, error) {
	databaseURL := resolveDatabaseURL()
	if err := validateResolvedDSN(databaseURL); err != nil {
		return nil, err
	}

	return pgxpool.New(context.Background(), databaseURL)
}
