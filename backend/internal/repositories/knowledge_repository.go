package repositories

import (
	"context"

	"github.com/ayush/supportiq/internal/models"
	"gorm.io/gorm"
)

// KnowledgeRepository handles all database access for the knowledge_bases table.
type KnowledgeRepository struct {
	db *gorm.DB
}

func NewKnowledgeRepository(db *gorm.DB) *KnowledgeRepository {
	return &KnowledgeRepository{db: db}
}

// Create inserts a new knowledge base document.
func (r *KnowledgeRepository) Create(doc *models.KnowledgeBase) error {
	return r.db.Create(doc).Error
}

// FindByID loads a single document by primary key.
func (r *KnowledgeRepository) FindByID(id uint) (*models.KnowledgeBase, error) {
	var doc models.KnowledgeBase
	if err := r.db.First(&doc, id).Error; err != nil {
		return nil, err
	}
	return &doc, nil
}

// Update persists changes to an existing document.
func (r *KnowledgeRepository) Update(doc *models.KnowledgeBase) error {
	return r.db.Save(doc).Error
}

// Delete hard-deletes a document by primary key.
func (r *KnowledgeRepository) Delete(id uint) error {
	return r.db.Delete(&models.KnowledgeBase{}, id).Error
}

// List returns a paginated, optionally filtered list of documents.
func (r *KnowledgeRepository) List(search, category string, activeOnly bool, page, limit int) ([]models.KnowledgeBase, int64, error) {
	q := r.db.Model(&models.KnowledgeBase{})

	if search != "" {
		pattern := "%" + search + "%"
		q = q.Where("title ILIKE ? OR content ILIKE ?", pattern, pattern)
	}
	if category != "" {
		q = q.Where("category = ?", category)
	}
	if activeOnly {
		q = q.Where("is_active = true")
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	var docs []models.KnowledgeBase
	err := q.Order("created_at DESC").Offset(offset).Limit(limit).Find(&docs).Error
	return docs, total, err
}

// Search performs a case-insensitive keyword search over active documents.
// Used by the RAG retrieval layer; designed to be replaced by vector search later.
func (r *KnowledgeRepository) Search(_ context.Context, query string, limit int) ([]models.KnowledgeBase, error) {
	if limit <= 0 {
		limit = 5
	}
	pattern := "%" + query + "%"
	var docs []models.KnowledgeBase
	err := r.db.
		Where("is_active = true AND (title ILIKE ? OR content ILIKE ?)", pattern, pattern).
		Order("updated_at DESC").
		Limit(limit).
		Find(&docs).Error
	return docs, err
}
