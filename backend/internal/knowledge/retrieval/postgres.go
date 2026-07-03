package retrieval

import (
	"context"

	"github.com/ayush/supportiq/internal/models"
	"github.com/ayush/supportiq/internal/repositories"
)

// PostgresRetriever retrieves relevant knowledge documents using SQL keyword search.
// It implements the Retriever interface backed by the PostgreSQL knowledge_bases table.
type PostgresRetriever struct {
	repo *repositories.KnowledgeRepository
}

// NewPostgresRetriever creates a PostgresRetriever backed by the given repository.
func NewPostgresRetriever(repo *repositories.KnowledgeRepository) *PostgresRetriever {
	return &PostgresRetriever{repo: repo}
}

// Retrieve performs a case-insensitive keyword search across title and content
// and returns up to limit active documents ordered by recency.
func (r *PostgresRetriever) Retrieve(ctx context.Context, query string, limit int) ([]models.KnowledgeBase, error) {
	return r.repo.Search(ctx, query, limit)
}
