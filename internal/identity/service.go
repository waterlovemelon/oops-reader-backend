package identity

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	UserStatusGuest      = "guest"
	UserStatusRegistered = "registered"

	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

var (
	ErrInvalidInput   = errors.New("invalid input")
	ErrUserNotFound   = errors.New("user not found")
	ErrInvalidToken   = errors.New("invalid token")
	ErrExpiredToken   = errors.New("expired token")
	ErrRefreshRevoked = errors.New("refresh token revoked")
)

type User struct {
	ID        string
	DeviceID  string
	Status    string
	Email     string
	Phone     string
	Nickname  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type TokenClaims struct {
	UserID    string `json:"user_id"`
	Type      string `json:"type"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
	Nonce     string `json:"nonce"`
}

type Service struct {
	mu            sync.RWMutex
	secret        []byte
	users         map[string]User
	refreshTokens map[string]string
	now           func() time.Time
}

func NewService(secret string) *Service {
	if strings.TrimSpace(secret) == "" {
		secret = "oops-reader-mvp-development-secret"
	}
	return &Service{
		secret:        []byte(secret),
		users:         make(map[string]User),
		refreshTokens: make(map[string]string),
		now:           time.Now,
	}
}

func (s *Service) CreateGuest(deviceID string) (User, string, string, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return User{}, "", "", fmt.Errorf("%w: device ID is required", ErrInvalidInput)
	}

	now := s.now().UTC()
	user := User{
		ID:        newID("usr"),
		DeviceID:  deviceID,
		Status:    UserStatusGuest,
		CreatedAt: now,
		UpdatedAt: now,
	}

	accessToken, refreshToken, err := s.issueTokens(user.ID, now)
	if err != nil {
		return User{}, "", "", err
	}

	s.mu.Lock()
	s.users[user.ID] = user
	s.refreshTokens[refreshToken] = user.ID
	s.mu.Unlock()

	return user, accessToken, refreshToken, nil
}

func (s *Service) Refresh(refreshToken string) (string, string, error) {
	claims, err := s.ValidateToken(refreshToken, TokenTypeRefresh)
	if err != nil {
		return "", "", err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	userID, ok := s.refreshTokens[refreshToken]
	if !ok || userID != claims.UserID {
		return "", "", ErrRefreshRevoked
	}
	if _, ok := s.users[claims.UserID]; !ok {
		return "", "", ErrUserNotFound
	}

	delete(s.refreshTokens, refreshToken)
	accessToken, newRefreshToken, err := s.issueTokens(claims.UserID, s.now().UTC())
	if err != nil {
		return "", "", err
	}
	s.refreshTokens[newRefreshToken] = claims.UserID

	return accessToken, newRefreshToken, nil
}

func (s *Service) Bind(userID, email, phone, nickname string) (User, error) {
	userID = strings.TrimSpace(userID)
	email = strings.TrimSpace(email)
	phone = strings.TrimSpace(phone)
	nickname = strings.TrimSpace(nickname)
	if userID == "" {
		return User{}, fmt.Errorf("%w: user ID is required", ErrInvalidInput)
	}
	if email == "" && phone == "" {
		return User{}, fmt.Errorf("%w: email or phone is required", ErrInvalidInput)
	}
	if email != "" && !strings.Contains(email, "@") {
		return User{}, fmt.Errorf("%w: email is invalid", ErrInvalidInput)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok := s.users[userID]
	if !ok {
		return User{}, ErrUserNotFound
	}
	user.Email = email
	user.Phone = phone
	user.Nickname = nickname
	user.Status = UserStatusRegistered
	user.UpdatedAt = s.now().UTC()
	s.users[userID] = user

	return user, nil
}

func (s *Service) GetUser(userID string) (User, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return User{}, fmt.Errorf("%w: user ID is required", ErrInvalidInput)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	user, ok := s.users[userID]
	if !ok {
		return User{}, ErrUserNotFound
	}
	return user, nil
}

func (s *Service) ValidateToken(token, expectedType string) (TokenClaims, error) {
	token = strings.TrimSpace(token)
	expectedType = strings.TrimSpace(expectedType)
	if token == "" || expectedType == "" {
		return TokenClaims{}, ErrInvalidToken
	}

	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return TokenClaims{}, ErrInvalidToken
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return TokenClaims{}, ErrInvalidToken
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return TokenClaims{}, ErrInvalidToken
	}
	if !hmac.Equal(signature, s.sign(payload)) {
		return TokenClaims{}, ErrInvalidToken
	}

	var claims TokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return TokenClaims{}, ErrInvalidToken
	}
	if claims.UserID == "" || claims.Type != expectedType {
		return TokenClaims{}, ErrInvalidToken
	}
	if s.now().UTC().Unix() > claims.ExpiresAt {
		return TokenClaims{}, ErrExpiredToken
	}
	return claims, nil
}

func (s *Service) issueTokens(userID string, now time.Time) (string, string, error) {
	accessToken, err := s.issueToken(userID, TokenTypeAccess, now, 15*time.Minute)
	if err != nil {
		return "", "", err
	}
	refreshToken, err := s.issueToken(userID, TokenTypeRefresh, now, 7*24*time.Hour)
	if err != nil {
		return "", "", err
	}
	return accessToken, refreshToken, nil
}

func (s *Service) issueToken(userID, tokenType string, now time.Time, ttl time.Duration) (string, error) {
	claims := TokenClaims{
		UserID:    userID,
		Type:      tokenType,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(ttl).Unix(),
		Nonce:     newID("tok"),
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(s.sign(payload)), nil
}

func (s *Service) sign(payload []byte) []byte {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write(payload)
	return mac.Sum(nil)
}

func newID(prefix string) string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
	}
	return fmt.Sprintf("%s_%x", prefix, b[:])
}
