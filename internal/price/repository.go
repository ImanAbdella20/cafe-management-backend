package price

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	CreateWithTransaction(ctx context.Context, itemID int, amount float64, currency string) error
	GetByItem(ctx context.Context, itemID int) ([]Price, error)
}

type repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) Repository {
	return &repository{db: db}
}

func (r *repository) CreateWithTransaction(ctx context.Context, itemID int, amount float64, currency string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx,
		`UPDATE prices SET is_active = false WHERE item_id = $1`,
		itemID,
	)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO prices (item_id, amount, currency) VALUES ($1, $2, $3)`,
		itemID, amount, currency,
	)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *repository) GetByItem(ctx context.Context, itemID int) ([]Price, error) {
	query := `
	SELECT id, item_id, amount, currency, effective_from, is_active
	FROM prices
	WHERE item_id = $1
	ORDER BY effective_from DESC, id DESC
	`

	rows, err := r.db.Query(ctx, query, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prices []Price
	for rows.Next() {
		var price Price
		if err := rows.Scan(
			&price.ID,
			&price.ItemID,
			&price.Amount,
			&price.Currency,
			&price.EffectiveFrom,
			&price.IsActive,
		); err != nil {
			return nil, err
		}
		prices = append(prices, price)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return prices, nil
}
