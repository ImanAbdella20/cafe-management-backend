package inventory

import (
	"context"
	"errors"
	"strings"
	"time"

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

type OrderInventoryRequirement struct {
	ItemID   uuid.UUID
	BranchID uuid.UUID
	Quantity float64
}

type Repository interface {
	WithTx(ctx context.Context, fn func(txRepo Repository) error) error
	Create(ctx context.Context, item *InventoryItem) error
	List(ctx context.Context) ([]InventoryItem, error)
	GetByID(ctx context.Context, id uuid.UUID) (*InventoryItem, error)
	Update(ctx context.Context, item *InventoryItem) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListStocksByItem(ctx context.Context, itemID uuid.UUID) ([]InventoryStock, error)
	InsertMovement(ctx context.Context, movement *InventoryMovement) error
	GetStockForUpdate(ctx context.Context, itemID uuid.UUID, branchID uuid.UUID) (*InventoryStock, error)
	CreateStock(ctx context.Context, stock *InventoryStock) error
	UpdateStockQuantity(ctx context.Context, stockID uuid.UUID, quantity float64) (time.Time, error)
	CreatePurchaseRequest(ctx context.Context, request *InventoryPurchaseRequest) error
	ListPurchaseRequests(ctx context.Context, branchID *uuid.UUID, status string) ([]InventoryPurchaseRequest, error)
	GetPurchaseRequestByIDForUpdate(ctx context.Context, id uuid.UUID) (*InventoryPurchaseRequest, error)
	SetPurchaseApproved(ctx context.Context, id uuid.UUID, approvedBy uuid.UUID, approvedAt time.Time) error
	UpsertMenuItemIngredient(ctx context.Context, recipe MenuItemIngredient) error
	GetOrderRequirements(ctx context.Context, orderID uuid.UUID) ([]OrderInventoryRequirement, error)
}

type repository struct {
	db *pgxpool.Pool
	q  queryExecutor
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &repository{db: db, q: db}
}

func EnsureSchema(ctx context.Context, db *pgxpool.Pool) error {
	type schemaStatement struct {
		sql              string
		allowOwnerDenied bool
	}

	statements := []schemaStatement{
		{sql: `
		CREATE TABLE IF NOT EXISTS inventory_items (
			id UUID PRIMARY KEY,
			name TEXT NOT NULL,
			unit TEXT NOT NULL DEFAULT 'unit',
			low_stock_threshold DOUBLE PRECISION NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
		`},
		{sql: `
		CREATE TABLE IF NOT EXISTS inventory_stock (
			id UUID PRIMARY KEY,
			item_id UUID NOT NULL REFERENCES inventory_items(id) ON DELETE CASCADE,
			branch_id UUID NOT NULL,
			quantity DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (quantity >= 0),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE (item_id, branch_id)
		)
		`},
		{sql: `
		CREATE TABLE IF NOT EXISTS inventory_movements (
			id UUID PRIMARY KEY,
			item_id UUID NOT NULL REFERENCES inventory_items(id) ON DELETE CASCADE,
			branch_id UUID NOT NULL,
			quantity DOUBLE PRECISION NOT NULL,
			movement_type TEXT NOT NULL,
			reference_id UUID NOT NULL,
			created_by UUID NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
		`},
		{sql: `
		CREATE TABLE IF NOT EXISTS inventory_purchase_requests (
			id UUID PRIMARY KEY,
			item_id UUID NOT NULL REFERENCES inventory_items(id) ON DELETE CASCADE,
			branch_id UUID NOT NULL,
			quantity DOUBLE PRECISION NOT NULL CHECK (quantity > 0),
			status TEXT NOT NULL,
			requested_by UUID NOT NULL,
			approved_by UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000000',
			requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			approved_at TIMESTAMPTZ NULL
		)
		`},
		{sql: `
		CREATE TABLE IF NOT EXISTS menu_item_ingredients (
			menu_item_id INTEGER NOT NULL,
			inventory_item_id UUID NOT NULL REFERENCES inventory_items(id) ON DELETE CASCADE,
			quantity_per_order DOUBLE PRECISION NOT NULL CHECK (quantity_per_order > 0),
			PRIMARY KEY (menu_item_id, inventory_item_id)
		)
		`},
		{sql: `ALTER TABLE orders ADD COLUMN IF NOT EXISTS branch_id UUID`, allowOwnerDenied: true},
		{sql: `CREATE INDEX IF NOT EXISTS idx_inventory_items_name ON inventory_items(name)`, allowOwnerDenied: true},
		{sql: `CREATE INDEX IF NOT EXISTS idx_inventory_stock_item_branch ON inventory_stock(item_id, branch_id)`, allowOwnerDenied: true},
		{sql: `CREATE INDEX IF NOT EXISTS idx_inventory_movements_item_branch_created_at ON inventory_movements(item_id, branch_id, created_at DESC)`, allowOwnerDenied: true},
		{sql: `CREATE INDEX IF NOT EXISTS idx_inventory_purchase_requests_branch_status ON inventory_purchase_requests(branch_id, status, requested_at DESC)`, allowOwnerDenied: true},
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

func (r *repository) Create(ctx context.Context, item *InventoryItem) error {
	query := `
	INSERT INTO inventory_items (id, name, unit, low_stock_threshold)
	VALUES ($1, $2, $3, $4)
	RETURNING created_at
	`

	return r.q.QueryRow(ctx, query,
		item.ID,
		item.Name,
		item.Unit,
		item.LowStockThreshold,
	).Scan(&item.CreatedAt)
}

func (r *repository) List(ctx context.Context) ([]InventoryItem, error) {
	query := `
	SELECT id, name, unit, low_stock_threshold, created_at
	FROM inventory_items
	ORDER BY name ASC, created_at ASC
	`

	rows, err := r.q.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]InventoryItem, 0)
	for rows.Next() {
		var item InventoryItem
		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.Unit,
			&item.LowStockThreshold,
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

func (r *repository) GetByID(ctx context.Context, id uuid.UUID) (*InventoryItem, error) {
	query := `
	SELECT id, name, unit, low_stock_threshold, created_at
	FROM inventory_items
	WHERE id = $1
	`

	var item InventoryItem
	err := r.q.QueryRow(ctx, query, id).Scan(
		&item.ID,
		&item.Name,
		&item.Unit,
		&item.LowStockThreshold,
		&item.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &item, nil
}

func (r *repository) Update(ctx context.Context, item *InventoryItem) error {
	query := `
	UPDATE inventory_items
	SET name = $1, unit = $2, low_stock_threshold = $3
	WHERE id = $4
	`

	cmd, err := r.q.Exec(ctx, query,
		item.Name,
		item.Unit,
		item.LowStockThreshold,
		item.ID,
	)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

func (r *repository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM inventory_items WHERE id = $1`

	cmd, err := r.q.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

func (r *repository) ListStocksByItem(ctx context.Context, itemID uuid.UUID) ([]InventoryStock, error) {
	query := `
	SELECT id, item_id, branch_id, quantity, updated_at
	FROM inventory_stock
	WHERE item_id = $1
	ORDER BY updated_at DESC, branch_id ASC
	`

	rows, err := r.q.Query(ctx, query, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stocks := make([]InventoryStock, 0)
	for rows.Next() {
		var stock InventoryStock
		if err := rows.Scan(&stock.ID, &stock.ItemID, &stock.BranchID, &stock.Quantity, &stock.UpdatedAt); err != nil {
			return nil, err
		}
		stocks = append(stocks, stock)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return stocks, nil
}

func (r *repository) InsertMovement(ctx context.Context, movement *InventoryMovement) error {
	query := `
	INSERT INTO inventory_movements (id, item_id, branch_id, quantity, movement_type, reference_id, created_by)
	VALUES ($1, $2, $3, $4, $5, $6, $7)
	RETURNING created_at
	`

	return r.q.QueryRow(
		ctx,
		query,
		movement.ID,
		movement.ItemID,
		movement.BranchID,
		movement.Quantity,
		movement.MovementType,
		movement.ReferenceID,
		movement.CreatedBy,
	).Scan(&movement.CreatedAt)
}

func (r *repository) GetStockForUpdate(ctx context.Context, itemID uuid.UUID, branchID uuid.UUID) (*InventoryStock, error) {
	query := `
	SELECT id, item_id, branch_id, quantity, updated_at
	FROM inventory_stock
	WHERE item_id = $1 AND branch_id = $2
	FOR UPDATE
	`

	var stock InventoryStock
	err := r.q.QueryRow(ctx, query, itemID, branchID).Scan(
		&stock.ID,
		&stock.ItemID,
		&stock.BranchID,
		&stock.Quantity,
		&stock.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &stock, nil
}

func (r *repository) CreateStock(ctx context.Context, stock *InventoryStock) error {
	query := `
	INSERT INTO inventory_stock (id, item_id, branch_id, quantity)
	VALUES ($1, $2, $3, $4)
	RETURNING updated_at
	`

	return r.q.QueryRow(ctx, query, stock.ID, stock.ItemID, stock.BranchID, stock.Quantity).Scan(&stock.UpdatedAt)
}

func (r *repository) UpdateStockQuantity(ctx context.Context, stockID uuid.UUID, quantity float64) (time.Time, error) {
	query := `
	UPDATE inventory_stock
	SET quantity = $1, updated_at = NOW()
	WHERE id = $2
	RETURNING updated_at
	`

	var updatedAt time.Time
	err := r.q.QueryRow(ctx, query, quantity, stockID).Scan(&updatedAt)
	if err != nil {
		return time.Time{}, err
	}

	return updatedAt, nil
}

func (r *repository) CreatePurchaseRequest(ctx context.Context, request *InventoryPurchaseRequest) error {
	query := `
	INSERT INTO inventory_purchase_requests (id, item_id, branch_id, quantity, status, requested_by)
	VALUES ($1, $2, $3, $4, $5, $6)
	RETURNING requested_at
	`

	return r.q.QueryRow(
		ctx,
		query,
		request.ID,
		request.ItemID,
		request.BranchID,
		request.Quantity,
		request.Status,
		request.RequestedBy,
	).Scan(&request.RequestedAt)
}

func (r *repository) ListPurchaseRequests(ctx context.Context, branchID *uuid.UUID, status string) ([]InventoryPurchaseRequest, error) {
	query := `
	SELECT id, item_id, branch_id, quantity, status, requested_by, approved_by, requested_at, approved_at
	FROM inventory_purchase_requests
	WHERE ($1::uuid IS NULL OR branch_id = $1)
	  AND ($2 = '' OR status = $2)
	ORDER BY requested_at DESC
	`

	var branchParam any
	if branchID != nil {
		branchParam = *branchID
	}

	rows, err := r.q.Query(ctx, query, branchParam, strings.TrimSpace(strings.ToLower(status)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	requests := make([]InventoryPurchaseRequest, 0)
	for rows.Next() {
		var request InventoryPurchaseRequest
		if err := rows.Scan(
			&request.ID,
			&request.ItemID,
			&request.BranchID,
			&request.Quantity,
			&request.Status,
			&request.RequestedBy,
			&request.ApprovedBy,
			&request.RequestedAt,
			&request.ApprovedAt,
		); err != nil {
			return nil, err
		}
		requests = append(requests, request)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return requests, nil
}

func (r *repository) GetPurchaseRequestByIDForUpdate(ctx context.Context, id uuid.UUID) (*InventoryPurchaseRequest, error) {
	query := `
	SELECT id, item_id, branch_id, quantity, status, requested_by, approved_by, requested_at, approved_at
	FROM inventory_purchase_requests
	WHERE id = $1
	FOR UPDATE
	`

	var request InventoryPurchaseRequest
	err := r.q.QueryRow(ctx, query, id).Scan(
		&request.ID,
		&request.ItemID,
		&request.BranchID,
		&request.Quantity,
		&request.Status,
		&request.RequestedBy,
		&request.ApprovedBy,
		&request.RequestedAt,
		&request.ApprovedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &request, nil
}

func (r *repository) SetPurchaseApproved(ctx context.Context, id uuid.UUID, approvedBy uuid.UUID, approvedAt time.Time) error {
	query := `
	UPDATE inventory_purchase_requests
	SET status = $1, approved_by = $2, approved_at = $3
	WHERE id = $4
	`

	cmd, err := r.q.Exec(ctx, query, PurchaseStatusApproved, approvedBy, approvedAt, id)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

func (r *repository) UpsertMenuItemIngredient(ctx context.Context, recipe MenuItemIngredient) error {
	query := `
	INSERT INTO menu_item_ingredients (menu_item_id, inventory_item_id, quantity_per_order)
	VALUES ($1, $2, $3)
	ON CONFLICT (menu_item_id, inventory_item_id)
	DO UPDATE SET quantity_per_order = EXCLUDED.quantity_per_order
	`

	_, err := r.q.Exec(ctx, query, recipe.MenuItemID, recipe.InventoryItemID, recipe.QuantityPerOrder)
	return err
}

func (r *repository) GetOrderRequirements(ctx context.Context, orderID uuid.UUID) ([]OrderInventoryRequirement, error) {
	query := `
	SELECT mii.inventory_item_id, o.branch_id, SUM(oi.quantity::DOUBLE PRECISION * mii.quantity_per_order) AS required_qty
	FROM order_items oi
	INNER JOIN orders o ON o.id = oi.order_id
	INNER JOIN menu_item_ingredients mii ON mii.menu_item_id = oi.menu_item_id
	WHERE oi.order_id = $1
	GROUP BY mii.inventory_item_id, o.branch_id
	`

	rows, err := r.q.Query(ctx, query, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	requirements := make([]OrderInventoryRequirement, 0)
	for rows.Next() {
		var requirement OrderInventoryRequirement
		if err := rows.Scan(&requirement.ItemID, &requirement.BranchID, &requirement.Quantity); err != nil {
			return nil, err
		}
		requirements = append(requirements, requirement)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return requirements, nil
}

func isInventoryConstraintViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23514"
	}

	errMessage := strings.ToLower(err.Error())
	return strings.Contains(errMessage, "check constraint") || strings.Contains(errMessage, "quantity")
}
