package orders

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type queryExecutor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Repository interface {
	CreateOrder(ctx context.Context, order *Order) error
	GetOrderByID(ctx context.Context, id uuid.UUID) (*Order, error)
	ListOrders(ctx context.Context) ([]Order, error)
	NextOrderSequence(ctx context.Context, branchCode string, sequenceDate string) (int, error)
	UpdateOrder(ctx context.Context, order *Order) error
	AddOrderItem(ctx context.Context, item *OrderItem) error
	GetOrderSubtotal(ctx context.Context, orderID uuid.UUID) (float64, error)
	ListOrderItems(ctx context.Context, orderID uuid.UUID) ([]OrderItem, error)
	UpdateStatus(ctx context.Context, orderID uuid.UUID, status string) error
	AddStatusHistory(ctx context.Context, orderID, changedBy uuid.UUID, status string) error
	GetCurrentMenuPrice(ctx context.Context, menuItemID int) (float64, error)
	DeleteOrder(ctx context.Context, orderID uuid.UUID) error
	WithTx(ctx context.Context, fn func(txRepo Repository) error) error
}

type repository struct {
	db *pgxpool.Pool
	q  queryExecutor
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &repository{db: db, q: db}
}

func (r *repository) WithTx(ctx context.Context, fn func(txRepo Repository) error) error {
	if tx, ok := r.q.(pgx.Tx); ok && tx != nil {
		return fn(r)
	}

	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}

	txRepo := &repository{db: r.db, q: tx}
	if err := fn(txRepo); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}

	return tx.Commit(ctx)
}

func (r *repository) CreateOrder(ctx context.Context, order *Order) error {
	query := `
	INSERT INTO orders (id, order_number, cashier_id, branch_id, order_type, status, subtotal, tax_amount, discount_amount, total_amount)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	RETURNING created_at, updated_at
	`

	return r.q.QueryRow(ctx, query,
		order.ID,
		order.OrderNumber,
		order.CashierID,
		order.BranchID,
		order.OrderType,
		order.Status,
		order.Subtotal,
		order.TaxAmount,
		order.DiscountAmount,
		order.TotalAmount,
	).Scan(&order.CreatedAt, &order.UpdatedAt)
}

func (r *repository) NextOrderSequence(ctx context.Context, branchCode string, sequenceDate string) (int, error) {
	query := `
	INSERT INTO order_number_sequences (branch_code, sequence_date, last_value)
	VALUES ($1, $2, 1)
	ON CONFLICT (branch_code, sequence_date)
	DO UPDATE SET last_value = order_number_sequences.last_value + 1
	RETURNING last_value
	`

	var seq int
	if err := r.q.QueryRow(ctx, query, branchCode, sequenceDate).Scan(&seq); err != nil {
		return 0, fmt.Errorf("next order sequence: %w", err)
	}

	return seq, nil
}

func (r *repository) GetOrderByID(ctx context.Context, id uuid.UUID) (*Order, error) {
	query := `
	SELECT id, order_number, cashier_id, branch_id, order_type, status, subtotal, tax_amount, discount_amount, total_amount, created_at, updated_at
	FROM orders
	WHERE id = $1
	`

	var order Order
	err := r.q.QueryRow(ctx, query, id).Scan(
		&order.ID,
		&order.OrderNumber,
		&order.CashierID,
		&order.BranchID,
		&order.OrderType,
		&order.Status,
		&order.Subtotal,
		&order.TaxAmount,
		&order.DiscountAmount,
		&order.TotalAmount,
		&order.CreatedAt,
		&order.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &order, nil
}

func (r *repository) ListOrders(ctx context.Context) ([]Order, error) {
	query := `
	SELECT id, order_number, cashier_id, branch_id, order_type, status, subtotal, tax_amount, discount_amount, total_amount, created_at, updated_at
	FROM orders
	ORDER BY created_at DESC
	`

	rows, err := r.q.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := make([]Order, 0)
	for rows.Next() {
		var order Order
		if err := rows.Scan(
			&order.ID,
			&order.OrderNumber,
			&order.CashierID,
			&order.BranchID,
			&order.OrderType,
			&order.Status,
			&order.Subtotal,
			&order.TaxAmount,
			&order.DiscountAmount,
			&order.TotalAmount,
			&order.CreatedAt,
			&order.UpdatedAt,
		); err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return orders, nil
}

func (r *repository) UpdateOrder(ctx context.Context, order *Order) error {
	query := `
	UPDATE orders
	SET subtotal = $1, tax_amount = $2, discount_amount = $3, total_amount = $4, updated_at = NOW()
	WHERE id = $5
	`

	cmd, err := r.q.Exec(ctx, query,
		order.Subtotal,
		order.TaxAmount,
		order.DiscountAmount,
		order.TotalAmount,
		order.ID,
	)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

func (r *repository) AddOrderItem(ctx context.Context, item *OrderItem) error {
	if item == nil {
		return errors.New("order item is required")
	}

	if item.ID == uuid.Nil {
		item.ID = uuid.New()
	}

	if item.OrderID == uuid.Nil {
		return ErrOrderNotFound
	}

	query := `
	WITH selected_item AS (
		SELECT name
		FROM menu_items
		WHERE id = $3
	)
	INSERT INTO order_items (id, order_id, menu_item_id, item_name, quantity, unit_price, total_price, notes)
	SELECT $1, $2, $3, selected_item.name, $4, $5, $6, $7
	FROM selected_item
	RETURNING created_at
	`

	if err := r.q.QueryRow(ctx, query,
		item.ID,
		item.OrderID,
		item.MenuItemID,
		item.Quantity,
		item.UnitPrice,
		item.TotalPrice,
		item.Notes,
	).Scan(&item.CreatedAt); err != nil {
		return err
	}

	return r.refreshOrderTotals(ctx, item.OrderID)
}

func (r *repository) refreshOrderTotals(ctx context.Context, orderID uuid.UUID) error {
	query := `
	UPDATE orders o
	SET
		subtotal = totals.subtotal,
		discount_amount = 0,
		tax_amount = GREATEST(totals.subtotal, 0) * 0.15,
		total_amount = GREATEST(totals.subtotal, 0) + (GREATEST(totals.subtotal, 0) * 0.15),
		updated_at = NOW()
	FROM (
		SELECT COALESCE(SUM(total_price), 0) AS subtotal
		FROM order_items
		WHERE order_id = $1
	) totals
	WHERE o.id = $1
	`

	cmd, err := r.q.Exec(ctx, query, orderID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

func (r *repository) ListOrderItems(ctx context.Context, orderID uuid.UUID) ([]OrderItem, error) {
	query := `
	SELECT id, order_id, menu_item_id, quantity, unit_price, total_price, notes, created_at
	FROM order_items
	WHERE order_id = $1
	ORDER BY created_at ASC
	`

	rows, err := r.q.Query(ctx, query, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]OrderItem, 0)
	for rows.Next() {
		var item OrderItem
		if err := rows.Scan(
			&item.ID,
			&item.OrderID,
			&item.MenuItemID,
			&item.Quantity,
			&item.UnitPrice,
			&item.TotalPrice,
			&item.Notes,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (r *repository) GetOrderSubtotal(ctx context.Context, orderID uuid.UUID) (float64, error) {
	query := `
	SELECT COALESCE(SUM(total_price), 0)
	FROM order_items
	WHERE order_id = $1
	`

	var subtotal float64
	if err := r.q.QueryRow(ctx, query, orderID).Scan(&subtotal); err != nil {
		return 0, err
	}

	return subtotal, nil
}

func (r *repository) UpdateStatus(ctx context.Context, orderID uuid.UUID, status string) error {
	query := `UPDATE orders SET status = $1, updated_at = NOW() WHERE id = $2`

	cmd, err := r.q.Exec(ctx, query, status, orderID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

func (r *repository) AddStatusHistory(ctx context.Context, orderID, changedBy uuid.UUID, status string) error {
	query := `
	INSERT INTO order_status_history (id, order_id, status, changed_by)
	VALUES ($1, $2, $3, $4)
	`

	_, err := r.q.Exec(ctx, query, uuid.New(), orderID, status, changedBy)
	return err
}

func (r *repository) GetCurrentMenuPrice(ctx context.Context, menuItemID int) (float64, error) {
	query := `
	SELECT p.amount
	FROM prices p
	WHERE p.item_id = $1 AND p.is_active = true
	ORDER BY p.effective_from DESC
	LIMIT 1
	`

	var amount sql.NullFloat64
	err := r.q.QueryRow(ctx, query, menuItemID).Scan(&amount)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}
	if !amount.Valid {
		return 0, nil
	}

	return amount.Float64, nil
}

func (r *repository) DeleteOrder(ctx context.Context, orderID uuid.UUID) error {
	query := `DELETE FROM orders WHERE id = $1`

	cmd, err := r.q.Exec(ctx, query, orderID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}
