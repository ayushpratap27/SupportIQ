package repositories

import (
	"github.com/ayush/supportiq/internal/models"
	"gorm.io/gorm"
)

// UserRepository encapsulates all database access for users.
type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// FindByID returns a user by primary key.
func (r *UserRepository) FindByID(id uint) (*models.User, error) {
	var user models.User
	if err := r.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// ListByRole returns all active users with the given role.
func (r *UserRepository) ListByRole(role models.Role) ([]models.User, error) {
	var users []models.User
	if err := r.db.Where("role = ? AND is_active = ?", role, true).Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}
