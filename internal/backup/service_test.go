package backup

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestUploadCreateIfEmptySucceeds(t *testing.T) {
	store := newFakeStore()
	service := NewService(store)

	saved, err := service.Upload(context.Background(), ModeCreateIfEmpty, sampleBackup(1, "first"))
	if err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}
	if saved.ID == 0 {
		t.Fatal("saved backup missing ID")
	}
}

func TestUploadCreateIfEmptyConflictsWhenBackupExists(t *testing.T) {
	store := newFakeStore()
	service := NewService(store)

	if _, err := service.Upload(context.Background(), ModeCreateIfEmpty, sampleBackup(1, "first")); err != nil {
		t.Fatalf("first Upload returned error: %v", err)
	}
	if _, err := service.Upload(context.Background(), ModeCreateIfEmpty, sampleBackup(1, "second")); !errors.Is(err, ErrCloudDataExists) {
		t.Fatalf("second Upload error = %v, want ErrCloudDataExists", err)
	}
}

func TestUploadOverwriteReplacesBackup(t *testing.T) {
	store := newFakeStore()
	service := NewService(store)

	if _, err := service.Upload(context.Background(), ModeCreateIfEmpty, sampleBackup(1, "first")); err != nil {
		t.Fatalf("first Upload returned error: %v", err)
	}
	saved, err := service.Upload(context.Background(), ModeOverwrite, sampleBackup(1, "second"))
	if err != nil {
		t.Fatalf("overwrite Upload returned error: %v", err)
	}
	if string(saved.Payload) != `{"name":"second"}` {
		t.Fatalf("payload = %s, want second payload", saved.Payload)
	}
}

func TestDownloadReturnsLatestPayload(t *testing.T) {
	store := newFakeStore()
	service := NewService(store)

	if _, err := service.Upload(context.Background(), ModeOverwrite, sampleBackup(1, "first")); err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}
	backup, ok, err := service.Download(context.Background(), 1)
	if err != nil {
		t.Fatalf("Download returned error: %v", err)
	}
	if !ok {
		t.Fatal("Download returned ok=false")
	}
	if !json.Valid(backup.Payload) {
		t.Fatalf("payload is not valid JSON: %s", backup.Payload)
	}
}

func sampleBackup(userID uint64, name string) Backup {
	return Backup{
		UserID:           userID,
		SchemaVersion:    1,
		Payload:          json.RawMessage(`{"name":"` + name + `"}`),
		BookCount:        2,
		NoteCount:        3,
		ProgressCount:    2,
		PreferenceCount:  1,
		SourceDeviceID:   "device-1",
		SourceDeviceName: "Phone",
	}
}

type fakeStore struct {
	nextID uint64
	latest map[uint64]Backup
}

func newFakeStore() *fakeStore {
	return &fakeStore{nextID: 1, latest: map[uint64]Backup{}}
}

func (s *fakeStore) GetLatest(ctx context.Context, userID uint64) (Backup, bool, error) {
	backup, ok := s.latest[userID]
	return backup, ok, nil
}

func (s *fakeStore) UpsertLatest(ctx context.Context, backup Backup) (Backup, error) {
	if existing, ok := s.latest[backup.UserID]; ok {
		backup.ID = existing.ID
		backup.CreatedAt = existing.CreatedAt
	} else {
		backup.ID = s.nextID
		s.nextID++
		backup.CreatedAt = time.Now().UTC()
	}
	backup.UpdatedAt = time.Now().UTC()
	s.latest[backup.UserID] = backup
	return backup, nil
}
