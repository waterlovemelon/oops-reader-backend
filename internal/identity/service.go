package identity

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"

	accessTokenTTL        = 15 * time.Minute
	refreshTokenTTL       = 30 * 24 * time.Hour
	passwordResetTokenTTL = 30 * time.Minute
)

var (
	ErrInvalidInput       = errors.New("invalid input")
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrAccountForbidden   = errors.New("account forbidden")
	ErrInvalidToken       = errors.New("invalid token")
	ErrExpiredToken       = errors.New("expired token")
	ErrRefreshRevoked     = errors.New("refresh token revoked")
)

type TokenClaims struct {
	UserID    uint64 `json:"user_id"`
	Type      string `json:"type"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
	Nonce     string `json:"nonce"`
}

type RegisterInput struct {
	Email      string
	Password   string
	Nickname   string
	DeviceID   string
	DeviceName string
	Platform   string
}

type LoginInput struct {
	Email      string
	Password   string
	DeviceID   string
	DeviceName string
	Platform   string
}

type AuthResult struct {
	User         Account
	AccessToken  string
	RefreshToken string
	TokenType    string
}

type Service struct {
	store                  Store
	secret                 []byte
	now                    func() time.Time
	sendPasswordResetEmail func(email, token string) error
}

func NewService(store Store, secret string) *Service {
	if strings.TrimSpace(secret) == "" {
		secret = "oops-reader-mvp-development-secret"
	}
	return &Service{
		store:  store,
		secret: []byte(secret),
		now:    time.Now,
	}
}

func (s *Service) Register(ctx context.Context, input RegisterInput) (AuthResult, error) {
	email := normalizeEmail(input.Email)
	password := strings.TrimSpace(input.Password)
	deviceID := strings.TrimSpace(input.DeviceID)
	platform := strings.TrimSpace(input.Platform)
	if email == "" || !strings.Contains(email, "@") {
		return AuthResult{}, fmt.Errorf("%w: valid email is required", ErrInvalidInput)
	}
	if len(password) < 8 {
		return AuthResult{}, fmt.Errorf("%w: password must be at least 8 characters", ErrInvalidInput)
	}
	if deviceID == "" || platform == "" {
		return AuthResult{}, fmt.Errorf("%w: device ID and platform are required", ErrInvalidInput)
	}

	passwordHash, err := HashPassword(password)
	if err != nil {
		return AuthResult{}, err
	}
	nickname := strings.TrimSpace(input.Nickname)
	if nickname == "" {
		nickname = defaultNickname(email)
	}
	account, err := s.store.CreateAccount(ctx, Account{
		Email:         email,
		PasswordHash:  passwordHash,
		Nickname:      nickname,
		AccountStatus: AccountStatusActive,
		AccountType:   AccountTypeNormal,
	})
	if err != nil {
		return AuthResult{}, err
	}
	return s.issueAuthResult(ctx, account, deviceID, strings.TrimSpace(input.DeviceName), platform)
}

func (s *Service) Login(ctx context.Context, input LoginInput) (AuthResult, error) {
	email := normalizeEmail(input.Email)
	deviceID := strings.TrimSpace(input.DeviceID)
	platform := strings.TrimSpace(input.Platform)
	if email == "" || strings.TrimSpace(input.Password) == "" || deviceID == "" || platform == "" {
		return AuthResult{}, fmt.Errorf("%w: email, password, device ID, and platform are required", ErrInvalidInput)
	}

	account, err := s.store.FindAccountByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return AuthResult{}, ErrInvalidCredentials
		}
		return AuthResult{}, err
	}
	if !VerifyPassword(account.PasswordHash, input.Password) {
		return AuthResult{}, ErrInvalidCredentials
	}
	if err := ensureAccountActive(account); err != nil {
		return AuthResult{}, err
	}

	now := s.now().UTC()
	if err := s.store.UpdateLastLogin(ctx, account.ID, now); err != nil {
		return AuthResult{}, err
	}
	account.LastLoginAt.Valid = true
	account.LastLoginAt.Time = now

	return s.issueAuthResult(ctx, account, deviceID, strings.TrimSpace(input.DeviceName), platform)
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (AuthResult, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return AuthResult{}, ErrInvalidToken
	}

	session, err := s.store.FindSessionByRefreshHash(ctx, HashToken(refreshToken))
	if err != nil {
		return AuthResult{}, err
	}
	now := s.now().UTC()
	if session.RevokedAt.Valid {
		return AuthResult{}, ErrRefreshRevoked
	}
	if now.After(session.ExpiresAt) {
		return AuthResult{}, ErrExpiredToken
	}

	account, err := s.store.FindAccountByID(ctx, session.UserID)
	if err != nil {
		return AuthResult{}, err
	}
	if err := ensureAccountActive(account); err != nil {
		return AuthResult{}, err
	}
	if err := s.store.RevokeSession(ctx, session.ID, now); err != nil {
		return AuthResult{}, err
	}
	return s.issueAuthResult(ctx, account, session.DeviceID, session.DeviceName, session.Platform)
}

func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil
	}
	session, err := s.store.FindSessionByRefreshHash(ctx, HashToken(refreshToken))
	if err != nil {
		if errors.Is(err, ErrRefreshRevoked) {
			return nil
		}
		return err
	}
	return s.store.RevokeSession(ctx, session.ID, s.now().UTC())
}

func (s *Service) RequestPasswordReset(ctx context.Context, email string) error {
	email = normalizeEmail(email)
	if email == "" {
		return nil
	}
	account, err := s.store.FindAccountByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil
		}
		return err
	}
	if err := ensureAccountActive(account); err != nil {
		return nil
	}

	token, err := NewOpaqueToken()
	if err != nil {
		return err
	}
	if _, err := s.store.CreatePasswordResetToken(ctx, PasswordResetToken{
		UserID:    account.ID,
		Email:     account.Email,
		TokenHash: HashToken(token),
		ExpiresAt: s.now().UTC().Add(passwordResetTokenTTL),
	}); err != nil {
		return err
	}
	if s.sendPasswordResetEmail != nil {
		return s.sendPasswordResetEmail(account.Email, token)
	}
	return nil
}

func (s *Service) ConfirmPasswordReset(ctx context.Context, token, newPassword string) error {
	token = strings.TrimSpace(token)
	newPassword = strings.TrimSpace(newPassword)
	if token == "" || len(newPassword) < 8 {
		return ErrInvalidInput
	}

	resetToken, err := s.store.FindPasswordResetToken(ctx, HashToken(token))
	if err != nil {
		return err
	}
	now := s.now().UTC()
	if resetToken.UsedAt.Valid {
		return ErrInvalidToken
	}
	if now.After(resetToken.ExpiresAt) {
		return ErrExpiredToken
	}

	passwordHash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	if err := s.store.UpdatePasswordHash(ctx, resetToken.UserID, passwordHash); err != nil {
		return err
	}
	if err := s.store.MarkPasswordResetTokenUsed(ctx, resetToken.ID, now); err != nil {
		return err
	}
	return s.store.RevokeUserSessions(ctx, resetToken.UserID, now)
}

func (s *Service) GetUser(ctx context.Context, userID uint64) (Account, error) {
	if userID == 0 {
		return Account{}, fmt.Errorf("%w: user ID is required", ErrInvalidInput)
	}
	account, err := s.store.FindAccountByID(ctx, userID)
	if err != nil {
		return Account{}, err
	}
	if err := ensureAccountActive(account); err != nil {
		return Account{}, err
	}
	return account, nil
}

func (s *Service) UpdateProfile(ctx context.Context, userID uint64, nickname, avatarURL string) (Account, error) {
	nickname = strings.TrimSpace(nickname)
	avatarURL = strings.TrimSpace(avatarURL)
	if userID == 0 || nickname == "" {
		return Account{}, fmt.Errorf("%w: user ID and nickname are required", ErrInvalidInput)
	}
	return s.store.UpdateProfile(ctx, userID, nickname, avatarURL)
}

func (s *Service) ValidateAccessToken(ctx context.Context, token string) (TokenClaims, error) {
	claims, err := s.ValidateToken(token, TokenTypeAccess)
	if err != nil {
		return TokenClaims{}, err
	}
	account, err := s.store.FindAccountByID(ctx, claims.UserID)
	if err != nil {
		return TokenClaims{}, err
	}
	if err := ensureAccountActive(account); err != nil {
		return TokenClaims{}, err
	}
	return claims, nil
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
	if claims.UserID == 0 || claims.Type != expectedType {
		return TokenClaims{}, ErrInvalidToken
	}
	if s.now().UTC().Unix() > claims.ExpiresAt {
		return TokenClaims{}, ErrExpiredToken
	}
	return claims, nil
}

func (s *Service) issueAuthResult(ctx context.Context, account Account, deviceID, deviceName, platform string) (AuthResult, error) {
	now := s.now().UTC()
	accessToken, err := s.issueAccessToken(account.ID, now)
	if err != nil {
		return AuthResult{}, err
	}
	refreshToken, err := NewOpaqueToken()
	if err != nil {
		return AuthResult{}, err
	}
	if _, err := s.store.CreateSession(ctx, Session{
		UserID:           account.ID,
		DeviceID:         deviceID,
		DeviceName:       deviceName,
		Platform:         platform,
		RefreshTokenHash: HashToken(refreshToken),
		ExpiresAt:        now.Add(refreshTokenTTL),
		LastActiveAt:     now,
	}); err != nil {
		return AuthResult{}, err
	}
	return AuthResult{
		User:         account,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
	}, nil
}

func (s *Service) issueAccessToken(userID uint64, now time.Time) (string, error) {
	nonce, err := NewOpaqueToken()
	if err != nil {
		return "", err
	}
	claims := TokenClaims{
		UserID:    userID,
		Type:      TokenTypeAccess,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(accessTokenTTL).Unix(),
		Nonce:     nonce,
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

func ensureAccountActive(account Account) error {
	if account.AccountStatus != "" && account.AccountStatus != AccountStatusActive {
		return ErrAccountForbidden
	}
	return nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func defaultNickname(email string) string {
	local, _, ok := strings.Cut(email, "@")
	if !ok || strings.TrimSpace(local) == "" {
		return "Reader"
	}
	return local
}
