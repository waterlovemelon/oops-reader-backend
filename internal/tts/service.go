package tts

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
)

// Service manages TTS providers and routes synthesis requests.
type Service struct {
	mu              sync.RWMutex
	providers       map[string]TTSProvider
	defaultProvider string
	defaultVoice    string
	db              *sql.DB
}

// Config holds all TTS provider configurations.
type Config struct {
	DefaultProvider string      `mapstructure:"default_provider"`
	DefaultVoice    string      `mapstructure:"default_voice"`
	Edge            EdgeConfig  `mapstructure:"edge"`
	MiMo            MiMoConfig  `mapstructure:"mimo"`
}

const fallbackVoice = "Microsoft Server Speech Text to Speech Voice (zh-CN, XiaoxiaoNeural)"

func NewService(cfg Config, db *sql.DB) *Service {
	defaultVoice := cfg.DefaultVoice
	if defaultVoice == "" {
		defaultVoice = fallbackVoice
	}

	defaultProv := cfg.DefaultProvider
	if defaultProv == "" {
		defaultProv = "edge"
	}

	s := &Service{
		providers:       make(map[string]TTSProvider),
		defaultProvider: defaultProv,
		defaultVoice:    defaultVoice,
		db:              db,
	}

	if cfg.Edge.BaseURL != "" {
		s.providers["edge"] = NewEdgeProvider(cfg.Edge)
	}

	if cfg.MiMo.APIKey != "" {
		s.providers["mimo"] = NewMiMoProvider(cfg.MiMo)
	}

	return s
}

// DefaultVoice returns the default voice identifier.
func (s *Service) DefaultVoice() string {
	return s.defaultVoice
}

// Register adds a provider at runtime.
func (s *Service) Register(p TTSProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.providers[p.Name()] = p
}

// Provider returns the named provider, or an error if not found.
func (s *Service) Provider(name string) (TTSProvider, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.providers[name]
	if !ok {
		return nil, fmt.Errorf("tts provider %q not found", name)
	}
	return p, nil
}

// Default returns the configured default provider name, falling back to edge.
func (s *Service) Default() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.providers[s.defaultProvider]; ok {
		return s.defaultProvider
	}
	// Fallback: return any available provider, edge last.
	if _, ok := s.providers["edge"]; ok {
		return "edge"
	}
	for name := range s.providers {
		return name
	}
	return ""
}

// ListProviders returns info about all registered providers.
func (s *Service) ListProviders() []ProviderInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	infos := make([]ProviderInfo, 0, len(s.providers))
	for name, p := range s.providers {
		infos = append(infos, ProviderInfo{
			Name:    name,
			Label:   p.Label(),
			Enabled: true,
		})
	}
	return infos
}

// ProviderForUser resolves the provider to use for a given user.
// Falls back to the default if the user has no preference.
func (s *Service) ProviderForUser(ctx context.Context, userID uint64) (TTSProvider, string, error) {
	// A stored preference that is no longer registered falls back to the
	// default rather than failing synthesis.
	if preference, err := s.UserPreference(ctx, userID); err == nil && preference.Configured {
		if p, pErr := s.Provider(preference.Provider); pErr == nil {
			return p, preference.Voice, nil
		}
	}

	defaultName := s.Default()
	p, err := s.Provider(defaultName)
	if err != nil {
		return nil, "", fmt.Errorf("no tts provider available: %w", err)
	}
	return p, "", nil
}

// ErrUnknownProvider reports a provider name that is not registered.
var ErrUnknownProvider = errors.New("unknown tts provider")

// UserPreference is an account's stored listening preference. Configured is
// false when the account never chose a provider, in which case Provider is the
// service default and Voice is empty.
type UserPreference struct {
	Provider   string
	Voice      string
	Configured bool
}

// UserPreference returns the account's stored provider and voice, so a new
// device can restore the selection made elsewhere.
func (s *Service) UserPreference(ctx context.Context, userID uint64) (UserPreference, error) {
	if s.db == nil || userID == 0 {
		return UserPreference{Provider: s.Default()}, nil
	}

	var provider, voice string
	err := s.db.QueryRowContext(ctx,
		"SELECT provider, voice FROM user_tts_prefs WHERE user_id = ?", userID,
	).Scan(&provider, &voice)
	if err == sql.ErrNoRows {
		return UserPreference{Provider: s.Default()}, nil
	}
	if err != nil {
		return UserPreference{}, fmt.Errorf("read tts preference: %w", err)
	}
	return UserPreference{Provider: provider, Voice: voice, Configured: true}, nil
}

// SaveUserPref persists a user's TTS provider and voice choice. An empty
// provider keeps the provider already stored for the account (or the service
// default), so a client that only changes the voice does not have to know it.
func (s *Service) SaveUserPref(ctx context.Context, userID uint64, provider, voice string) error {
	if provider == "" {
		current, err := s.UserPreference(ctx, userID)
		if err != nil {
			return err
		}
		provider = current.Provider
	} else if _, err := s.Provider(provider); err != nil {
		return fmt.Errorf("%w: %s", ErrUnknownProvider, provider)
	}
	if s.db == nil {
		return fmt.Errorf("database not available")
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO user_tts_prefs (user_id, provider, voice, updated_at)
		 VALUES (?, ?, ?, NOW())
		 ON DUPLICATE KEY UPDATE provider = VALUES(provider), voice = VALUES(voice), updated_at = NOW()`,
		userID, provider, voice,
	)
	return err
}
