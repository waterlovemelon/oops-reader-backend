package reading

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// UpsertResult is the validated outcome of one progress upload.
type UpsertResult struct {
	// Progress is the authoritative stored row. Applied is false when an older
	// upload lost the merge, in which case Progress is the winning row and the
	// client should write it back locally.
	Progress  Progress
	Applied   bool
	Duplicate bool
	ClockSkew bool
}

// ProgressService validates client progress updates and stores them.
type ProgressService struct {
	store        ProgressStore
	lookup       BookLookup
	recordDevice DeviceRecorder
	now          func() time.Time
}

// NewProgressService creates a ProgressService backed by store. lookup resolves
// whether a catalog book_key exists; recordDevice registers the reporting device
// so the account's device list stays current (it may be nil in tests).
func NewProgressService(store ProgressStore, lookup BookLookup, recordDevice DeviceRecorder) *ProgressService {
	return &ProgressService{
		store:        store,
		lookup:       lookup,
		recordDevice: recordDevice,
		now:          time.Now,
	}
}

// Upsert merges one book's progress into the account's stored position.
func (s *ProgressService) Upsert(ctx context.Context, userID uint64, input ProgressInput) (UpsertResult, error) {
	progress, operationID, err := s.normalize(input)
	if err != nil {
		return UpsertResult{}, err
	}

	catalogBookKey := CatalogBookKey(progress.BookKey)
	if catalogBookKey == "" {
		return UpsertResult{}, ErrUnsupportedBookKey
	}
	exists, err := s.lookup(ctx, catalogBookKey)
	if err != nil {
		return UpsertResult{}, fmt.Errorf("lookup catalog book: %w", err)
	}
	if !exists {
		return UpsertResult{}, ErrUnknownBook
	}

	// 上报的设备要单独登记:进度行的 updated_by_device_id 指向设备清单,
	// 后续限制设备数量也以这张表为准。
	if s.recordDevice != nil && progress.DeviceID != "" {
		if err := s.recordDevice(ctx, userID, progress.DeviceID); err != nil {
			return UpsertResult{}, fmt.Errorf("record device: %w", err)
		}
	}

	recordedAt, clockSkew := ClampRecordedAt(progress.RecordedAt, s.now())
	progress.RecordedAt = recordedAt

	payload, err := json.Marshal(progress)
	if err != nil {
		return UpsertResult{}, fmt.Errorf("encode progress payload: %w", err)
	}

	outcome, err := s.store.UpsertProgress(ctx, userID, progress, ProgressOperation{
		OperationID:      operationID,
		DeviceID:         progress.DeviceID,
		Payload:          payload,
		ClientOccurredAt: recordedAt,
	})
	if err != nil {
		return UpsertResult{}, err
	}

	return UpsertResult{
		Progress:  outcome.Progress,
		Applied:   outcome.Applied,
		Duplicate: outcome.Duplicate,
		ClockSkew: clockSkew,
	}, nil
}

// Get returns the stored progress of one book.
func (s *ProgressService) Get(ctx context.Context, userID uint64, bookKey string) (Progress, error) {
	bookKey = strings.TrimSpace(bookKey)
	if bookKey == "" || utf8.RuneCountInString(bookKey) > MaxBookKeyLength {
		return Progress{}, ErrInvalidBookKey
	}
	return s.store.GetProgress(ctx, userID, bookKey)
}

// List returns one page of a user's progress ordered by most recently updated.
func (s *ProgressService) List(ctx context.Context, userID uint64, page, pageSize int) ([]Progress, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = DefaultProgressPageSize
	}
	if pageSize > MaxProgressPageSize {
		pageSize = MaxProgressPageSize
	}
	return s.store.ListProgress(ctx, userID, pageSize, (page-1)*pageSize)
}

func (s *ProgressService) normalize(input ProgressInput) (Progress, string, error) {
	bookKey := strings.TrimSpace(input.BookKey)
	if bookKey == "" || utf8.RuneCountInString(bookKey) > MaxBookKeyLength {
		return Progress{}, "", ErrInvalidBookKey
	}

	progressType := strings.ToLower(strings.TrimSpace(input.ProgressType))
	if progressType == "" {
		progressType = ProgressTypeLocation
	}
	switch progressType {
	case ProgressTypePage, ProgressTypeChapter, ProgressTypePercent, ProgressTypeLocation:
	default:
		return Progress{}, "", fmt.Errorf("%w: %q", ErrInvalidProgressType, input.ProgressType)
	}

	var percent *float64
	if input.ProgressPercent != nil {
		if math.IsNaN(*input.ProgressPercent) || *input.ProgressPercent < 0 || *input.ProgressPercent > 100 {
			return Progress{}, "", ErrInvalidPercent
		}
		// progress_percent is DECIMAL(5,2); round before storing so the echoed
		// value matches the persisted one.
		rounded := math.Round(*input.ProgressPercent*100) / 100
		percent = &rounded
	}

	progressValue := strings.TrimSpace(input.ProgressValue)
	if progressValue == "" {
		if percent == nil {
			return Progress{}, "", ErrInvalidProgressValue
		}
		// A client that only reports a location and a percentage still gets a
		// populated scalar column.
		progressValue = strconv.FormatFloat(*percent, 'f', 2, 64)
	}
	if utf8.RuneCountInString(progressValue) > MaxProgressValueLength {
		return Progress{}, "", ErrInvalidProgressValue
	}

	chapterTitle := strings.TrimSpace(input.ChapterTitle)
	if utf8.RuneCountInString(chapterTitle) > MaxChapterTitleLength {
		return Progress{}, "", ErrInvalidChapterTitle
	}

	positionCFI := strings.TrimSpace(input.PositionCFI)
	if utf8.RuneCountInString(positionCFI) > MaxPositionCFILength {
		return Progress{}, "", ErrInvalidPositionCFI
	}

	contentVersion := strings.TrimSpace(input.ContentVersion)
	if utf8.RuneCountInString(contentVersion) > MaxContentVersionLength {
		return Progress{}, "", ErrInvalidContentVersion
	}

	deviceID := strings.TrimSpace(input.DeviceID)
	if utf8.RuneCountInString(deviceID) > MaxDeviceIDLength {
		return Progress{}, "", ErrInvalidDeviceID
	}

	operationID := strings.TrimSpace(input.OperationID)
	if utf8.RuneCountInString(operationID) > MaxOperationIDLength {
		return Progress{}, "", ErrInvalidOperationID
	}

	recordedAt := s.now()
	if input.RecordedAt != nil {
		recordedAt = *input.RecordedAt
	}

	return Progress{
		BookKey:         bookKey,
		ProgressType:    progressType,
		ProgressValue:   progressValue,
		ProgressPercent: percent,
		ChapterTitle:    chapterTitle,
		PositionCFI:     positionCFI,
		ContentVersion:  contentVersion,
		DeviceID:        deviceID,
		RecordedAt:      recordedAt,
	}, operationID, nil
}
