package catalog

import (
	"context"
	"time"
)

// RecommendationStore defines persistence operations for catalog book recommendations.
type RecommendationStore interface {
	// CurrentRecommendation returns the most recent published recommendation, or nil if none.
	CurrentRecommendation(ctx context.Context, now time.Time) (*Recommendation, error)
	// ListPublishedRecommendations returns a paginated list of published recommendations.
	ListPublishedRecommendations(ctx context.Context, now time.Time, limit, offset int) ([]Recommendation, int, error)
}
