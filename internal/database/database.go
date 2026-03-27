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

func getenvAny(keys ...string) string {
	for _, key := range keys {
		value := strings.TrimSpace(os.Getenv(key))
		if value != "" {
			return strings.Trim(value, "\"'")
		}
	}

	return ""
}

func resolveEnvDatabaseURL() string {
	databaseURL := getenvAny(
		"DATABASE_URL",
		"POSTGRES_URL",
		"POSTGRESQL_URL",
		"DATABASE_PUBLIC_URL",
		"RENDER_DATABASE_URL",
	)
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

	host := getenvAny("DB_HOST", "PGHOST")
	port := getenvAny("DB_PORT", "PGPORT")
	user := getenvAny("DB_USER", "PGUSER")
	password := getenvAny("DB_PASSWORD", "PGPASSWORD")
	name := getenvAny("DB_NAME", "PGDATABASE")
	sslmode := getenvAny("DB_SSLMODE", "PGSSLMODE")

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

	host := getenvAny("DB_HOST", "PGHOST")
	port := getenvAny("DB_PORT", "PGPORT")
	user := getenvAny("DB_USER", "PGUSER")
	password := getenvAny("DB_PASSWORD", "PGPASSWORD")
	name := getenvAny("DB_NAME", "PGDATABASE")
	sslmode := getenvAny("DB_SSLMODE", "PGSSLMODE")

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
		return errors.New("database configuration missing: set DATABASE_URL (or POSTGRES_URL) or DB_HOST/DB_PORT/DB_USER/DB_NAME")
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
