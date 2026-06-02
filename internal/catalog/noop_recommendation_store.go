package catalog

import (
	"context"
	"time"
)

// noopRecommendationStore is a no-op implementation of RecommendationStore
// used when the database is not available.
type noopRecommendationStore struct{}

// NewNoopRecommendationStore creates a new noopRecommendationStore.
func NewNoopRecommendationStore() RecommendationStore {
	return &noopRecommendationStore{}
}

func (s *noopRecommendationStore) CurrentRecommendation(ctx context.Context, now time.Time) (*Recommendation, error) {
	return nil, nil
}

func (s *noopRecommendationStore) ListPublishedRecommendations(ctx context.Context, now time.Time, limit, offset int) ([]Recommendation, int, error) {
	return []Recommendation{}, 0, nil
}
