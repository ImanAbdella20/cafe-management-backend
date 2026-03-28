package menu

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	Create(ctx context.Context, item *MenuItem) error
	GetAll(ctx context.Context) ([]MenuItem, error)
	GetAllWithActivePrice(ctx context.Context) ([]MenuItemWithPrice, error)
	GetByID(ctx context.Context, id int) (*MenuItem, error)
	Update(ctx context.Context, id int, req UpdateMenuItemRequest) error
	UpdateAvailability(ctx context.Context, id int, available bool) error
	Delete(ctx context.Context, id int) error
}

type repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, item *MenuItem) error {
	query := `
	INSERT INTO menu_items (category_id, name, description, image_url)
	VALUES ($1, $2, $3, $4)
	RETURNING id, is_available, created_at
	`

	return r.db.QueryRow(ctx, query,
		item.CategoryID,
		item.Name,
		item.Description,
		item.ImageURL,
	).Scan(&item.ID, &item.IsAvailable, &item.CreatedAt)
}

func (r *repository) GetAll(ctx context.Context) ([]MenuItem, error) {
	query := `
	SELECT id, category_id, name, description, image_url, is_available, created_at
	FROM menu_items
	ORDER BY id ASC
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []MenuItem
	for rows.Next() {
		var item MenuItem
		if err := rows.Scan(
			&item.ID,
			&item.CategoryID,
			&item.Name,
			&item.Description,
			&item.ImageURL,
			&item.IsAvailable,
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

func (r *repository) GetAllWithActivePrice(ctx context.Context) ([]MenuItemWithPrice, error) {
	query := `
	SELECT m.id, m.category_id, COALESCE(c.name, ''), m.name, m.description, m.image_url, m.is_available,
	       p.amount, p.currency
	FROM menu_items m
	LEFT JOIN categories c
		ON m.category_id = c.id
	LEFT JOIN prices p
		ON m.id = p.item_id AND p.is_active = true
	ORDER BY m.id ASC
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []MenuItemWithPrice
	for rows.Next() {
		var item MenuItemWithPrice
		var categoryID sql.NullInt32
		var price sql.NullFloat64
		var currency sql.NullString

		if err := rows.Scan(
			&item.ID,
			&categoryID,
			&item.CategoryName,
			&item.Name,
			&item.Description,
			&item.ImageURL,
			&item.IsAvailable,
			&price,
			&currency,
		); err != nil {
			return nil, err
		}

		if categoryID.Valid {
			value := int(categoryID.Int32)
			item.CategoryID = &value
		}

		if price.Valid {
			item.Price = price.Float64
		}
		if currency.Valid {
			item.Currency = currency.String
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (r *repository) GetByID(ctx context.Context, id int) (*MenuItem, error) {
	query := `
	SELECT id, category_id, name, description, image_url, is_available, created_at
	FROM menu_items
	WHERE id = $1
	`

	var item MenuItem
	err := r.db.QueryRow(ctx, query, id).Scan(
		&item.ID,
		&item.CategoryID,
		&item.Name,
		&item.Description,
		&item.ImageURL,
		&item.IsAvailable,
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

func (r *repository) Update(ctx context.Context, id int, req UpdateMenuItemRequest) error {
	query := `
	UPDATE menu_items
	SET name = $1, description = $2, image_url = $3
	WHERE id = $4
	`

	cmd, err := r.db.Exec(ctx, query,
		req.Name,
		req.Description,
		req.ImageURL,
		id,
	)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

func (r *repository) UpdateAvailability(ctx context.Context, id int, available bool) error {
	query := `UPDATE menu_items SET is_available = $1 WHERE id = $2`

	cmd, err := r.db.Exec(ctx, query, available, id)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

func (r *repository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM menu_items WHERE id = $1`

	cmd, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}
