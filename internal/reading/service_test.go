package reading

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"
)

// fakeProgressStore emulates the transactional merge of MySQLProgressStore so
// service behavior (validation, clock clamp, book lookup) is testable without a
// database.
type fakeProgressStore struct {
	rows       map[string]Progress
	operations []ProgressOperation
	lastLimit  int
	lastOffset int
}

func newFakeProgressStore() *fakeProgressStore {
	return &fakeProgressStore{rows: map[string]Progress{}}
}

func (f *fakeProgressStore) GetProgress(_ context.Context, _ uint64, bookKey string) (Progress, error) {
	progress, ok := f.rows[bookKey]
	if !ok {
		return Progress{}, ErrNotFound
	}
	return progress, nil
}

func (f *fakeProgressStore) ListProgress(_ context.Context, _ uint64, limit, offset int) ([]Progress, int, error) {
	f.lastLimit = limit
	f.lastOffset = offset
	items := make([]Progress, 0, len(f.rows))
	for _, progress := range f.rows {
		items = append(items, progress)
	}
	return items, len(f.rows), nil
}

func (f *fakeProgressStore) UpsertProgress(_ context.Context, _ uint64, incoming Progress, operation ProgressOperation) (UpsertOutcome, error) {
	if operation.OperationID != "" {
		for _, seen := range f.operations {
			if seen.OperationID == operation.OperationID && seen.DeviceID == operation.DeviceID {
				return UpsertOutcome{Progress: f.rows[incoming.BookKey], Duplicate: true}, nil
			}
		}
		f.operations = append(f.operations, operation)
	}

	current, ok := f.rows[incoming.BookKey]
	if ok && !Preferred(current, incoming) {
		return UpsertOutcome{Progress: current}, nil
	}
	incoming.UpdatedAt = incoming.RecordedAt
	f.rows[incoming.BookKey] = incoming
	return UpsertOutcome{Progress: incoming, Applied: true}, nil
}

func anyBook(_ context.Context, _ string) (bool, error) { return true, nil }

func fixedClock(at time.Time) func() time.Time {
	return func() time.Time { return at }
}

func TestPreferredKeepsTheMostRecentProgress(t *testing.T) {
	base := Progress{BookKey: "catalog:book", DeviceID: "device-a", RecordedAt: time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)}

	tests := []struct {
		name     string
		incoming Progress
		want     bool
	}{
		{"newer timestamp wins", Progress{DeviceID: "device-b", RecordedAt: base.RecordedAt.Add(time.Second)}, true},
		{"older timestamp loses", Progress{DeviceID: "device-z", RecordedAt: base.RecordedAt.Add(-time.Second)}, false},
		{"equal timestamp larger device id wins", Progress{DeviceID: "device-b", RecordedAt: base.RecordedAt}, true},
		{"equal timestamp smaller device id loses", Progress{DeviceID: "device-a", RecordedAt: base.RecordedAt}, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := Preferred(base, test.incoming); got != test.want {
				t.Fatalf("Preferred() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestClampRecordedAtCollapsesFutureClientClocks(t *testing.T) {
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		at       time.Time
		want     time.Time
		wantSkew bool
	}{
		{"recent past is kept", now.Add(-time.Hour), now.Add(-time.Hour), false},
		{"within allowance is kept", now.Add(ClockSkewAllowance), now.Add(ClockSkewAllowance), false},
		{"future beyond allowance collapses to now", now.Add(time.Hour), now, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, skewed := ClampRecordedAt(test.at, now)
			if !got.Equal(test.want) || skewed != test.wantSkew {
				t.Fatalf("ClampRecordedAt() = %v, %v, want %v, %v", got, skewed, test.want, test.wantSkew)
			}
		})
	}
}

func TestUpsertNormalizesProgress(t *testing.T) {
	now := time.Date(2026, 9, 14, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name  string
		input ProgressInput
		want  Progress
	}{
		{
			name: "location with rounded percentage defaults type and timestamp",
			input: ProgressInput{
				BookKey:         "  catalog:remote_walden  ",
				ProgressPercent: floatPtr(7.777),
				PositionCFI:     ` epub:{"spineIndex":2} `,
				ChapterTitle:    " Chapter Two ",
				ContentVersion:  " n2-s2-image-variants-v1 ",
				DeviceID:        "device-a",
			},
			want: Progress{
				BookKey:         "catalog:remote_walden",
				ProgressType:    ProgressTypeLocation,
				ProgressValue:   "7.78",
				ProgressPercent: floatPtr(7.78),
				ChapterTitle:    "Chapter Two",
				PositionCFI:     `epub:{"spineIndex":2}`,
				ContentVersion:  "n2-s2-image-variants-v1",
				DeviceID:        "device-a",
				RecordedAt:      now,
			},
		},
		{
			name: "explicit chapter progress keeps its value and timestamp",
			input: ProgressInput{
				BookKey:       "catalog:remote_walden",
				ProgressType:  "CHAPTER",
				ProgressValue: "12",
				RecordedAt:    timePtr(time.Date(2026, 9, 13, 8, 0, 0, 0, time.UTC)),
			},
			want: Progress{
				BookKey:       "catalog:remote_walden",
				ProgressType:  ProgressTypeChapter,
				ProgressValue: "12",
				RecordedAt:    time.Date(2026, 9, 13, 8, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "percentage without a scalar value derives progress_value",
			input: ProgressInput{
				BookKey:         "catalog:remote_walden",
				ProgressPercent: floatPtr(42.5),
			},
			want: Progress{
				BookKey:         "catalog:remote_walden",
				ProgressType:    ProgressTypeLocation,
				ProgressValue:   "42.50",
				ProgressPercent: floatPtr(42.5),
				RecordedAt:      now,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := newFakeProgressStore()
			service := NewProgressService(store, anyBook, nil)
			service.now = fixedClock(now)

			result, err := service.Upsert(context.Background(), 7, test.input)
			if err != nil {
				t.Fatalf("Upsert() error = %v", err)
			}
			if !result.Applied {
				t.Fatalf("Upsert() applied = false, want true")
			}
			assertProgress(t, result.Progress, test.want)
			assertProgress(t, store.rows[test.want.BookKey], test.want)
		})
	}
}

func TestUpsertRejectsInvalidProgress(t *testing.T) {
	tests := []struct {
		name  string
		input ProgressInput
		want  error
	}{
		{"empty book key", ProgressInput{BookKey: "", ProgressValue: "1"}, ErrInvalidBookKey},
		{"blank book key", ProgressInput{BookKey: "   ", ProgressValue: "1"}, ErrInvalidBookKey},
		{"book key over 191 characters", ProgressInput{BookKey: "catalog:" + strings.Repeat("书", 192), ProgressValue: "1"}, ErrInvalidBookKey},
		{"locally imported book", ProgressInput{BookKey: "1780149222080", ProgressValue: "1"}, ErrUnsupportedBookKey},
		{"unknown progress type", ProgressInput{BookKey: "catalog:b", ProgressType: "scroll", ProgressValue: "1"}, ErrInvalidProgressType},
		{"progress value over 64 characters", ProgressInput{BookKey: "catalog:b", ProgressValue: strings.Repeat("a", 65)}, ErrInvalidProgressValue},
		{"missing progress value and percent", ProgressInput{BookKey: "catalog:b"}, ErrInvalidProgressValue},
		{"negative percent", ProgressInput{BookKey: "catalog:b", ProgressPercent: floatPtr(-0.01)}, ErrInvalidPercent},
		{"percent above 100", ProgressInput{BookKey: "catalog:b", ProgressPercent: floatPtr(100.01)}, ErrInvalidPercent},
		{"nan percent", ProgressInput{BookKey: "catalog:b", ProgressPercent: floatPtr(math.NaN())}, ErrInvalidPercent},
		{"infinite percent", ProgressInput{BookKey: "catalog:b", ProgressPercent: floatPtr(math.Inf(1))}, ErrInvalidPercent},
		{"chapter title over 255 characters", ProgressInput{BookKey: "catalog:b", ProgressValue: "1", ChapterTitle: strings.Repeat("t", 256)}, ErrInvalidChapterTitle},
		{"position cfi over 1024 characters", ProgressInput{BookKey: "catalog:b", ProgressValue: "1", PositionCFI: strings.Repeat("p", 1025)}, ErrInvalidPositionCFI},
		{"content version over 191 characters", ProgressInput{BookKey: "catalog:b", ProgressValue: "1", ContentVersion: strings.Repeat("v", 192)}, ErrInvalidContentVersion},
		{"device id over 128 characters", ProgressInput{BookKey: "catalog:b", ProgressValue: "1", DeviceID: strings.Repeat("d", 129)}, ErrInvalidDeviceID},
		{"operation id over 128 characters", ProgressInput{BookKey: "catalog:b", ProgressValue: "1", OperationID: strings.Repeat("o", 129)}, ErrInvalidOperationID},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := newFakeProgressStore()
			service := NewProgressService(store, anyBook, nil)

			if _, err := service.Upsert(context.Background(), 7, test.input); !errors.Is(err, test.want) {
				t.Fatalf("Upsert() error = %v, want %v", err, test.want)
			}
			if len(store.rows) != 0 {
				t.Fatalf("invalid input stored %d rows", len(store.rows))
			}
		})
	}
}

func TestUpsertRejectsBooksOutsideTheCatalog(t *testing.T) {
	store := newFakeProgressStore()
	service := NewProgressService(store, func(_ context.Context, key string) (bool, error) {
		return key == "known-book", nil
	}, nil)

	if _, err := service.Upsert(context.Background(), 7, ProgressInput{BookKey: "catalog:ghost", ProgressValue: "1"}); !errors.Is(err, ErrUnknownBook) {
		t.Fatalf("Upsert(unknown catalog book) error = %v, want ErrUnknownBook", err)
	}
	if len(store.rows) != 0 {
		t.Fatalf("unknown book stored %d rows", len(store.rows))
	}

	if _, err := service.Upsert(context.Background(), 7, ProgressInput{BookKey: "catalog:known-book", ProgressValue: "1"}); err != nil {
		t.Fatalf("Upsert(known catalog book) error = %v", err)
	}
}

func TestUpsertKeepsTheNewerProgressAcrossDevices(t *testing.T) {
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	store := newFakeProgressStore()
	service := NewProgressService(store, anyBook, nil)
	service.now = fixedClock(now)

	phone, err := service.Upsert(context.Background(), 7, ProgressInput{
		BookKey:         "catalog:book",
		ProgressValue:   "40",
		ProgressPercent: floatPtr(40),
		DeviceID:        "phone",
		RecordedAt:      timePtr(now),
	})
	if err != nil || !phone.Applied {
		t.Fatalf("phone upsert = %#v, %v", phone, err)
	}

	// An offline tablet uploads an older position: it must not win, and the
	// winning row is echoed back so the tablet can reconcile locally.
	tablet, err := service.Upsert(context.Background(), 7, ProgressInput{
		BookKey:         "catalog:book",
		ProgressValue:   "70",
		ProgressPercent: floatPtr(70),
		DeviceID:        "tablet",
		RecordedAt:      timePtr(now.Add(-time.Hour)),
	})
	if err != nil {
		t.Fatalf("tablet upsert error = %v", err)
	}
	if tablet.Applied {
		t.Fatalf("stale tablet upload applied = true, want false")
	}
	if tablet.Progress.ProgressValue != "40" || *tablet.Progress.ProgressPercent != 40 {
		t.Fatalf("authoritative row = %#v, want the phone's 40%%", tablet.Progress)
	}
	if len(store.rows) != 1 {
		t.Fatalf("stored rows = %d, want 1", len(store.rows))
	}

	// The tablet reads on and uploads a newer position later.
	tabletLater, err := service.Upsert(context.Background(), 7, ProgressInput{
		BookKey:         "catalog:book",
		ProgressValue:   "85",
		ProgressPercent: floatPtr(85),
		DeviceID:        "tablet",
		RecordedAt:      timePtr(now.Add(time.Minute)),
	})
	if err != nil || !tabletLater.Applied {
		t.Fatalf("tablet later upsert = %#v, %v", tabletLater, err)
	}
	if tabletLater.Progress.ProgressValue != "85" {
		t.Fatalf("stored value = %q, want 85", tabletLater.Progress.ProgressValue)
	}
}

func TestUpsertReplaysOperationIdempotently(t *testing.T) {
	store := newFakeProgressStore()
	service := NewProgressService(store, anyBook, nil)

	replay := ProgressInput{
		BookKey:       "catalog:book",
		ProgressValue: "10",
		DeviceID:      "phone",
		OperationID:   "op-1",
	}
	first, err := service.Upsert(context.Background(), 7, replay)
	if err != nil || !first.Applied || first.Duplicate {
		t.Fatalf("first upsert = %#v, %v", first, err)
	}

	second, err := service.Upsert(context.Background(), 7, replay)
	if err != nil {
		t.Fatalf("replay error = %v", err)
	}
	if second.Applied || !second.Duplicate {
		t.Fatalf("replay = %#v, want applied=false duplicate=true", second)
	}
	if second.Progress.ProgressValue != "10" {
		t.Fatalf("replay echoed %q, want the stored 10", second.Progress.ProgressValue)
	}
	if len(store.operations) != 1 {
		t.Fatalf("recorded operations = %d, want 1", len(store.operations))
	}
}

func TestUpsertReportsClockSkew(t *testing.T) {
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	store := newFakeProgressStore()
	service := NewProgressService(store, anyBook, nil)
	service.now = fixedClock(now)

	result, err := service.Upsert(context.Background(), 7, ProgressInput{
		BookKey:       "catalog:book",
		ProgressValue: "10",
		DeviceID:      "phone",
		RecordedAt:    timePtr(now.Add(48 * time.Hour)),
	})
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}
	if !result.ClockSkew {
		t.Fatalf("clock_skew = false, want true")
	}
	if !result.Progress.RecordedAt.Equal(now) {
		t.Fatalf("recorded_at = %v, want server time %v", result.Progress.RecordedAt, now)
	}
}

func TestGetTrimsKeyAndReportsMissingProgress(t *testing.T) {
	store := newFakeProgressStore()
	store.rows["catalog:remote_walden"] = Progress{BookKey: "catalog:remote_walden", ProgressValue: "1"}
	service := NewProgressService(store, anyBook, nil)

	found, err := service.Get(context.Background(), 7, " catalog:remote_walden ")
	if err != nil || found.BookKey != "catalog:remote_walden" {
		t.Fatalf("Get() = %#v, %v", found, err)
	}

	if _, err := service.Get(context.Background(), 7, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(missing) error = %v, want ErrNotFound", err)
	}
	if _, err := service.Get(context.Background(), 7, "  "); !errors.Is(err, ErrInvalidBookKey) {
		t.Fatalf("Get(blank) error = %v, want ErrInvalidBookKey", err)
	}
}

func TestListClampsPagination(t *testing.T) {
	tests := []struct {
		name           string
		page, pageSize int
		wantLimit      int
		wantOffset     int
	}{
		{"defaults", 0, 0, DefaultProgressPageSize, 0},
		{"page size cap", 1, 500, MaxProgressPageSize, 0},
		{"later page", 3, 20, 20, 40},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := newFakeProgressStore()
			service := NewProgressService(store, anyBook, nil)

			if _, _, err := service.List(context.Background(), 7, test.page, test.pageSize); err != nil {
				t.Fatalf("List() error = %v", err)
			}
			if store.lastLimit != test.wantLimit || store.lastOffset != test.wantOffset {
				t.Fatalf("store got limit=%d offset=%d, want limit=%d offset=%d",
					store.lastLimit, store.lastOffset, test.wantLimit, test.wantOffset)
			}
		})
	}
}

func TestCatalogBookKeyStripsOnlyTheCatalogPrefix(t *testing.T) {
	tests := []struct{ in, want string }{
		{"catalog:remote_walden", "remote_walden"},
		{"catalog:", ""},
		{"1780149222080", ""},
		{"sha1:abc", ""},
	}
	for _, test := range tests {
		if got := CatalogBookKey(test.in); got != test.want {
			t.Fatalf("CatalogBookKey(%q) = %q, want %q", test.in, got, test.want)
		}
	}
}

func assertProgress(t *testing.T, got, want Progress) {
	t.Helper()
	if got.BookKey != want.BookKey ||
		got.ProgressType != want.ProgressType ||
		got.ProgressValue != want.ProgressValue ||
		got.ChapterTitle != want.ChapterTitle ||
		got.PositionCFI != want.PositionCFI ||
		got.ContentVersion != want.ContentVersion ||
		got.DeviceID != want.DeviceID {
		t.Fatalf("progress = %#v, want %#v", got, want)
	}
	if !got.RecordedAt.Equal(want.RecordedAt) {
		t.Fatalf("recorded_at = %v, want %v", got.RecordedAt, want.RecordedAt)
	}
	switch {
	case got.ProgressPercent == nil && want.ProgressPercent == nil:
	case got.ProgressPercent == nil || want.ProgressPercent == nil:
		t.Fatalf("progress_percent = %v, want %v", got.ProgressPercent, want.ProgressPercent)
	case *got.ProgressPercent != *want.ProgressPercent:
		t.Fatalf("progress_percent = %v, want %v", *got.ProgressPercent, *want.ProgressPercent)
	}
}

func floatPtr(value float64) *float64 {
	return &value
}

func timePtr(value time.Time) *time.Time {
	return &value
}
