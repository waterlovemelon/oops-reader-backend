package identity

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/go-sql-driver/mysql"
)

const (
	AccountStatusActive      = "active"
	AccountStatusFrozen      = "frozen"
	AccountStatusDeactivated = "deactivated"
	AccountTypeNormal        = "normal"
)

var ErrDuplicateEmail = errors.New("duplicate email")

type Account struct {
	ID              uint64
	Email           string
	Phone           string
	PasswordHash    string
	Nickname        string
	AvatarURL       string
	AccountStatus   string
	AccountType     string
	EmailVerifiedAt sql.NullTime
	LastLoginAt     sql.NullTime
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Session struct {
	ID               uint64
	UserID           uint64
	DeviceID         string
	DeviceName       string
	Platform         string
	RefreshTokenHash string
	ExpiresAt        time.Time
	RevokedAt        sql.NullTime
	LastActiveAt     time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type PasswordResetToken struct {
	ID        uint64
	UserID    uint64
	Email     string
	TokenHash string
	ExpiresAt time.Time
	UsedAt    sql.NullTime
	CreatedAt time.Time
}

type Store interface {
	CreateAccount(ctx context.Context, account Account) (Account, error)
	FindAccountByEmail(ctx context.Context, email string) (Account, error)
	FindAccountByID(ctx context.Context, userID uint64) (Account, error)
	UpdateLastLogin(ctx context.Context, userID uint64, at time.Time) error
	UpdateProfile(ctx context.Context, userID uint64, nickname, avatarURL string) (Account, error)
	UpdatePasswordHash(ctx context.Context, userID uint64, passwordHash string) error
	CreateSession(ctx context.Context, session Session) (Session, error)
	FindSessionByRefreshHash(ctx context.Context, refreshHash string) (Session, error)
	RevokeSession(ctx context.Context, sessionID uint64, at time.Time) error
	RevokeUserSessions(ctx context.Context, userID uint64, at time.Time) error
	CreatePasswordResetToken(ctx context.Context, token PasswordResetToken) (PasswordResetToken, error)
	FindPasswordResetToken(ctx context.Context, tokenHash string) (PasswordResetToken, error)
	MarkPasswordResetTokenUsed(ctx context.Context, tokenID uint64, at time.Time) error
}

type MySQLStore struct {
	db *sql.DB
}

func NewMySQLStore(db *sql.DB) *MySQLStore {
	return &MySQLStore{db: db}
}

func (s *MySQLStore) CreateAccount(ctx context.Context, account Account) (Account, error) {
	if account.AccountStatus == "" {
		account.AccountStatus = AccountStatusActive
	}
	if account.AccountType == "" {
		account.AccountType = AccountTypeNormal
	}

	result, err := s.db.ExecContext(ctx, `
		INSERT INTO users (
			email, phone, password_hash, nickname, avatar_url,
			account_status, account_type, email_verified_at, last_login_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, nullableString(account.Email), nullableString(account.Phone), nullableString(account.PasswordHash),
		account.Nickname, nullableString(account.AvatarURL), account.AccountStatus, account.AccountType,
		account.EmailVerifiedAt, account.LastLoginAt)
	if err != nil {
		if isDuplicateKey(err) {
			return Account{}, ErrDuplicateEmail
		}
		return Account{}, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return Account{}, err
	}
	return s.FindAccountByID(ctx, uint64(id))
}

func (s *MySQLStore) FindAccountByEmail(ctx context.Context, email string) (Account, error) {
	return s.scanAccount(s.db.QueryRowContext(ctx, accountSelectSQL()+" WHERE email = ?", email))
}

func (s *MySQLStore) FindAccountByID(ctx context.Context, userID uint64) (Account, error) {
	return s.scanAccount(s.db.QueryRowContext(ctx, accountSelectSQL()+" WHERE id = ?", userID))
}

func (s *MySQLStore) UpdateLastLogin(ctx context.Context, userID uint64, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET last_login_at = ? WHERE id = ?`, at, userID)
	return err
}

func (s *MySQLStore) UpdateProfile(ctx context.Context, userID uint64, nickname, avatarURL string) (Account, error) {
	_, err := s.db.ExecContext(ctx, `
		UPDATE users
		SET nickname = ?, avatar_url = ?
		WHERE id = ?
	`, nickname, nullableString(avatarURL), userID)
	if err != nil {
		return Account{}, err
	}
	return s.FindAccountByID(ctx, userID)
}

func (s *MySQLStore) UpdatePasswordHash(ctx context.Context, userID uint64, passwordHash string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, passwordHash, userID)
	return err
}

func (s *MySQLStore) CreateSession(ctx context.Context, session Session) (Session, error) {
	if session.LastActiveAt.IsZero() {
		session.LastActiveAt = time.Now().UTC()
	}

	result, err := s.db.ExecContext(ctx, `
		INSERT INTO user_sessions (
			user_id, device_id, device_name, platform, refresh_token_hash,
			expires_at, revoked_at, last_active_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, session.UserID, session.DeviceID, nullableString(session.DeviceName), session.Platform,
		session.RefreshTokenHash, session.ExpiresAt, session.RevokedAt, session.LastActiveAt)
	if err != nil {
		return Session{}, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return Session{}, err
	}
	return s.findSessionByID(ctx, uint64(id))
}

func (s *MySQLStore) FindSessionByRefreshHash(ctx context.Context, refreshHash string) (Session, error) {
	return s.scanSession(s.db.QueryRowContext(ctx, sessionSelectSQL()+" WHERE refresh_token_hash = ?", refreshHash))
}

func (s *MySQLStore) RevokeSession(ctx context.Context, sessionID uint64, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE user_sessions SET revoked_at = ? WHERE id = ?`, at, sessionID)
	return err
}

func (s *MySQLStore) RevokeUserSessions(ctx context.Context, userID uint64, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE user_sessions SET revoked_at = ? WHERE user_id = ? AND revoked_at IS NULL`, at, userID)
	return err
}

func (s *MySQLStore) CreatePasswordResetToken(ctx context.Context, token PasswordResetToken) (PasswordResetToken, error) {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO password_reset_tokens (
			user_id, email, token_hash, expires_at, used_at
		) VALUES (?, ?, ?, ?, ?)
	`, token.UserID, token.Email, token.TokenHash, token.ExpiresAt, token.UsedAt)
	if err != nil {
		return PasswordResetToken{}, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return PasswordResetToken{}, err
	}
	return s.findPasswordResetTokenByID(ctx, uint64(id))
}

func (s *MySQLStore) FindPasswordResetToken(ctx context.Context, tokenHash string) (PasswordResetToken, error) {
	return s.scanPasswordResetToken(s.db.QueryRowContext(ctx, passwordResetTokenSelectSQL()+" WHERE token_hash = ?", tokenHash))
}

func (s *MySQLStore) MarkPasswordResetTokenUsed(ctx context.Context, tokenID uint64, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE password_reset_tokens SET used_at = ? WHERE id = ?`, at, tokenID)
	return err
}

func (s *MySQLStore) findSessionByID(ctx context.Context, sessionID uint64) (Session, error) {
	return s.scanSession(s.db.QueryRowContext(ctx, sessionSelectSQL()+" WHERE id = ?", sessionID))
}

func (s *MySQLStore) findPasswordResetTokenByID(ctx context.Context, tokenID uint64) (PasswordResetToken, error) {
	return s.scanPasswordResetToken(s.db.QueryRowContext(ctx, passwordResetTokenSelectSQL()+" WHERE id = ?", tokenID))
}

func (s *MySQLStore) scanAccount(row *sql.Row) (Account, error) {
	var account Account
	var email, phone, passwordHash, avatarURL sql.NullString
	if err := row.Scan(
		&account.ID,
		&email,
		&phone,
		&passwordHash,
		&account.Nickname,
		&avatarURL,
		&account.AccountStatus,
		&account.AccountType,
		&account.EmailVerifiedAt,
		&account.LastLoginAt,
		&account.CreatedAt,
		&account.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Account{}, ErrUserNotFound
		}
		return Account{}, err
	}
	account.Email = email.String
	account.Phone = phone.String
	account.PasswordHash = passwordHash.String
	account.AvatarURL = avatarURL.String
	return account, nil
}

func (s *MySQLStore) scanSession(row *sql.Row) (Session, error) {
	var session Session
	var deviceName sql.NullString
	if err := row.Scan(
		&session.ID,
		&session.UserID,
		&session.DeviceID,
		&deviceName,
		&session.Platform,
		&session.RefreshTokenHash,
		&session.ExpiresAt,
		&session.RevokedAt,
		&session.LastActiveAt,
		&session.CreatedAt,
		&session.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Session{}, ErrRefreshRevoked
		}
		return Session{}, err
	}
	session.DeviceName = deviceName.String
	return session, nil
}

func (s *MySQLStore) scanPasswordResetToken(row *sql.Row) (PasswordResetToken, error) {
	var token PasswordResetToken
	if err := row.Scan(
		&token.ID,
		&token.UserID,
		&token.Email,
		&token.TokenHash,
		&token.ExpiresAt,
		&token.UsedAt,
		&token.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PasswordResetToken{}, ErrInvalidToken
		}
		return PasswordResetToken{}, err
	}
	return token, nil
}

func accountSelectSQL() string {
	return `
		SELECT
			id, email, phone, password_hash, nickname, avatar_url,
			account_status, account_type, email_verified_at, last_login_at,
			created_at, updated_at
		FROM users`
}

func sessionSelectSQL() string {
	return `
		SELECT
			id, user_id, device_id, device_name, platform, refresh_token_hash,
			expires_at, revoked_at, last_active_at, created_at, updated_at
		FROM user_sessions`
}

func passwordResetTokenSelectSQL() string {
	return `
		SELECT
			id, user_id, email, token_hash, expires_at, used_at, created_at
		FROM password_reset_tokens`
}

func nullableString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

func isDuplicateKey(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
