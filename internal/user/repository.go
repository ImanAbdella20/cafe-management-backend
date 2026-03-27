package user

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

type Repository interface {
	Create(ctx context.Context, u *User) error
	GetByEmail(ctx context.Context, email string) (*User, error)
	ListUsers(ctx context.Context, page, limit int, role string) ([]User, int64, error)
	GetByID(ctx context.Context, id uint) (*User, error)
	UpdateStatus(ctx context.Context, id uint, status bool) error
	UpdateRole(ctx context.Context, id uint, role string) error
	AssignShift(ctx context.Context, s *Shift) error
	ListShiftsByUserID(ctx context.Context, userID uint) ([]Shift, error)
	ListShiftsByDate(ctx context.Context, shiftDate string) ([]Shift, error)
}

type GormRepository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *GormRepository {
	return &GormRepository{db: db}
}

func (r *GormRepository) Create(ctx context.Context, u *User) error {
	return r.db.WithContext(ctx).Create(u).Error
}

func (r *GormRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&u).Error
	if err == nil {
		return &u, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return nil, err
}

func (r *GormRepository) ListUsers(ctx context.Context, page, limit int, role string) ([]User, int64, error) {
	query := r.db.WithContext(ctx).Model(&User{})
	if role != "" {
		query = query.Where("role = ?", role)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}

	var users []User
	err := query.Order("id desc").Offset((page - 1) * limit).Limit(limit).Find(&users).Error
	if err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

func (r *GormRepository) GetByID(ctx context.Context, id uint) (*User, error) {
	var u User
	err := r.db.WithContext(ctx).First(&u, id).Error
	if err == nil {
		return &u, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return nil, err
}

func (r *GormRepository) UpdateStatus(ctx context.Context, id uint, status bool) error {
	result := r.db.WithContext(ctx).Model(&User{}).Where("id = ?", id).Update("is_active", status)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *GormRepository) UpdateRole(ctx context.Context, id uint, role string) error {
	result := r.db.WithContext(ctx).Model(&User{}).Where("id = ?", id).Update("role", role)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *GormRepository) AssignShift(ctx context.Context, s *Shift) error {
	return r.db.WithContext(ctx).Create(s).Error
}

func (r *GormRepository) ListShiftsByUserID(ctx context.Context, userID uint) ([]Shift, error) {
	var shifts []Shift
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("shift_date desc, start_time asc").Find(&shifts).Error
	if err != nil {
		return nil, err
	}
	return shifts, nil
}

func (r *GormRepository) ListShiftsByDate(ctx context.Context, shiftDate string) ([]Shift, error) {
	var shifts []Shift
	err := r.db.WithContext(ctx).Where("shift_date = ?", shiftDate).Order("start_time asc").Find(&shifts).Error
	if err != nil {
		return nil, err
	}
	return shifts, nil
}
