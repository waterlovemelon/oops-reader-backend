package backup

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"time"
)

var ErrCloudDataExists = errors.New("cloud backup already exists")

type UploadMode string

const (
	ModeCreateIfEmpty UploadMode = "create_if_empty"
	ModeOverwrite     UploadMode = "overwrite"
)

type Backup struct {
	ID               uint64
	UserID           uint64
	SchemaVersion    uint
	Payload          json.RawMessage
	BookCount        uint
	NoteCount        uint
	ProgressCount    uint
	PreferenceCount  uint
	SourceDeviceID   string
	SourceDeviceName string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type Summary struct {
	Exists          bool       `json:"exists"`
	BackupID        string     `json:"backup_id,omitempty"`
	BookCount       uint       `json:"book_count"`
	NoteCount       uint       `json:"note_count"`
	ProgressCount   uint       `json:"progress_count"`
	PreferenceCount uint       `json:"preference_count"`
	UpdatedAt       *time.Time `json:"updated_at,omitempty"`
	DeviceName      string     `json:"device_name,omitempty"`
}

type Store interface {
	GetLatest(ctx context.Context, userID uint64) (Backup, bool, error)
	UpsertLatest(ctx context.Context, backup Backup) (Backup, error)
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) Summary(ctx context.Context, userID uint64) (Summary, error) {
	backup, ok, err := s.store.GetLatest(ctx, userID)
	if err != nil {
		return Summary{}, err
	}
	if !ok {
		return Summary{Exists: false}, nil
	}
	return backupSummary(backup), nil
}

func (s *Service) Upload(ctx context.Context, mode UploadMode, backup Backup) (Backup, error) {
	if mode == "" {
		mode = ModeCreateIfEmpty
	}
	if mode != ModeCreateIfEmpty && mode != ModeOverwrite {
		return Backup{}, errors.New("invalid upload mode")
	}
	if mode == ModeCreateIfEmpty {
		if _, exists, err := s.store.GetLatest(ctx, backup.UserID); err != nil {
			return Backup{}, err
		} else if exists {
			return Backup{}, ErrCloudDataExists
		}
	}
	return s.store.UpsertLatest(ctx, backup)
}

func (s *Service) Download(ctx context.Context, userID uint64) (Backup, bool, error) {
	return s.store.GetLatest(ctx, userID)
}

type MySQLStore struct {
	db *sql.DB
}

func NewMySQLStore(db *sql.DB) *MySQLStore {
	return &MySQLStore{db: db}
}

func (s *MySQLStore) GetLatest(ctx context.Context, userID uint64) (Backup, bool, error) {
	var backup Backup
	var payload []byte
	var sourceDeviceName sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT
			id, user_id, schema_version, payload,
			book_count, note_count, progress_count, preference_count,
			source_device_id, source_device_name, created_at, updated_at
		FROM user_data_backups
		WHERE user_id = ?
	`, userID).Scan(
		&backup.ID,
		&backup.UserID,
		&backup.SchemaVersion,
		&payload,
		&backup.BookCount,
		&backup.NoteCount,
		&backup.ProgressCount,
		&backup.PreferenceCount,
		&backup.SourceDeviceID,
		&sourceDeviceName,
		&backup.CreatedAt,
		&backup.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Backup{}, false, nil
		}
		return Backup{}, false, err
	}
	backup.Payload = json.RawMessage(payload)
	backup.SourceDeviceName = sourceDeviceName.String
	return backup, true, nil
}

func (s *MySQLStore) UpsertLatest(ctx context.Context, backup Backup) (Backup, error) {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO user_data_backups (
			user_id, schema_version, payload, book_count, note_count,
			progress_count, preference_count, source_device_id, source_device_name
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			schema_version = VALUES(schema_version),
			payload = VALUES(payload),
			book_count = VALUES(book_count),
			note_count = VALUES(note_count),
			progress_count = VALUES(progress_count),
			preference_count = VALUES(preference_count),
			source_device_id = VALUES(source_device_id),
			source_device_name = VALUES(source_device_name),
			updated_at = CURRENT_TIMESTAMP
	`, backup.UserID, backup.SchemaVersion, []byte(backup.Payload), backup.BookCount, backup.NoteCount,
		backup.ProgressCount, backup.PreferenceCount, backup.SourceDeviceID, nullableString(backup.SourceDeviceName))
	if err != nil {
		return Backup{}, err
	}
	latest, _, err := s.GetLatest(ctx, backup.UserID)
	return latest, err
}

func backupSummary(backup Backup) Summary {
	updatedAt := backup.UpdatedAt
	return Summary{
		Exists:          true,
		BackupID:        strconv.FormatUint(backup.ID, 10),
		BookCount:       backup.BookCount,
		NoteCount:       backup.NoteCount,
		ProgressCount:   backup.ProgressCount,
		PreferenceCount: backup.PreferenceCount,
		UpdatedAt:       &updatedAt,
		DeviceName:      backup.SourceDeviceName,
	}
}

func nullableString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}
