package tts

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
)

// Service manages TTS providers and routes synthesis requests.
type Service struct {
	mu           sync.RWMutex
	providers    map[string]TTSProvider
	defaultVoice string
	db           *sql.DB
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

	s := &Service{
		providers:    make(map[string]TTSProvider),
		defaultVoice: defaultVoice,
		db:           db,
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

// Default returns the first registered provider name.
func (s *Service) Default() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
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
	if s.db != nil && userID > 0 {
		var provider, voice string
		err := s.db.QueryRowContext(ctx,
			"SELECT provider, voice FROM user_tts_prefs WHERE user_id = ?", userID,
		).Scan(&provider, &voice)
		if err == nil {
			p, pErr := s.Provider(provider)
			if pErr == nil {
				return p, voice, nil
			}
		}
	}

	defaultName := s.Default()
	p, err := s.Provider(defaultName)
	if err != nil {
		return nil, "", fmt.Errorf("no tts provider available: %w", err)
	}
	return p, "", nil
}

// SaveUserPref persists a user's TTS provider and voice choice.
func (s *Service) SaveUserPref(ctx context.Context, userID uint64, provider, voice string) error {
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
