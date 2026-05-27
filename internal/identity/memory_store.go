package identity

import (
	"context"
	"database/sql"
	"sync"
	"time"
)

type MemoryStore struct {
	mu            sync.Mutex
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

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
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

func (s *MemoryStore) CreateAccount(ctx context.Context, account Account) (Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
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

func (s *MemoryStore) FindAccountByEmail(ctx context.Context, email string) (Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.emailIndex[email]
	if !ok {
		return Account{}, ErrUserNotFound
	}
	return s.accounts[id], nil
}

func (s *MemoryStore) FindAccountByID(ctx context.Context, userID uint64) (Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	account, ok := s.accounts[userID]
	if !ok {
		return Account{}, ErrUserNotFound
	}
	return account, nil
}

func (s *MemoryStore) UpdateLastLogin(ctx context.Context, userID uint64, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	account, ok := s.accounts[userID]
	if !ok {
		return ErrUserNotFound
	}
	account.LastLoginAt = sql.NullTime{Time: at, Valid: true}
	s.accounts[userID] = account
	return nil
}

func (s *MemoryStore) UpdateProfile(ctx context.Context, userID uint64, nickname, avatarURL string) (Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	account, ok := s.accounts[userID]
	if !ok {
		return Account{}, ErrUserNotFound
	}
	account.Nickname = nickname
	account.AvatarURL = avatarURL
	s.accounts[userID] = account
	return account, nil
}

func (s *MemoryStore) UpdatePasswordHash(ctx context.Context, userID uint64, passwordHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	account, ok := s.accounts[userID]
	if !ok {
		return ErrUserNotFound
	}
	account.PasswordHash = passwordHash
	s.accounts[userID] = account
	return nil
}

func (s *MemoryStore) CreateSession(ctx context.Context, session Session) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session.ID = s.nextSessionID
	s.nextSessionID++
	now := time.Now().UTC()
	session.CreatedAt = now
	session.UpdatedAt = now
	s.sessions[session.ID] = session
	s.sessionIndex[session.RefreshTokenHash] = session.ID
	return session, nil
}

func (s *MemoryStore) FindSessionByRefreshHash(ctx context.Context, refreshHash string) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
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

func (s *MemoryStore) RevokeSession(ctx context.Context, sessionID uint64, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[sessionID]
	if !ok {
		return ErrRefreshRevoked
	}
	session.RevokedAt = sql.NullTime{Time: at, Valid: true}
	s.sessions[sessionID] = session
	return nil
}

func (s *MemoryStore) RevokeUserSessions(ctx context.Context, userID uint64, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, session := range s.sessions {
		if session.UserID == userID {
			session.RevokedAt = sql.NullTime{Time: at, Valid: true}
			s.sessions[id] = session
		}
	}
	return nil
}

func (s *MemoryStore) CreatePasswordResetToken(ctx context.Context, token PasswordResetToken) (PasswordResetToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	token.ID = s.nextResetID
	s.nextResetID++
	token.CreatedAt = time.Now().UTC()
	s.resetTokens[token.ID] = token
	s.resetIndex[token.TokenHash] = token.ID
	return token, nil
}

func (s *MemoryStore) FindPasswordResetToken(ctx context.Context, tokenHash string) (PasswordResetToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.resetIndex[tokenHash]
	if !ok {
		return PasswordResetToken{}, ErrInvalidToken
	}
	return s.resetTokens[id], nil
}

func (s *MemoryStore) MarkPasswordResetTokenUsed(ctx context.Context, tokenID uint64, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	token, ok := s.resetTokens[tokenID]
	if !ok {
		return ErrInvalidToken
	}
	token.UsedAt = sql.NullTime{Time: at, Valid: true}
	s.resetTokens[tokenID] = token
	return nil
}
