package menu

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func EnsureSchema(ctx context.Context, db *pgxpool.Pool) error {
	type schemaStatement struct {
		sql              string
		allowOwnerDenied bool
	}

	statements := []schemaStatement{
		{sql: `
		CREATE TABLE IF NOT EXISTS categories (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			is_active BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
		`},
		{sql: `
		CREATE TABLE IF NOT EXISTS menu_items (
			id SERIAL PRIMARY KEY,
			category_id INTEGER NULL REFERENCES categories(id) ON DELETE SET NULL,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			image_url TEXT NOT NULL DEFAULT '',
			is_available BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
		`},
		{sql: `
		CREATE TABLE IF NOT EXISTS prices (
			id SERIAL PRIMARY KEY,
			item_id INTEGER NOT NULL REFERENCES menu_items(id) ON DELETE CASCADE,
			amount DOUBLE PRECISION NOT NULL CHECK (amount >= 0),
			currency TEXT NOT NULL DEFAULT 'ETB',
			effective_from TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			is_active BOOLEAN NOT NULL DEFAULT true
		)
		`},
		{sql: `CREATE INDEX IF NOT EXISTS idx_menu_items_category_id ON menu_items(category_id)`, allowOwnerDenied: true},
		{sql: `CREATE INDEX IF NOT EXISTS idx_menu_items_name ON menu_items(name)`, allowOwnerDenied: true},
		{sql: `CREATE INDEX IF NOT EXISTS idx_prices_item_id ON prices(item_id)`, allowOwnerDenied: true},
		{sql: `CREATE INDEX IF NOT EXISTS idx_prices_item_active ON prices(item_id, is_active)`, allowOwnerDenied: true},
	}

	for _, statement := range statements {
		if _, err := db.Exec(ctx, statement.sql); err != nil {
			if statement.allowOwnerDenied && isOwnerPrivilegeError(err) {
				continue
			}
			return err
		}
	}

	return nil
}

func isOwnerPrivilegeError(err error) bool {
	var pgErr *pgconn.PgError
	if ok := errors.As(err, &pgErr); ok {
		return pgErr.Code == "42501"
	}

	errMessage := strings.ToLower(err.Error())
	return strings.Contains(errMessage, "must be owner") || strings.Contains(errMessage, "permission denied")
}
