package retrieval

import (
	"context"

	"github.com/ayush/supportiq/internal/models"
)

// Retriever defines the contract for knowledge document retrieval.
//
// The current implementation uses PostgreSQL full-text keyword search.
// This interface allows the retrieval layer to be swapped for a vector
// database (e.g. pgvector, Pinecone, Weaviate) without any changes to
// the services that depend on it.
type Retriever interface {
	Retrieve(ctx context.Context, query string, limit int) ([]models.KnowledgeBase, error)
}
