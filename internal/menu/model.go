package menu

import "time"

type UploadedImage struct {
	Filename    string
	ContentType string
	Data        []byte
}

type MenuItem struct {
	ID          int       `json:"id"`
	CategoryID  *int      `json:"category_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	ImageURL    string    `json:"image_url"`
	IsAvailable bool      `json:"is_available"`
	CreatedAt   time.Time `json:"created_at"`
}

type CreateMenuItemRequest struct {
	CategoryID  int            `json:"category_id" binding:"required" form:"category_id"`
	Name        string         `json:"name" binding:"required" form:"name"`
	Description string         `json:"description" form:"description"`
	ImageURL    string         `json:"image_url" form:"image_url"`
	Image       *UploadedImage `json:"-" form:"-"`
}

type UpdateMenuItemRequest struct {
	Name        string         `json:"name" form:"name"`
	Description string         `json:"description" form:"description"`
	ImageURL    string         `json:"image_url" form:"image_url"`
	RemoveImage bool           `json:"remove_image" form:"remove_image"`
	Image       *UploadedImage `json:"-" form:"-"`
}

type CreateMenuItemResponse struct {
	Message string    `json:"message"`
	Item    *MenuItem `json:"item"`
}

type MenuItemWithPrice struct {
	ID           int     `json:"id"`
	CategoryID   *int    `json:"category_id"`
	CategoryName string  `json:"category_name"`
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	ImageURL     string  `json:"image_url"`
	IsAvailable  bool    `json:"is_available"`
	Price        float64 `json:"price"`
	Currency     string  `json:"currency"`
}
