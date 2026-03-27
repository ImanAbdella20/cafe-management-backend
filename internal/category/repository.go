package category

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	Create(ctx context.Context, c *Category) error
	GetAll(ctx context.Context) ([]Category, error)
	GetByID(ctx context.Context, id int) (*Category, error)
	Exists(ctx context.Context, id int) (bool, error)
	Update(ctx context.Context, c *Category) error
	Delete(ctx context.Context, id int) error
}

type repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, c *Category) error {
	query := `
	INSERT INTO categories (name, description)
	VALUES ($1, $2)
	RETURNING id, is_active, created_at
	`

	return r.db.QueryRow(ctx, query, c.Name, c.Description).
		Scan(&c.ID, &c.IsActive, &c.CreatedAt)
}

func (r *repository) GetAll(ctx context.Context) ([]Category, error) {
	query := `SELECT id, name, description, is_active, created_at FROM categories ORDER BY id ASC`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.IsActive, &c.CreatedAt); err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return categories, nil
}

func (r *repository) GetByID(ctx context.Context, id int) (*Category, error) {
	query := `SELECT id, name, description, is_active, created_at FROM categories WHERE id = $1`

	var c Category
	err := r.db.QueryRow(ctx, query, id).Scan(&c.ID, &c.Name, &c.Description, &c.IsActive, &c.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &c, nil
}

func (r *repository) Exists(ctx context.Context, id int) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM categories WHERE id = $1)`

	var exists bool
	err := r.db.QueryRow(ctx, query, id).Scan(&exists)
	return exists, err
}

func (r *repository) Update(ctx context.Context, c *Category) error {
	query := `
	UPDATE categories
	SET name = $1, description = $2
	WHERE id = $3
	`

	cmd, err := r.db.Exec(ctx, query, c.Name, c.Description, c.ID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

func (r *repository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM categories WHERE id = $1`

	cmd, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}
