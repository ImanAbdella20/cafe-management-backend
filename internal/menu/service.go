package menu

import (
	"backend/internal/category"
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

var ErrMenuItemNameRequired = errors.New("item name required")
var ErrMenuCategoryRequired = errors.New("category id required")
var ErrMenuCategoryNotFound = errors.New("category not found")
var ErrMenuItemNotFound = errors.New("menu item not found")

type Service interface {
	CreateItem(ctx context.Context, req CreateMenuItemRequest) (*MenuItem, error)
	GetItems(ctx context.Context) ([]MenuItemWithPrice, error)
	UpdateItem(ctx context.Context, id int, req UpdateMenuItemRequest) error
	DeleteItem(ctx context.Context, id int) error
}

type service struct {
	repo         Repository
	categoryRepo category.Repository
}

func NewService(repo Repository, categoryRepo category.Repository) Service {
	return &service{
		repo:         repo,
		categoryRepo: categoryRepo,
	}
}

func (s *service) CreateItem(ctx context.Context, req CreateMenuItemRequest) (*MenuItem, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrMenuItemNameRequired
	}
	if req.CategoryID <= 0 {
		return nil, ErrMenuCategoryRequired
	}

	exists, err := s.categoryRepo.Exists(ctx, req.CategoryID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrMenuCategoryNotFound
	}

	imageURL := strings.TrimSpace(req.ImageURL)
	if req.Image != nil {
		uploadedImageURL, err := saveUploadedImage(req.Image)
		if err != nil {
			return nil, err
		}
		imageURL = uploadedImageURL
	}

	item := &MenuItem{
		CategoryID:  &req.CategoryID,
		Name:        name,
		Description: strings.TrimSpace(req.Description),
		ImageURL:    imageURL,
	}

	err = s.repo.Create(ctx, item)
	if err != nil {
		if imageURL != "" {
			_ = deleteManagedImage(imageURL)
		}
		return nil, err
	}

	return item, nil
}

func (s *service) GetItems(ctx context.Context) ([]MenuItemWithPrice, error) {
	return s.repo.GetAllWithActivePrice(ctx)
}

func (s *service) UpdateItem(ctx context.Context, id int, req UpdateMenuItemRequest) error {
	if id <= 0 {
		return ErrMenuItemNotFound
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return ErrMenuItemNameRequired
	}

	existingItem, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if existingItem == nil {
		return ErrMenuItemNotFound
	}

	finalImageURL := strings.TrimSpace(existingItem.ImageURL)
	if imageURL := strings.TrimSpace(req.ImageURL); imageURL != "" {
		finalImageURL = imageURL
	}
	if req.RemoveImage {
		finalImageURL = ""
	}

	uploadedImageURL := ""
	if req.Image != nil {
		uploadedImageURL, err = saveUploadedImage(req.Image)
		if err != nil {
			return err
		}
		finalImageURL = uploadedImageURL
	}

	err = s.repo.Update(ctx, id, UpdateMenuItemRequest{
		Name:        name,
		Description: strings.TrimSpace(req.Description),
		ImageURL:    finalImageURL,
	})
	if err != nil && uploadedImageURL != "" {
		_ = deleteManagedImage(uploadedImageURL)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrMenuItemNotFound
	}
	if err != nil {
		return err
	}

	if uploadedImageURL != "" {
		_ = deleteManagedImage(existingItem.ImageURL)
	}
	if req.RemoveImage && uploadedImageURL == "" {
		_ = deleteManagedImage(existingItem.ImageURL)
	}

	return nil
}

func (s *service) DeleteItem(ctx context.Context, id int) error {
	if id <= 0 {
		return ErrMenuItemNotFound
	}

	existingItem, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if existingItem == nil {
		return ErrMenuItemNotFound
	}

	err = s.repo.Delete(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrMenuItemNotFound
	}
	if err != nil {
		return err
	}

	_ = deleteManagedImage(existingItem.ImageURL)

	return nil
}
