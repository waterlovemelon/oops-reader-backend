package community

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// AttachmentStorage abstracts file storage for community attachments.
// Implementations: LocalStorage (disk), future: S3Storage, OSSStorage, COSStorage.
type AttachmentStorage interface {
	Save(ctx context.Context, input SaveAttachmentInput) (StoredAttachment, error)
	URL(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, key string) error
}

// SaveAttachmentInput contains the data needed to store an attachment file.
type SaveAttachmentInput struct {
	OwnerUserID string
	Filename    string
	Reader      io.Reader
}

// StoredAttachment contains the result of saving a file.
type StoredAttachment struct {
	StorageKey  string
	PublicURL   string
	MIMEType    string
	FileSize    uint
	Width       uint
	Height      uint
	ChecksumSHA256 string
}

// LocalStorageConfig configures the local file storage.
type LocalStorageConfig struct {
	BasePath  string // e.g. "/data/oops-reader/community/images"
	PublicURL string // e.g. "https://example.com/community/images"
}

// LocalStorage saves attachment files to the local filesystem.
type LocalStorage struct {
	basePath  string
	publicURL string
}

// NewLocalStorage creates a LocalStorage with the given config.
func NewLocalStorage(cfg LocalStorageConfig) *LocalStorage {
	return &LocalStorage{
		basePath:  cfg.BasePath,
		publicURL: strings.TrimRight(cfg.PublicURL, "/"),
	}
}

// Save writes the file to disk and returns metadata.
func (ls *LocalStorage) Save(_ context.Context, input SaveAttachmentInput) (StoredAttachment, error) {
	// Read the entire file into memory so we can compute checksum and detect MIME.
	data, err := io.ReadAll(input.Reader)
	if err != nil {
		return StoredAttachment{}, fmt.Errorf("read upload: %w", err)
	}

	// Detect MIME from content (not filename).
	mimeType := http.DetectContentType(data)
	if !allowedMIME(mimeType) {
		return StoredAttachment{}, fmt.Errorf("%w: unsupported image type %s", ErrInvalidInput, mimeType)
	}

	// Compute SHA-256.
	hash := sha256.Sum256(data)
	checksum := hex.EncodeToString(hash[:])

	// Generate storage key: 2026/05/att_xxx.ext
	now := time.Now().UTC()
	ext := mimeExtension(mimeType)
	attID := newID("att")
	relPath := fmt.Sprintf("%d/%02d/%s%s", now.Year(), now.Month(), attID, ext)

	// Ensure directory exists.
	fullPath := filepath.Join(ls.basePath, relPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return StoredAttachment{}, fmt.Errorf("create directory: %w", err)
	}

	// Write file.
	if err := os.WriteFile(fullPath, data, 0o644); err != nil {
		return StoredAttachment{}, fmt.Errorf("write file: %w", err)
	}

	// Detect image dimensions.
	width, height := detectDimensions(bytes.NewReader(data), mimeType)

	publicURL := ls.publicURL + "/" + relPath

	return StoredAttachment{
		StorageKey:     relPath,
		PublicURL:      publicURL,
		MIMEType:       mimeType,
		FileSize:       uint(len(data)),
		Width:          width,
		Height:         height,
		ChecksumSHA256: checksum,
	}, nil
}

// URL returns the public URL for a storage key.
func (ls *LocalStorage) URL(_ context.Context, key string) (string, error) {
	return ls.publicURL + "/" + key, nil
}

// Delete removes a file from disk.
func (ls *LocalStorage) Delete(_ context.Context, key string) error {
	fullPath := filepath.Join(ls.basePath, key)
	return os.Remove(fullPath)
}

// ── helpers ──────────────────────────────────────────────────────────────────

var allowedMIMETypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
	"image/gif":  true,
}

func allowedMIME(mime string) bool {
	return allowedMIMETypes[mime]
}

func mimeExtension(mime string) string {
	switch mime {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ".bin"
	}
}

func detectDimensions(r io.Reader, mimeType string) (uint, uint) {
	cfg, _, err := image.DecodeConfig(r)
	if err != nil {
		return 0, 0
	}
	return uint(cfg.Width), uint(cfg.Height)
}
