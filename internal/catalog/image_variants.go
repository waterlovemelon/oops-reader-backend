package catalog

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const coverVariantsManifestName = "variants.json"

type coverVariantsManifest struct {
	Version  int                    `json:"version"`
	Variants []coverVariantManifest `json:"variants"`
}

type coverVariantManifest struct {
	Key           string `json:"key"`
	Width         int    `json:"width"`
	Height        int    `json:"height"`
	Path          string `json:"path"`
	MediaType     string `json:"media_type"`
	ContentSHA256 string `json:"sha256"`
}

// GetCoverVariant 返回最接近请求宽度的预生成封面；距离相同时选取较大的版本。
func (s *Service) GetCoverVariant(id string, width int) (*ImageVariant, error) {
	if width <= 0 {
		return nil, ErrInvalidImageWidth
	}
	book, err := s.findBook(id)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(book.CoverStoragePath) == "" {
		return nil, fmt.Errorf("%w: book %s has no stored cover", ErrImageVariantNotFound, id)
	}

	coverPath, err := s.catalogStoragePath(book.CoverStoragePath)
	if err != nil {
		return nil, err
	}
	manifestPath := filepath.Join(filepath.Dir(coverPath), coverVariantsManifestName)
	data, err := os.ReadFile(manifestPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w: manifest for book %s", ErrImageVariantNotFound, id)
	}
	if err != nil {
		return nil, fmt.Errorf("read cover variants manifest: %w", err)
	}

	var manifest coverVariantsManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("%w: decode manifest: %v", ErrInvalidImageVariant, err)
	}
	if manifest.Version != 1 {
		return nil, fmt.Errorf("%w: unsupported manifest version %d", ErrInvalidImageVariant, manifest.Version)
	}

	seenKeys := make(map[string]struct{}, len(manifest.Variants))
	var selected *ImageVariant
	for _, entry := range manifest.Variants {
		if _, exists := seenKeys[entry.Key]; exists {
			return nil, fmt.Errorf("%w: duplicate variant key %q", ErrInvalidImageVariant, entry.Key)
		}
		seenKeys[entry.Key] = struct{}{}
		variant, err := s.validatedCoverVariant(filepath.Dir(manifestPath), entry)
		if err != nil {
			return nil, err
		}
		if selected == nil || isCloserImageVariant(variant, selected, width) {
			selected = variant
		}
	}
	if selected == nil {
		return nil, fmt.Errorf("%w: manifest for book %s has no variants", ErrImageVariantNotFound, id)
	}
	return selected, nil
}

func (s *Service) validatedCoverVariant(directory string, entry coverVariantManifest) (*ImageVariant, error) {
	if entry.Key != "small" && entry.Key != "medium" && entry.Key != "large" {
		return nil, fmt.Errorf("%w: unsupported variant key %q", ErrInvalidImageVariant, entry.Key)
	}
	if entry.Width <= 0 || entry.Height <= 0 {
		return nil, fmt.Errorf("%w: invalid dimensions for %s", ErrInvalidImageVariant, entry.Key)
	}
	if !validImageMediaType(entry.MediaType) {
		return nil, fmt.Errorf("%w: unsupported media type %q", ErrInvalidImageVariant, entry.MediaType)
	}
	if !validSHA256(entry.ContentSHA256) {
		return nil, fmt.Errorf("%w: invalid sha256 for %s", ErrInvalidImageVariant, entry.Key)
	}
	if !isPlainFilename(entry.Path) {
		return nil, fmt.Errorf("%w: invalid path for %s", ErrInvalidImageVariant, entry.Key)
	}

	path := filepath.Join(directory, entry.Path)
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w: file for %s", ErrImageVariantNotFound, entry.Key)
	}
	if err != nil {
		return nil, fmt.Errorf("stat cover variant: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: file for %s is not regular", ErrInvalidImageVariant, entry.Key)
	}
	return &ImageVariant{
		Key:           entry.Key,
		Width:         entry.Width,
		Height:        entry.Height,
		MediaType:     entry.MediaType,
		ContentSHA256: strings.ToLower(entry.ContentSHA256),
		Path:          path,
	}, nil
}

func (s *Service) catalogStoragePath(relative string) (string, error) {
	if filepath.IsAbs(relative) {
		return "", fmt.Errorf("%w: absolute storage path", ErrInvalidImageVariant)
	}
	base, err := filepath.Abs(s.root)
	if err != nil {
		return "", fmt.Errorf("resolve catalog root: %w", err)
	}
	path, err := filepath.Abs(filepath.Join(base, filepath.FromSlash(relative)))
	if err != nil {
		return "", fmt.Errorf("resolve storage path: %w", err)
	}
	relativeToRoot, err := filepath.Rel(base, path)
	if err != nil || relativeToRoot == ".." || strings.HasPrefix(relativeToRoot, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: storage path escapes catalog root", ErrInvalidImageVariant)
	}
	return path, nil
}

func isCloserImageVariant(candidate, current *ImageVariant, targetWidth int) bool {
	candidateDistance := absoluteDistance(candidate.Width, targetWidth)
	currentDistance := absoluteDistance(current.Width, targetWidth)
	return candidateDistance < currentDistance ||
		(candidateDistance == currentDistance && candidate.Width > current.Width)
}

func absoluteDistance(left, right int) int {
	if left >= right {
		return left - right
	}
	return right - left
}

func validImageMediaType(mediaType string) bool {
	switch mediaType {
	case "image/jpeg", "image/png", "image/webp", "image/gif":
		return true
	default:
		return false
	}
}

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func isPlainFilename(value string) bool {
	return value != "" && filepath.Base(value) == value && value != "." && value != string(filepath.Separator)
}
