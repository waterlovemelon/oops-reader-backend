package reading

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// MySQLProgressStore implements ProgressStore on top of reading_progress.
type MySQLProgressStore struct {
	db *sql.DB
}

// NewMySQLProgressStore creates a MySQLProgressStore.
func NewMySQLProgressStore(db *sql.DB) *MySQLProgressStore {
	return &MySQLProgressStore{db: db}
}

const progressColumns = `book_key, progress_type, progress_value, progress_percent,
	chapter_title, position_cfi, content_version, updated_by_device_id,
	recorded_at, updated_at`

// GetProgress returns one row, or ErrNotFound when the account has no progress
// for that book yet.
func (s *MySQLProgressStore) GetProgress(ctx context.Context, userID uint64, bookKey string) (Progress, error) {
	progress, err := scanProgress(s.db.QueryRowContext(ctx, `
SELECT `+progressColumns+`
FROM reading_progress
WHERE user_id = ? AND book_key = ?`, userID, bookKey))
	if errors.Is(err, sql.ErrNoRows) {
		return Progress{}, ErrNotFound
	}
	if err != nil {
		return Progress{}, fmt.Errorf("get progress: %w", err)
	}
	return progress, nil
}

// ListProgress returns one page of rows plus the account's total row count.
func (s *MySQLProgressStore) ListProgress(ctx context.Context, userID uint64, limit, offset int) ([]Progress, int, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT `+progressColumns+`
FROM reading_progress
WHERE user_id = ?
ORDER BY updated_at DESC, id DESC
LIMIT ? OFFSET ?`, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list progress: %w", err)
	}
	defer rows.Close()

	items := make([]Progress, 0, limit)
	for rows.Next() {
		progress, err := scanProgress(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan progress: %w", err)
		}
		items = append(items, progress)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("list progress: %w", err)
	}

	var total int
	if err := s.db.QueryRowContext(ctx, `
SELECT COUNT(*) FROM reading_progress WHERE user_id = ?`, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count progress: %w", err)
	}
	return items, total, nil
}

// UpsertProgress merges one upload into the stored row inside a single
// transaction, so concurrent devices cannot interleave a read and a write.
//
// Lock order is users -> sync_operations -> reading_progress:
//   - the users row serializes one account's uploads, which keeps the
//     sync_operations server_version monotonic;
//   - an operation_id replay is answered with the stored row instead of writing
//     again;
//   - the progress row is read FOR UPDATE and only replaced when the upload
//     wins the merge.
func (s *MySQLProgressStore) UpsertProgress(ctx context.Context, userID uint64, incoming Progress, operation ProgressOperation) (UpsertOutcome, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return UpsertOutcome{}, fmt.Errorf("begin progress transaction: %w", err)
	}
	defer tx.Rollback()

	if operation.OperationID != "" {
		if err := lockAccount(ctx, tx, userID); err != nil {
			return UpsertOutcome{}, err
		}
		duplicate, err := recordOperation(ctx, tx, userID, incoming.BookKey, operation)
		if err != nil {
			return UpsertOutcome{}, err
		}
		if duplicate {
			stored, err := lockedProgress(ctx, tx, userID, incoming.BookKey)
			if errors.Is(err, ErrNotFound) {
				// The operation log outlived its row. Nothing to write again.
				return UpsertOutcome{Progress: incoming, Duplicate: true}, nil
			}
			if err != nil {
				return UpsertOutcome{}, err
			}
			return UpsertOutcome{Progress: stored, Duplicate: true}, nil
		}
	}

	stored, err := lockedProgress(ctx, tx, userID, incoming.BookKey)
	applied := false
	switch {
	case errors.Is(err, ErrNotFound):
		if err := insertProgress(ctx, tx, userID, incoming); err != nil {
			return UpsertOutcome{}, err
		}
		applied = true
	case err != nil:
		return UpsertOutcome{}, err
	case Preferred(stored, incoming):
		if err := updateProgress(ctx, tx, userID, incoming); err != nil {
			return UpsertOutcome{}, err
		}
		applied = true
	}

	if applied {
		if err := touchShelfLastRead(ctx, tx, userID, incoming); err != nil {
			return UpsertOutcome{}, err
		}
	}

	authoritative, err := lockedProgress(ctx, tx, userID, incoming.BookKey)
	if err != nil {
		return UpsertOutcome{}, err
	}
	if err := tx.Commit(); err != nil {
		return UpsertOutcome{}, fmt.Errorf("commit progress transaction: %w", err)
	}
	return UpsertOutcome{Progress: authoritative, Applied: applied}, nil
}

func lockAccount(ctx context.Context, tx *sql.Tx, userID uint64) error {
	var id uint64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id = ? FOR UPDATE`, userID).Scan(&id); err != nil {
		return fmt.Errorf("lock account: %w", err)
	}
	return nil
}

// recordOperation appends the upload to sync_operations and reports whether the
// same (device_id, operation_id) had already been applied.
func recordOperation(ctx context.Context, tx *sql.Tx, userID uint64, bookKey string, operation ProgressOperation) (bool, error) {
	var version uint64
	if err := tx.QueryRowContext(ctx, `
SELECT COALESCE(MAX(server_version), 0) + 1 FROM sync_operations WHERE user_id = ?`, userID).Scan(&version); err != nil {
		return false, fmt.Errorf("next sync version: %w", err)
	}

	// The no-op update makes a replay affect zero rows, which is how the
	// duplicate is detected without parsing driver error codes.
	result, err := tx.ExecContext(ctx, `
INSERT INTO sync_operations
	(user_id, device_id, operation_id, entity_type, entity_id, operation_type, payload, client_occurred_at, server_version)
VALUES (?, ?, ?, 'progress', ?, 'update', ?, ?, ?)
ON DUPLICATE KEY UPDATE id = id`,
		userID, operation.DeviceID, operation.OperationID, bookKey, operation.Payload, operation.ClientOccurredAt, version)
	if err != nil {
		return false, fmt.Errorf("record sync operation: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("record sync operation: %w", err)
	}
	return affected == 0, nil
}

func lockedProgress(ctx context.Context, tx *sql.Tx, userID uint64, bookKey string) (Progress, error) {
	progress, err := scanProgress(tx.QueryRowContext(ctx, `
SELECT `+progressColumns+`
FROM reading_progress
WHERE user_id = ? AND book_key = ?
FOR UPDATE`, userID, bookKey))
	if errors.Is(err, sql.ErrNoRows) {
		return Progress{}, ErrNotFound
	}
	if err != nil {
		return Progress{}, fmt.Errorf("read progress: %w", err)
	}
	return progress, nil
}

func insertProgress(ctx context.Context, tx *sql.Tx, userID uint64, progress Progress) error {
	if _, err := tx.ExecContext(ctx, `
INSERT INTO reading_progress
	(user_id, book_key, progress_type, progress_value, progress_percent,
	 chapter_title, position_cfi, content_version, updated_by_device_id, recorded_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		userID,
		progress.BookKey,
		progress.ProgressType,
		progress.ProgressValue,
		nullFloat(progress.ProgressPercent),
		nullText(progress.ChapterTitle),
		nullText(progress.PositionCFI),
		nullText(progress.ContentVersion),
		nullText(progress.DeviceID),
		progress.RecordedAt,
	); err != nil {
		return fmt.Errorf("insert progress: %w", err)
	}
	return nil
}

func updateProgress(ctx context.Context, tx *sql.Tx, userID uint64, progress Progress) error {
	if _, err := tx.ExecContext(ctx, `
UPDATE reading_progress
SET progress_type = ?,
	progress_value = ?,
	progress_percent = ?,
	chapter_title = ?,
	position_cfi = ?,
	content_version = ?,
	updated_by_device_id = ?,
	recorded_at = ?
WHERE user_id = ? AND book_key = ?`,
		progress.ProgressType,
		progress.ProgressValue,
		nullFloat(progress.ProgressPercent),
		nullText(progress.ChapterTitle),
		nullText(progress.PositionCFI),
		nullText(progress.ContentVersion),
		nullText(progress.DeviceID),
		progress.RecordedAt,
		userID,
		progress.BookKey,
	); err != nil {
		return fmt.Errorf("update progress: %w", err)
	}
	return nil
}

// touchShelfLastRead keeps the catalog shelf's "last read" projection aligned
// with progress. It never inserts: adding a book to the shelf stays an explicit
// user action.
func touchShelfLastRead(ctx context.Context, tx *sql.Tx, userID uint64, progress Progress) error {
	catalogBookKey := CatalogBookKey(progress.BookKey)
	if catalogBookKey == "" {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE user_catalog_bookshelves
SET last_read_at = ?
WHERE user_id = ? AND catalog_book_key = ?
  AND (last_read_at IS NULL OR last_read_at < ?)`,
		progress.RecordedAt, userID, catalogBookKey, progress.RecordedAt,
	); err != nil {
		return fmt.Errorf("touch shelf last read: %w", err)
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanProgress(scanner rowScanner) (Progress, error) {
	var progress Progress
	var percent sql.NullFloat64
	var chapterTitle, positionCFI, contentVersion, deviceID sql.NullString
	if err := scanner.Scan(
		&progress.BookKey,
		&progress.ProgressType,
		&progress.ProgressValue,
		&percent,
		&chapterTitle,
		&positionCFI,
		&contentVersion,
		&deviceID,
		&progress.RecordedAt,
		&progress.UpdatedAt,
	); err != nil {
		return Progress{}, err
	}
	if percent.Valid {
		value := percent.Float64
		progress.ProgressPercent = &value
	}
	progress.ChapterTitle = chapterTitle.String
	progress.PositionCFI = positionCFI.String
	progress.ContentVersion = contentVersion.String
	progress.DeviceID = deviceID.String
	return progress, nil
}

func nullText(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullFloat(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}
