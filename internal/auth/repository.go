package auth

import (
	"context"
	"errors"
	"strings"

	"backend/internal/user"

	"gorm.io/gorm"
)

type Repository interface {
	Create(ctx context.Context, user *user.User) error
	GetByEmail(ctx context.Context, email string) (*user.User, error)
}

type GormRepository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *GormRepository {
	return &GormRepository{db: db}
}

func (r *GormRepository) Create(ctx context.Context, user *user.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *GormRepository) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	var foundUser user.User
	normalizedEmail := strings.TrimSpace(strings.ToLower(email))
	err := r.db.WithContext(ctx).Where("email = ?", normalizedEmail).Take(&foundUser).Error
	if err == nil {
		return &foundUser, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return nil, err
}
