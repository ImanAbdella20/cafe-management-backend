package user

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	defaultAdminEmail    = "admin@cafe.com"
	defaultAdminPassword = "admin123"
)

func CreateAdminIfNotExists(db *gorm.DB) error {
	var existing User
	queryErr := db.Where("email = ?", defaultAdminEmail).First(&existing).Error
	if queryErr != nil && !errors.Is(queryErr, gorm.ErrRecordNotFound) {
		return queryErr
	}

	hashedPassword, hashErr := bcrypt.GenerateFromPassword([]byte(defaultAdminPassword), bcrypt.DefaultCost)
	if hashErr != nil {
		return hashErr
	}

	if queryErr == nil {
		updates := map[string]any{}

		if existing.Name == "" {
			updates["name"] = "Admin"
		}
		if existing.Role != "admin" {
			updates["role"] = "admin"
		}
		if !existing.IsActive {
			updates["is_active"] = true
		}

		if bcrypt.CompareHashAndPassword([]byte(existing.Password), []byte(defaultAdminPassword)) != nil {
			updates["password"] = string(hashedPassword)
		}

		if len(updates) == 0 {
			return nil
		}

		return db.Model(&User{}).Where("id = ?", existing.ID).Updates(updates).Error
	}

	admin := User{
		Name:     "Admin",
		Email:    defaultAdminEmail,
		Password: string(hashedPassword),
		Role:     "admin",
		IsActive: true,
	}

	return db.Create(&admin).Error
}
