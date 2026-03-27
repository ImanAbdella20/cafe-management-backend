package orders

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
		CREATE TABLE IF NOT EXISTS order_number_sequences (
			branch_code TEXT NOT NULL,
			sequence_date TEXT NOT NULL,
			last_value INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (branch_code, sequence_date)
		)
		`},
		{sql: `
		CREATE TABLE IF NOT EXISTS orders (
			id UUID PRIMARY KEY,
			order_number TEXT NOT NULL UNIQUE,
			cashier_id UUID NOT NULL,
			order_type TEXT NOT NULL,
			status TEXT NOT NULL,
			subtotal DOUBLE PRECISION NOT NULL DEFAULT 0,
			tax_amount DOUBLE PRECISION NOT NULL DEFAULT 0,
			discount_amount DOUBLE PRECISION NOT NULL DEFAULT 0,
			total_amount DOUBLE PRECISION NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
		`},
		{sql: `
		CREATE TABLE IF NOT EXISTS order_items (
			id UUID PRIMARY KEY,
			order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
			menu_item_id INTEGER NOT NULL,
			item_name TEXT NOT NULL,
			quantity INTEGER NOT NULL,
			unit_price DOUBLE PRECISION NOT NULL,
			total_price DOUBLE PRECISION NOT NULL,
			notes TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
		`},
		{sql: `
		CREATE TABLE IF NOT EXISTS order_status_history (
			id UUID PRIMARY KEY,
			order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
			status TEXT NOT NULL,
			changed_by UUID NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
		`},
		{sql: `ALTER TABLE orders ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`, allowOwnerDenied: true},
		{sql: `ALTER TABLE orders ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`, allowOwnerDenied: true},
		{sql: `ALTER TABLE orders ADD COLUMN IF NOT EXISTS subtotal DOUBLE PRECISION NOT NULL DEFAULT 0`, allowOwnerDenied: true},
		{sql: `ALTER TABLE orders ADD COLUMN IF NOT EXISTS tax_amount DOUBLE PRECISION NOT NULL DEFAULT 0`, allowOwnerDenied: true},
		{sql: `ALTER TABLE orders ADD COLUMN IF NOT EXISTS discount_amount DOUBLE PRECISION NOT NULL DEFAULT 0`, allowOwnerDenied: true},
		{sql: `ALTER TABLE orders ADD COLUMN IF NOT EXISTS total_amount DOUBLE PRECISION NOT NULL DEFAULT 0`, allowOwnerDenied: true},
		{sql: `ALTER TABLE orders ADD COLUMN IF NOT EXISTS branch_id UUID`, allowOwnerDenied: true},
		{sql: `ALTER TABLE order_items ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`, allowOwnerDenied: true},
		{sql: `ALTER TABLE order_items ADD COLUMN IF NOT EXISTS notes TEXT NOT NULL DEFAULT ''`, allowOwnerDenied: true},
		{sql: `ALTER TABLE order_items ADD COLUMN IF NOT EXISTS item_name TEXT NOT NULL DEFAULT ''`, allowOwnerDenied: true},
		{sql: `ALTER TABLE order_items ALTER COLUMN menu_item_id DROP NOT NULL`, allowOwnerDenied: true},
		{sql: `
		ALTER TABLE order_items
		ALTER COLUMN menu_item_id TYPE INTEGER
		USING (
			CASE
				WHEN menu_item_id::text ~ '^[0-9]+$' THEN menu_item_id::text::INTEGER
				ELSE NULL
			END
		)
		`, allowOwnerDenied: true},
		{sql: `DELETE FROM order_items WHERE menu_item_id IS NULL`, allowOwnerDenied: true},
		{sql: `ALTER TABLE order_items ALTER COLUMN menu_item_id SET NOT NULL`, allowOwnerDenied: true},
		{sql: `ALTER TABLE order_status_history ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`, allowOwnerDenied: true},
		{sql: `CREATE INDEX IF NOT EXISTS idx_orders_created_at ON orders(created_at DESC)`, allowOwnerDenied: true},
		{sql: `CREATE INDEX IF NOT EXISTS idx_order_items_order_id ON order_items(order_id)`, allowOwnerDenied: true},
		{sql: `CREATE INDEX IF NOT EXISTS idx_order_status_history_order_id ON order_status_history(order_id)`, allowOwnerDenied: true},
		{sql: `
		CREATE OR REPLACE FUNCTION refresh_order_totals(p_order_id UUID)
		RETURNS VOID AS $$
		DECLARE
			v_subtotal DOUBLE PRECISION;
			v_discount DOUBLE PRECISION;
			v_taxable DOUBLE PRECISION;
		BEGIN
			IF p_order_id IS NULL THEN
				RETURN;
			END IF;

			SELECT COALESCE(SUM(total_price), 0)
			INTO v_subtotal
			FROM order_items
			WHERE order_id = p_order_id;

			v_discount := 0;
			v_taxable := GREATEST(v_subtotal - v_discount, 0);

			UPDATE orders
			SET subtotal = v_subtotal,
				discount_amount = v_discount,
				tax_amount = v_taxable * 0.15,
				total_amount = v_taxable + (v_taxable * 0.15),
				updated_at = NOW()
			WHERE id = p_order_id;
		END;
		$$ LANGUAGE plpgsql
		`, allowOwnerDenied: true},
		{sql: `
		CREATE OR REPLACE FUNCTION trg_refresh_order_totals_from_items()
		RETURNS TRIGGER AS $$
		BEGIN
			IF TG_OP = 'DELETE' THEN
				PERFORM refresh_order_totals(OLD.order_id);
				RETURN OLD;
			END IF;

			PERFORM refresh_order_totals(NEW.order_id);

			IF TG_OP = 'UPDATE' AND OLD.order_id IS DISTINCT FROM NEW.order_id THEN
				PERFORM refresh_order_totals(OLD.order_id);
			END IF;

			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql
		`, allowOwnerDenied: true},
		{sql: `DROP TRIGGER IF EXISTS trg_order_items_refresh_totals ON order_items`, allowOwnerDenied: true},
		{sql: `
		CREATE TRIGGER trg_order_items_refresh_totals
		AFTER INSERT OR UPDATE OR DELETE ON order_items
		FOR EACH ROW
		EXECUTE FUNCTION trg_refresh_order_totals_from_items()
		`, allowOwnerDenied: true},
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
