package category

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

var ErrCategoryNameRequired = errors.New("category name required")
var ErrCategoryNotFound = errors.New("category not found")

type Service interface {
	CreateCategory(ctx context.Context, req CreateCategoryRequest) (*Category, error)
	GetCategories(ctx context.Context) ([]Category, error)
	UpdateCategory(ctx context.Context, id int, req UpdateCategoryRequest) error
	DeleteCategory(ctx context.Context, id int) error
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) CreateCategory(ctx context.Context, req CreateCategoryRequest) (*Category, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrCategoryNameRequired
	}

	category := &Category{
		Name:        name,
		Description: strings.TrimSpace(req.Description),
	}

	if err := s.repo.Create(ctx, category); err != nil {
		return nil, err
	}

	return category, nil
}

func (s *service) GetCategories(ctx context.Context) ([]Category, error) {
	return s.repo.GetAll(ctx)
}

func (s *service) UpdateCategory(ctx context.Context, id int, req UpdateCategoryRequest) error {
	if id <= 0 {
		return ErrCategoryNotFound
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return ErrCategoryNameRequired
	}

	err := s.repo.Update(ctx, &Category{
		ID:          id,
		Name:        name,
		Description: strings.TrimSpace(req.Description),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrCategoryNotFound
	}
	if err != nil {
		return err
	}

	return nil
}

func (s *service) DeleteCategory(ctx context.Context, id int) error {
	if id <= 0 {
		return ErrCategoryNotFound
	}

	err := s.repo.Delete(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrCategoryNotFound
	}
	if err != nil {
		return err
	}

	return nil
}
