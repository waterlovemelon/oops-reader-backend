package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

var ErrReadingNotReady = errors.New("reading content is not ready")
var ErrInvalidReadingParam = errors.New("invalid reading parameter")

const (
	ReadingSchemaVersion     = 2
	ReadingNormalizerVersion = 2
)

type ReadingManifest struct {
	SchemaVersion     int              `json:"schema_version"`
	NormalizerVersion int              `json:"normalizer_version"`
	BookID            string           `json:"book_id"`
	ContentVersion    string           `json:"content_version"`
	Title             string           `json:"title"`
	Author            string           `json:"author,omitempty"`
	Language          string           `json:"language,omitempty"`
	Chapters          []ReadingChapter `json:"chapters"`
	TotalUnits        int64            `json:"total_units"`
	ReadingOrder      []string         `json:"reading_order,omitempty"`
	TOC               json.RawMessage  `json:"toc,omitempty"`
	SegmentIndex      json.RawMessage  `json:"segment_index,omitempty"`
	Resources         json.RawMessage  `json:"resources,omitempty"`
	raw               map[string]json.RawMessage
}
type ReadingChapter struct {
	ID         string   `json:"id"`
	Href       string   `json:"href"`
	Title      string   `json:"title"`
	Order      int      `json:"order"`
	SpineIndex int      `json:"spine_index,omitempty"`
	StartUnit  int64    `json:"start_unit"`
	EndUnit    int64    `json:"end_unit"`
	SegmentIDs []string `json:"segment_ids"`
	raw        map[string]json.RawMessage
}

func (c *ReadingChapter) UnmarshalJSON(body []byte) error {
	type alias ReadingChapter
	var decoded alias
	if err := json.Unmarshal(body, &decoded); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return err
	}
	*c = ReadingChapter(decoded)
	c.raw = fields
	return nil
}

func (c ReadingChapter) MarshalJSON() ([]byte, error) {
	type alias ReadingChapter
	encoded, err := json.Marshal(alias(c))
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		return nil, err
	}
	for key, value := range c.raw {
		fields[key] = value
	}
	return json.Marshal(fields)
}

// Retain fields added by newer preprocessors when proxying a manifest.
func (m *ReadingManifest) UnmarshalJSON(body []byte) error {
	type alias ReadingManifest
	var decoded alias
	if err := json.Unmarshal(body, &decoded); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return err
	}
	*m = ReadingManifest(decoded)
	m.raw = fields
	return nil
}

func (m ReadingManifest) MarshalJSON() ([]byte, error) {
	type alias ReadingManifest
	fields := make(map[string]json.RawMessage, len(m.raw)+12)
	for key, value := range m.raw {
		fields[key] = value
	}
	encoded, err := json.Marshal(alias(m))
	if err != nil {
		return nil, err
	}
	var known map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &known); err != nil {
		return nil, err
	}
	for key, value := range known {
		fields[key] = value
	}
	return json.Marshal(fields)
}

func (s *Service) LoadReadingManifest(id, version string) (ReadingManifest, string, error) {
	root, err := s.ReadingVersionRoot(id, version)
	if err != nil {
		return ReadingManifest{}, "", err
	}
	// Resolve the current pointer once before reading it. The content version
	// then identifies an immutable directory for all following artifact reads.
	if version == "current" || version == "" {
		resolved, resolveErr := filepath.EvalSymlinks(root)
		if resolveErr != nil {
			if errors.Is(resolveErr, os.ErrNotExist) {
				return ReadingManifest{}, "", fmt.Errorf("%w: %s", ErrReadingNotReady, id)
			}
			return ReadingManifest{}, "", resolveErr
		}
		root = resolved
	}
	manifestPath := filepath.Join(root, "manifest.json")
	body, err := os.ReadFile(manifestPath)
	if errors.Is(err, os.ErrNotExist) {
		if version != "current" && version != "" {
			return ReadingManifest{}, "", fmt.Errorf("%w: version %s", ErrNotFound, version)
		}
		return ReadingManifest{}, "", fmt.Errorf("%w: %s", ErrReadingNotReady, id)
	}
	if err != nil {
		return ReadingManifest{}, "", err
	}
	var m ReadingManifest
	if err := json.Unmarshal(body, &m); err != nil {
		return m, "", fmt.Errorf("decode manifest: %w", err)
	}
	if m.BookID != "" && m.BookID != id {
		return ReadingManifest{}, "", fmt.Errorf("%w: manifest book mismatch", ErrInvalidReadingParam)
	}
	if m.BookID == "" {
		m.BookID = id
	}
	if m.SchemaVersion != 0 && m.SchemaVersion != ReadingSchemaVersion {
		return ReadingManifest{}, "", fmt.Errorf("%w: unsupported schema version %d", ErrInvalidReadingParam, m.SchemaVersion)
	}
	if m.NormalizerVersion != 0 && m.NormalizerVersion != ReadingNormalizerVersion {
		return ReadingManifest{}, "", fmt.Errorf("%w: unsupported normalizer version %d", ErrInvalidReadingParam, m.NormalizerVersion)
	}
	if m.ContentVersion == "" {
		if version == "current" || version == "" {
			return ReadingManifest{}, "", fmt.Errorf("%w: manifest has no content version", ErrInvalidReadingParam)
		}
		m.ContentVersion = version
	}
	if version != "" && version != "current" && m.ContentVersion != version {
		return ReadingManifest{}, "", fmt.Errorf("%w: manifest content version mismatch", ErrInvalidReadingParam)
	}
	return m, root, nil
}

func safeReadingName(value string) (string, error) {
	decoded, err := url.PathUnescape(value)
	if err != nil {
		return "", err
	}
	decoded = filepath.ToSlash(decoded)
	clean := filepath.ToSlash(filepath.Clean(decoded))
	for _, part := range strings.Split(decoded, "/") {
		if part == ".." {
			return "", fmt.Errorf("invalid reading asset name")
		}
	}
	if decoded == "" || clean == "." || strings.HasPrefix(clean, "../") || clean == ".." || strings.Contains(decoded, "\\") {
		return "", fmt.Errorf("invalid reading asset name")
	}
	return clean, nil
}

func safeReadingComponent(value string) (string, error) {
	clean, err := safeReadingName(value)
	if err != nil || strings.Contains(clean, "/") {
		return "", fmt.Errorf("invalid reading path component")
	}
	return clean, nil
}

func (s *Service) LoadReadingFile(id, version, name string) ([]byte, string, error) {
	clean, err := safeReadingName(name)
	if err != nil {
		return nil, "", fmt.Errorf("%w: asset path", ErrInvalidReadingParam)
	}
	_, root, err := s.LoadReadingManifest(id, version)
	if err != nil {
		return nil, "", err
	}
	path := filepath.Join(root, clean)
	// Also reject symlinks escaping the immutable version directory.
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, "", err
	}
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, "", fmt.Errorf("%w: asset %s", ErrNotFound, clean)
		}
		return nil, "", err
	}
	rel, err := filepath.Rel(realRoot, realPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return nil, "", fmt.Errorf("%w: asset path", ErrInvalidReadingParam)
	}
	body, err := os.ReadFile(realPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, "", fmt.Errorf("%w: asset %s", ErrNotFound, clean)
	}
	if err != nil {
		return nil, "", err
	}
	return body, realPath, nil
}
