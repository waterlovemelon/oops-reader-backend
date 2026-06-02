package catalog

import (
	"context"
	"testing"
	"time"
)

type fakeRecommendationStore struct {
	currentCalled  bool
	listCalled     bool
	currentResult  *Recommendation
	listResult     []Recommendation
	listTotal      int
	lastLimit      int
	lastOffset     int
}

func (s *fakeRecommendationStore) CurrentRecommendation(ctx context.Context, now time.Time) (*Recommendation, error) {
	s.currentCalled = true
	return s.currentResult, nil
}

func (s *fakeRecommendationStore) ListPublishedRecommendations(ctx context.Context, now time.Time, limit, offset int) ([]Recommendation, int, error) {
	s.listCalled = true
	s.lastLimit = limit
	s.lastOffset = offset
	return s.listResult, s.listTotal, nil
}

func TestRecommendationServiceCurrentUsesNow(t *testing.T) {
	fixed := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	store := &fakeRecommendationStore{
		currentResult: &Recommendation{
			ID:      1,
			BookKey: "test-book",
			Comment: "Great read",
		},
	}
	svc := &RecommendationService{store: store, now: func() time.Time { return fixed }}

	rec, err := svc.Current()
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}
	if !store.currentCalled {
		t.Fatal("expected CurrentRecommendation to be called")
	}
	if rec == nil || rec.BookKey != "test-book" {
		t.Fatalf("rec = %v, want book_key test-book", rec)
	}
}

func TestRecommendationServiceHistoryNormalizesPagination(t *testing.T) {
	store := &fakeRecommendationStore{
		listResult: []Recommendation{{ID: 1}},
		listTotal:  1,
	}
	svc := &RecommendationService{store: store, now: time.Now}

	// page=0, pageSize=0 should be normalized to page=1, pageSize=20
	_, _, err := svc.History(0, 0)
	if err != nil {
		t.Fatalf("History() error = %v", err)
	}
	if !store.listCalled {
		t.Fatal("expected ListPublishedRecommendations to be called")
	}
	if store.lastLimit != 20 {
		t.Fatalf("limit = %d, want 20", store.lastLimit)
	}
	if store.lastOffset != 0 {
		t.Fatalf("offset = %d, want 0", store.lastOffset)
	}

	// page=2, pageSize=10 should produce offset=10
	_, _, err = svc.History(2, 10)
	if err != nil {
		t.Fatalf("History() error = %v", err)
	}
	if store.lastLimit != 10 {
		t.Fatalf("limit = %d, want 10", store.lastLimit)
	}
	if store.lastOffset != 10 {
		t.Fatalf("offset = %d, want 10", store.lastOffset)
	}
}
