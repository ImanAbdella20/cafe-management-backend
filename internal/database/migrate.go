package database

import (
	"context"
	"fmt"

	"backend/internal/inventory"
	"backend/internal/menu"
	"backend/internal/orders"
	"backend/internal/payment"
	"backend/internal/user"

	"gorm.io/gorm"
)

func AutoMigrateAll(db *gorm.DB) error {
	if err := db.AutoMigrate(&user.User{}); err != nil {
		return fmt.Errorf("automigrate users: %w", err)
	}

	pgxPool, err := ConnectPGXPool()
	if err != nil {
		return fmt.Errorf("connect pgx for schema migration: %w", err)
	}
	defer pgxPool.Close()

	ctx := context.Background()
	if err := menu.EnsureSchema(ctx, pgxPool); err != nil {
		return fmt.Errorf("ensure menu schema: %w", err)
	}
	if err := orders.EnsureSchema(ctx, pgxPool); err != nil {
		return fmt.Errorf("ensure orders schema: %w", err)
	}
	if err := payment.EnsureSchema(ctx, pgxPool); err != nil {
		return fmt.Errorf("ensure payment schema: %w", err)
	}
	if err := inventory.EnsureSchema(ctx, pgxPool); err != nil {
		return fmt.Errorf("ensure inventory schema: %w", err)
	}

	return nil
}
