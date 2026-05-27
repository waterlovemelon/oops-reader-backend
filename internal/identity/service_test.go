package identity

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

func TestRegisterLoginRefreshLogoutLifecycle(t *testing.T) {
	store := newFakeStore()
	service := NewService(store, "test-secret")
	service.now = fixedClock()

	ctx := context.Background()
	registered, err := service.Register(ctx, RegisterInput{
		Email:      "reader@example.com",
		Password:   "password12345",
		Nickname:   "Reader",
		DeviceID:   "device-1",
		DeviceName: "Phone",
		Platform:   "ios",
	})
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	if registered.User.ID == 0 {
		t.Fatal("Register returned empty user ID")
	}
	if registered.AccessToken == "" || registered.RefreshToken == "" {
		t.Fatal("Register returned empty tokens")
	}

	login, err := service.Login(ctx, LoginInput{
		Email:    "reader@example.com",
		Password: "password12345",
		DeviceID: "device-1",
		Platform: "ios",
	})
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}
	if login.User.ID != registered.User.ID {
		t.Fatalf("Login user ID = %d, want %d", login.User.ID, registered.User.ID)
	}

	refreshed, err := service.Refresh(ctx, login.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh returned error: %v", err)
	}
	if refreshed.RefreshToken == login.RefreshToken {
		t.Fatal("Refresh did not rotate refresh token")
	}
	if _, err := service.Refresh(ctx, login.RefreshToken); !errors.Is(err, ErrRefreshRevoked) {
		t.Fatalf("Refresh with old token error = %v, want ErrRefreshRevoked", err)
	}

	if err := service.Logout(ctx, refreshed.RefreshToken); err != nil {
		t.Fatalf("Logout returned error: %v", err)
	}
	if _, err := service.Refresh(ctx, refreshed.RefreshToken); !errors.Is(err, ErrRefreshRevoked) {
		t.Fatalf("Refresh after logout error = %v, want ErrRefreshRevoked", err)
	}
}

func TestRegisterRejectsDuplicateEmail(t *testing.T) {
	store := newFakeStore()
	service := NewService(store, "test-secret")
	ctx := context.Background()

	input := RegisterInput{
		Email:    "reader@example.com",
		Password: "password12345",
		Nickname: "Reader",
		DeviceID: "device-1",
		Platform: "ios",
	}
	if _, err := service.Register(ctx, input); err != nil {
		t.Fatalf("first Register returned error: %v", err)
	}
	if _, err := service.Register(ctx, input); !errors.Is(err, ErrDuplicateEmail) {
		t.Fatalf("second Register error = %v, want ErrDuplicateEmail", err)
	}
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	store := newFakeStore()
	service := NewService(store, "test-secret")
	ctx := context.Background()

	if _, err := service.Register(ctx, RegisterInput{
		Email:    "reader@example.com",
		Password: "password12345",
		DeviceID: "device-1",
		Platform: "ios",
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	if _, err := service.Login(ctx, LoginInput{
		Email:    "reader@example.com",
		Password: "wrong-password",
		DeviceID: "device-1",
		Platform: "ios",
	}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login error = %v, want ErrInvalidCredentials", err)
	}
}

func TestFrozenAccountCannotLogin(t *testing.T) {
	store := newFakeStore()
	service := NewService(store, "test-secret")
	ctx := context.Background()

	result, err := service.Register(ctx, RegisterInput{
		Email:    "reader@example.com",
		Password: "password12345",
		DeviceID: "device-1",
		Platform: "ios",
	})
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	account := store.accounts[result.User.ID]
	account.AccountStatus = AccountStatusFrozen
	store.accounts[result.User.ID] = account

	if _, err := service.Login(ctx, LoginInput{
		Email:    "reader@example.com",
		Password: "password12345",
		DeviceID: "device-1",
		Platform: "ios",
	}); !errors.Is(err, ErrAccountForbidden) {
		t.Fatalf("Login error = %v, want ErrAccountForbidden", err)
	}
}

func TestPasswordResetRevokesSessions(t *testing.T) {
	store := newFakeStore()
	service := NewService(store, "test-secret")
	service.now = fixedClock()
	var resetToken string
	service.sendPasswordResetEmail = func(email, token string) error {
		resetToken = token
		return nil
	}
	ctx := context.Background()

	login, err := service.Register(ctx, RegisterInput{
		Email:    "reader@example.com",
		Password: "password12345",
		DeviceID: "device-1",
		Platform: "ios",
	})
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	if err := service.RequestPasswordReset(ctx, "reader@example.com"); err != nil {
		t.Fatalf("RequestPasswordReset returned error: %v", err)
	}
	if resetToken == "" {
		t.Fatal("password reset email was not sent")
	}
	if err := service.ConfirmPasswordReset(ctx, resetToken, "new-password12345"); err != nil {
		t.Fatalf("ConfirmPasswordReset returned error: %v", err)
	}
	if _, err := service.Refresh(ctx, login.RefreshToken); !errors.Is(err, ErrRefreshRevoked) {
		t.Fatalf("Refresh after password reset error = %v, want ErrRefreshRevoked", err)
	}
	if _, err := service.Login(ctx, LoginInput{
		Email:    "reader@example.com",
		Password: "new-password12345",
		DeviceID: "device-1",
		Platform: "ios",
	}); err != nil {
		t.Fatalf("Login with new password returned error: %v", err)
	}
}

func fixedClock() func() time.Time {
	return func() time.Time {
		return time.Date(2026, 5, 28, 1, 0, 0, 0, time.UTC)
	}
}

type fakeStore struct {
	nextAccountID uint64
	nextSessionID uint64
	nextResetID   uint64
	accounts      map[uint64]Account
	emailIndex    map[string]uint64
	sessions      map[uint64]Session
	sessionIndex  map[string]uint64
	resetTokens   map[uint64]PasswordResetToken
	resetIndex    map[string]uint64
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		nextAccountID: 1,
		nextSessionID: 1,
		nextResetID:   1,
		accounts:      map[uint64]Account{},
		emailIndex:    map[string]uint64{},
		sessions:      map[uint64]Session{},
		sessionIndex:  map[string]uint64{},
		resetTokens:   map[uint64]PasswordResetToken{},
		resetIndex:    map[string]uint64{},
	}
}

func (s *fakeStore) CreateAccount(ctx context.Context, account Account) (Account, error) {
	if _, exists := s.emailIndex[account.Email]; exists {
		return Account{}, ErrDuplicateEmail
	}
	account.ID = s.nextAccountID
	s.nextAccountID++
	now := time.Now().UTC()
	account.CreatedAt = now
	account.UpdatedAt = now
	s.accounts[account.ID] = account
	s.emailIndex[account.Email] = account.ID
	return account, nil
}

func (s *fakeStore) FindAccountByEmail(ctx context.Context, email string) (Account, error) {
	id, ok := s.emailIndex[email]
	if !ok {
		return Account{}, ErrUserNotFound
	}
	return s.accounts[id], nil
}

func (s *fakeStore) FindAccountByID(ctx context.Context, userID uint64) (Account, error) {
	account, ok := s.accounts[userID]
	if !ok {
		return Account{}, ErrUserNotFound
	}
	return account, nil
}

func (s *fakeStore) UpdateLastLogin(ctx context.Context, userID uint64, at time.Time) error {
	account, ok := s.accounts[userID]
	if !ok {
		return ErrUserNotFound
	}
	account.LastLoginAt = sql.NullTime{Time: at, Valid: true}
	s.accounts[userID] = account
	return nil
}

func (s *fakeStore) UpdateProfile(ctx context.Context, userID uint64, nickname, avatarURL string) (Account, error) {
	account, ok := s.accounts[userID]
	if !ok {
		return Account{}, ErrUserNotFound
	}
	account.Nickname = nickname
	account.AvatarURL = avatarURL
	s.accounts[userID] = account
	return account, nil
}

func (s *fakeStore) UpdatePasswordHash(ctx context.Context, userID uint64, passwordHash string) error {
	account, ok := s.accounts[userID]
	if !ok {
		return ErrUserNotFound
	}
	account.PasswordHash = passwordHash
	s.accounts[userID] = account
	return nil
}

func (s *fakeStore) CreateSession(ctx context.Context, session Session) (Session, error) {
	session.ID = s.nextSessionID
	s.nextSessionID++
	now := time.Now().UTC()
	session.CreatedAt = now
	session.UpdatedAt = now
	s.sessions[session.ID] = session
	s.sessionIndex[session.RefreshTokenHash] = session.ID
	return session, nil
}

func (s *fakeStore) FindSessionByRefreshHash(ctx context.Context, refreshHash string) (Session, error) {
	id, ok := s.sessionIndex[refreshHash]
	if !ok {
		return Session{}, ErrRefreshRevoked
	}
	session := s.sessions[id]
	if session.RevokedAt.Valid {
		return Session{}, ErrRefreshRevoked
	}
	return session, nil
}

func (s *fakeStore) RevokeSession(ctx context.Context, sessionID uint64, at time.Time) error {
	session, ok := s.sessions[sessionID]
	if !ok {
		return ErrRefreshRevoked
	}
	session.RevokedAt = sql.NullTime{Time: at, Valid: true}
	s.sessions[sessionID] = session
	return nil
}

func (s *fakeStore) RevokeUserSessions(ctx context.Context, userID uint64, at time.Time) error {
	for id, session := range s.sessions {
		if session.UserID == userID {
			session.RevokedAt = sql.NullTime{Time: at, Valid: true}
			s.sessions[id] = session
		}
	}
	return nil
}

func (s *fakeStore) CreatePasswordResetToken(ctx context.Context, token PasswordResetToken) (PasswordResetToken, error) {
	token.ID = s.nextResetID
	s.nextResetID++
	token.CreatedAt = time.Now().UTC()
	s.resetTokens[token.ID] = token
	s.resetIndex[token.TokenHash] = token.ID
	return token, nil
}

func (s *fakeStore) FindPasswordResetToken(ctx context.Context, tokenHash string) (PasswordResetToken, error) {
	id, ok := s.resetIndex[tokenHash]
	if !ok {
		return PasswordResetToken{}, ErrInvalidToken
	}
	return s.resetTokens[id], nil
}

func (s *fakeStore) MarkPasswordResetTokenUsed(ctx context.Context, tokenID uint64, at time.Time) error {
	token, ok := s.resetTokens[tokenID]
	if !ok {
		return ErrInvalidToken
	}
	token.UsedAt = sql.NullTime{Time: at, Valid: true}
	s.resetTokens[tokenID] = token
	return nil
}
