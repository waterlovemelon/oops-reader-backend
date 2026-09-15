package device

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// MySQLStore implements Store on top of user_devices.
type MySQLStore struct {
	db *sql.DB
}

// NewMySQLStore creates a MySQLStore.
func NewMySQLStore(db *sql.DB) *MySQLStore {
	return &MySQLStore{db: db}
}

// TouchDevice inserts or refreshes the device row. Device name and platform keep
// their stored value when the caller passes none, so activity-only touches
// cannot wipe the metadata captured at login.
func (s *MySQLStore) TouchDevice(ctx context.Context, userID uint64, info Info) (Device, error) {
	if _, err := s.db.ExecContext(ctx, `
INSERT INTO user_devices (user_id, device_id, device_name, platform, first_seen_at, last_seen_at)
VALUES (?, ?, ?, ?, NOW(), NOW())
ON DUPLICATE KEY UPDATE
	device_name = COALESCE(VALUES(device_name), device_name),
	platform = COALESCE(VALUES(platform), platform),
	last_seen_at = NOW()`,
		userID, info.DeviceID, nullText(info.DeviceName), nullText(info.Platform),
	); err != nil {
		return Device{}, fmt.Errorf("touch device: %w", err)
	}

	device, err := scanDevice(s.db.QueryRowContext(ctx, `
SELECT id, device_id, device_name, platform, first_seen_at, last_seen_at
FROM user_devices
WHERE user_id = ? AND device_id = ?`, userID, info.DeviceID))
	if errors.Is(err, sql.ErrNoRows) {
		return Device{}, fmt.Errorf("touch device: row missing after upsert")
	}
	if err != nil {
		return Device{}, fmt.Errorf("touch device: %w", err)
	}
	return device, nil
}

// ListDevices returns the account's devices, most recently active first.
func (s *MySQLStore) ListDevices(ctx context.Context, userID uint64) ([]Device, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, device_id, device_name, platform, first_seen_at, last_seen_at
FROM user_devices
WHERE user_id = ?
ORDER BY last_seen_at DESC, id DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list devices: %w", err)
	}
	defer rows.Close()

	devices := make([]Device, 0, 4)
	for rows.Next() {
		device, err := scanDevice(rows)
		if err != nil {
			return nil, fmt.Errorf("scan device: %w", err)
		}
		devices = append(devices, device)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list devices: %w", err)
	}
	return devices, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanDevice(scanner rowScanner) (Device, error) {
	var device Device
	var name, platform sql.NullString
	if err := scanner.Scan(
		&device.ID,
		&device.DeviceID,
		&name,
		&platform,
		&device.FirstSeenAt,
		&device.LastSeenAt,
	); err != nil {
		return Device{}, err
	}
	device.DeviceName = name.String
	device.Platform = platform.String
	return device, nil
}

func nullText(value string) any {
	if value == "" {
		return nil
	}
	return value
}
