// Package device 记录账号使用过的设备,供展示设备清单与后续限制设备数量。
package device

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// 列边界,与 user_devices 表一致。
const (
	MaxDeviceIDLength   = 128
	MaxDeviceNameLength = 128
	MaxPlatformLength   = 32
)

var (
	ErrInvalidDeviceID   = errors.New("device_id is required and must be at most 128 characters")
	ErrInvalidDeviceName = errors.New("device_name must be at most 128 characters")
	ErrInvalidPlatform   = errors.New("platform must be at most 32 characters")
)

// Info is the caller-supplied device metadata. Name and platform may be empty
// when the device is only seen through an activity that does not carry them.
type Info struct {
	DeviceID   string
	DeviceName string
	Platform   string
}

// Device is a registered device of one account.
type Device struct {
	ID          uint64
	DeviceID    string
	DeviceName  string
	Platform    string
	FirstSeenAt time.Time
	LastSeenAt  time.Time
}

// Store persists devices.
type Store interface {
	TouchDevice(ctx context.Context, userID uint64, info Info) (Device, error)
	ListDevices(ctx context.Context, userID uint64) ([]Device, error)
}

// TouchFunc is the reduced signature other domains inject when they only need
// to register activity.
type TouchFunc func(ctx context.Context, userID uint64, info Info) error

// Service registers devices and lists them.
type Service struct {
	store Store
}

// NewService creates a Service backed by store.
func NewService(store Store) *Service {
	return &Service{store: store}
}

// Touch registers the device or refreshes its last_seen_at. Name and platform
// keep their previous value when the caller does not supply them.
func (s *Service) Touch(ctx context.Context, userID uint64, info Info) (Device, error) {
	normalized, err := normalize(info)
	if err != nil {
		return Device{}, err
	}
	return s.store.TouchDevice(ctx, userID, normalized)
}

// List returns the account's devices, most recently active first.
func (s *Service) List(ctx context.Context, userID uint64) ([]Device, error) {
	return s.store.ListDevices(ctx, userID)
}

// TouchFunc adapts Touch to the reduced signature used by other domains.
func (s *Service) TouchFunc() TouchFunc {
	return func(ctx context.Context, userID uint64, info Info) error {
		_, err := s.Touch(ctx, userID, info)
		return err
	}
}

func normalize(info Info) (Info, error) {
	deviceID := strings.TrimSpace(info.DeviceID)
	if deviceID == "" || utf8.RuneCountInString(deviceID) > MaxDeviceIDLength {
		return Info{}, ErrInvalidDeviceID
	}
	deviceName := strings.TrimSpace(info.DeviceName)
	if utf8.RuneCountInString(deviceName) > MaxDeviceNameLength {
		return Info{}, ErrInvalidDeviceName
	}
	platform := strings.ToLower(strings.TrimSpace(info.Platform))
	if utf8.RuneCountInString(platform) > MaxPlatformLength {
		return Info{}, ErrInvalidPlatform
	}
	return Info{DeviceID: deviceID, DeviceName: deviceName, Platform: platform}, nil
}
