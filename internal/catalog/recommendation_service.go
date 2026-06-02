package catalog

import (
	"context"
	"time"
)

// RecommendationService provides business logic for catalog book recommendations.
type RecommendationService struct {
	store RecommendationStore
	now   func() time.Time
}

// NewRecommendationService creates a new RecommendationService.
func NewRecommendationService(store RecommendationStore) *RecommendationService {
	return &RecommendationService{
		store: store,
		now:   time.Now,
	}
}

// Current returns the most recent published recommendation, or nil if none.
func (s *RecommendationService) Current() (*Recommendation, error) {
	return s.store.CurrentRecommendation(context.Background(), s.now())
}

// History returns a paginated list of published recommendations.
func (s *RecommendationService) History(page, pageSize int) ([]Recommendation, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.store.ListPublishedRecommendations(context.Background(), s.now(), pageSize, offset)
}
