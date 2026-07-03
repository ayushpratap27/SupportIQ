package models

import "time"

type Role string

const (
	RoleAdmin        Role = "Admin"
	RoleSupportAgent Role = "SupportAgent"
)

// User represents a registered user in the system.
// PasswordHash is never serialised to JSON (json:"-").
type User struct {
	ID           uint      `gorm:"primarykey;autoIncrement"                json:"id"`
	Name         string    `gorm:"type:varchar(100);not null"              json:"name"`
	Email        string    `gorm:"type:varchar(255);uniqueIndex;not null"  json:"email"`
	PasswordHash string    `gorm:"type:varchar(255);not null"              json:"-"`
	Role         Role      `gorm:"type:varchar(20);not null;default:'SupportAgent'" json:"role"`
	IsActive     bool      `gorm:"not null;default:true"                   json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
