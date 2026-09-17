package device

import (
	"context"
	"time"
)

// noopStore is a Store used when the database is not available, mirroring the
// catalog noop stores. The router builds a device service either way, and
// device bookkeeping must never fail an authentication request that the
// in-memory identity store could otherwise serve.
type noopStore struct{}

// NewNoopStore creates a Store that accepts touches without persisting them.
func NewNoopStore() Store {
	return &noopStore{}
}

func (s *noopStore) TouchDevice(_ context.Context, _ uint64, info Info) (Device, error) {
	now := time.Now()
	return Device{
		DeviceID:    info.DeviceID,
		DeviceName:  info.DeviceName,
		Platform:    info.Platform,
		FirstSeenAt: now,
		LastSeenAt:  now,
	}, nil
}

func (s *noopStore) ListDevices(_ context.Context, _ uint64) ([]Device, error) {
	return []Device{}, nil
}
