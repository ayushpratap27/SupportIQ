package repositories

import (
	"github.com/ayush/supportiq/internal/models"
	"gorm.io/gorm"
)

// EmailAccountRepository encapsulates all database access for email_accounts.
type EmailAccountRepository struct {
	db *gorm.DB
}

func NewEmailAccountRepository(db *gorm.DB) *EmailAccountRepository {
	return &EmailAccountRepository{db: db}
}

func (r *EmailAccountRepository) Create(a *models.EmailAccount) error {
	return r.db.Create(a).Error
}

func (r *EmailAccountRepository) FindByID(id uint) (*models.EmailAccount, error) {
	var a models.EmailAccount
	if err := r.db.First(&a, id).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *EmailAccountRepository) FindAll() ([]models.EmailAccount, error) {
	var accounts []models.EmailAccount
	if err := r.db.Order("created_at desc").Find(&accounts).Error; err != nil {
		return nil, err
	}
	return accounts, nil
}

func (r *EmailAccountRepository) FindActive() ([]models.EmailAccount, error) {
	var accounts []models.EmailAccount
	if err := r.db.Where("is_active = true").Find(&accounts).Error; err != nil {
		return nil, err
	}
	return accounts, nil
}

func (r *EmailAccountRepository) Update(a *models.EmailAccount) error {
	return r.db.Save(a).Error
}

func (r *EmailAccountRepository) Delete(id uint) error {
	return r.db.Delete(&models.EmailAccount{}, id).Error
}

func (r *EmailAccountRepository) Count() (int64, error) {
	var n int64
	return n, r.db.Model(&models.EmailAccount{}).Count(&n).Error
}

func (r *EmailAccountRepository) CountActive() (int64, error) {
	var n int64
	return n, r.db.Model(&models.EmailAccount{}).Where("is_active = true").Count(&n).Error
}
