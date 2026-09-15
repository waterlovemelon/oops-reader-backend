package tts

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
)

// The preference code talks to exactly two statements; a tiny in-memory driver
// answers them, so the stored round trip is testable without a database.
var ttsPrefState = &fakePrefState{}

func init() { sql.Register("tts-preference-fake", fakeTTSDriver{}) }

type fakePrefState struct {
	mu       sync.Mutex
	provider string
	voice    string
	hasRow   bool
}

func (s *fakePrefState) reset(provider, voice string, hasRow bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.provider, s.voice, s.hasRow = provider, voice, hasRow
}

func (s *fakePrefState) stored() (string, string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.provider, s.voice, s.hasRow
}

type fakeTTSDriver struct{}

func (fakeTTSDriver) Open(string) (driver.Conn, error) { return fakeTTSConn{}, nil }

type fakeTTSConn struct{}

func (fakeTTSConn) Prepare(query string) (driver.Stmt, error) { return fakeTTSStmt{query: query}, nil }
func (fakeTTSConn) Close() error                              { return nil }
func (fakeTTSConn) Begin() (driver.Tx, error) {
	return nil, errors.New("transactions are not supported")
}

type fakeTTSStmt struct{ query string }

func (fakeTTSStmt) Close() error  { return nil }
func (fakeTTSStmt) NumInput() int { return -1 }

func (s fakeTTSStmt) Exec(args []driver.Value) (driver.Result, error) {
	if !strings.Contains(s.query, "INSERT INTO user_tts_prefs") {
		return nil, errors.New("unexpected statement: " + s.query)
	}
	provider, _ := args[1].(string)
	voice, _ := args[2].(string)
	ttsPrefState.reset(provider, voice, true)
	return driver.RowsAffected(1), nil
}

func (s fakeTTSStmt) Query([]driver.Value) (driver.Rows, error) {
	if !strings.Contains(s.query, "FROM user_tts_prefs") {
		return nil, errors.New("unexpected statement: " + s.query)
	}
	provider, voice, hasRow := ttsPrefState.stored()
	if !hasRow {
		return &fakeTTSRows{}, nil
	}
	return &fakeTTSRows{
		columns: []string{"provider", "voice"},
		values:  [][]driver.Value{{provider, voice}},
	}, nil
}

type fakeTTSRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *fakeTTSRows) Columns() []string { return r.columns }
func (r *fakeTTSRows) Close() error      { return nil }

func (r *fakeTTSRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

type stubProvider struct{ name string }

func (p stubProvider) Name() string  { return p.name }
func (p stubProvider) Label() string { return p.name }
func (p stubProvider) Synthesize(context.Context, SynthesizeRequest) (*SynthesizeResponse, error) {
	return &SynthesizeResponse{}, nil
}
func (p stubProvider) ListVoices(context.Context, string) ([]Voice, error) { return nil, nil }

func newPreferenceService(t *testing.T, providers ...string) *Service {
	t.Helper()
	db, err := sql.Open("tts-preference-fake", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	service := NewService(Config{DefaultProvider: providers[0]}, db)
	for _, name := range providers {
		service.Register(stubProvider{name: name})
	}
	return service
}

func TestUserPreferenceWithoutStoredRowUsesTheDefaultProvider(t *testing.T) {
	ttsPrefState.reset("", "", false)
	service := newPreferenceService(t, "edge", "mimo")

	preference, err := service.UserPreference(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	if preference.Configured {
		t.Fatalf("preference = %+v, want configured false", preference)
	}
	if preference.Provider != "edge" || preference.Voice != "" {
		t.Fatalf("preference = %+v, want the default provider and no voice", preference)
	}
}

func TestUserPreferenceReportsTheStoredSelection(t *testing.T) {
	ttsPrefState.reset("mimo", "voice-x", true)
	service := newPreferenceService(t, "edge", "mimo")

	preference, err := service.UserPreference(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	if !preference.Configured || preference.Provider != "mimo" || preference.Voice != "voice-x" {
		t.Fatalf("preference = %+v, want the stored mimo/voice-x as configured", preference)
	}
}

func TestUserPreferenceWithoutDatabaseIsUnconfigured(t *testing.T) {
	service := NewService(Config{DefaultProvider: "edge"}, nil)
	service.Register(stubProvider{name: "edge"})

	preference, err := service.UserPreference(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	if preference.Configured || preference.Provider != "edge" {
		t.Fatalf("preference = %+v, want an unconfigured edge default", preference)
	}
}

func TestSaveUserPrefKeepsTheStoredProviderWhenNoneIsGiven(t *testing.T) {
	ttsPrefState.reset("mimo", "voice-x", true)
	service := newPreferenceService(t, "edge", "mimo")

	if err := service.SaveUserPref(context.Background(), 42, "", "voice-y"); err != nil {
		t.Fatal(err)
	}
	provider, voice, hasRow := ttsPrefState.stored()
	if !hasRow || provider != "mimo" || voice != "voice-y" {
		t.Fatalf("stored %q/%q (row %v), want the kept provider mimo with voice-y", provider, voice, hasRow)
	}
}

func TestSaveUserPrefWithoutStoredRowFallsBackToTheDefaultProvider(t *testing.T) {
	ttsPrefState.reset("", "", false)
	service := newPreferenceService(t, "edge")

	if err := service.SaveUserPref(context.Background(), 42, "", "voice-y"); err != nil {
		t.Fatal(err)
	}
	provider, voice, _ := ttsPrefState.stored()
	if provider != "edge" || voice != "voice-y" {
		t.Fatalf("stored %q/%q, want the default provider edge", provider, voice)
	}
}

func TestSaveUserPrefRejectsAnUnregisteredProvider(t *testing.T) {
	ttsPrefState.reset("mimo", "voice-x", true)
	service := newPreferenceService(t, "edge", "mimo")

	err := service.SaveUserPref(context.Background(), 42, "ghost", "voice-y")
	if !errors.Is(err, ErrUnknownProvider) {
		t.Fatalf("SaveUserPref(ghost) = %v, want ErrUnknownProvider", err)
	}
	provider, voice, _ := ttsPrefState.stored()
	if provider != "mimo" || voice != "voice-x" {
		t.Fatalf("stored %q/%q, want the untouched mimo/voice-x", provider, voice)
	}
}

func TestProviderForUserUsesTheStoredProviderAndVoice(t *testing.T) {
	ttsPrefState.reset("mimo", "voice-x", true)
	service := newPreferenceService(t, "edge", "mimo")

	provider, voice, err := service.ProviderForUser(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	if provider.Name() != "mimo" || voice != "voice-x" {
		t.Fatalf("resolved %q/%q, want the stored mimo/voice-x", provider.Name(), voice)
	}
}
