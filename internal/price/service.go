package price

import (
	"context"
	"errors"
	"strings"
)

type Service interface {
	CreatePrice(ctx context.Context, itemID int, req CreatePriceRequest) error
	GetByItem(ctx context.Context, itemID int) ([]Price, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) CreatePrice(ctx context.Context, itemID int, req CreatePriceRequest) error {
	if itemID <= 0 {
		return errors.New("invalid item id")
	}
	if req.Amount <= 0 {
		return errors.New("amount must be greater than zero")
	}

	currency := strings.ToUpper(strings.TrimSpace(req.Currency))
	if currency == "" {
		currency = "BR"
	}

	return s.repo.CreateWithTransaction(ctx, itemID, req.Amount, currency)
}

func (s *service) GetByItem(ctx context.Context, itemID int) ([]Price, error) {
	if itemID <= 0 {
		return nil, errors.New("invalid item id")
	}

	return s.repo.GetByItem(ctx, itemID)
}
