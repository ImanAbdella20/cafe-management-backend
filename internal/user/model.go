package user

import "time"

type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:120" json:"name"`
	Email     string    `gorm:"uniqueIndex;size:255;not null" json:"email"`
	Password  string    `gorm:"size:255;not null" json:"-"`
	Role      string    `gorm:"size:50;default:staff" json:"role"`
	BranchID  string    `gorm:"size:36;default:''" json:"branch_id"`
	IsActive  bool      `gorm:"default:true" json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Shift struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	ShiftDate string    `gorm:"size:20;not null" json:"shift_date"`
	StartTime string    `gorm:"size:20;not null" json:"start_time"`
	EndTime   string    `gorm:"size:20;not null" json:"end_time"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
