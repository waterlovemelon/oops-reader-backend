package device

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeDeviceStore struct {
	rows    map[string]Device
	touched []Info
}

func newFakeDeviceStore() *fakeDeviceStore {
	return &fakeDeviceStore{rows: map[string]Device{}}
}

func (f *fakeDeviceStore) TouchDevice(_ context.Context, userID uint64, info Info) (Device, error) {
	f.touched = append(f.touched, info)
	existing, ok := f.rows[info.DeviceID]
	if !ok {
		existing = Device{ID: uint64(len(f.rows) + 1), FirstSeenAt: time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)}
	}
	existing.DeviceID = info.DeviceID
	if info.DeviceName != "" {
		existing.DeviceName = info.DeviceName
	}
	if info.Platform != "" {
		existing.Platform = info.Platform
	}
	existing.LastSeenAt = time.Date(2026, 9, 14, 11, 0, 0, 0, time.UTC)
	f.rows[info.DeviceID] = existing
	return existing, nil
}

func (f *fakeDeviceStore) ListDevices(_ context.Context, _ uint64) ([]Device, error) {
	devices := make([]Device, 0, len(f.rows))
	for _, item := range f.rows {
		devices = append(devices, item)
	}
	return devices, nil
}

func TestTouchNormalizesDeviceInfo(t *testing.T) {
	store := newFakeDeviceStore()
	service := NewService(store)

	device, err := service.Touch(context.Background(), 7, Info{
		DeviceID:   "  flutter-1  ",
		DeviceName: "  Pixel 8  ",
		Platform:   "  ANDROID ",
	})
	if err != nil {
		t.Fatalf("Touch() error = %v", err)
	}
	if device.DeviceID != "flutter-1" {
		t.Fatalf("device_id = %q, want flutter-1", device.DeviceID)
	}
	if device.DeviceName != "Pixel 8" {
		t.Fatalf("device_name = %q, want Pixel 8", device.DeviceName)
	}
	if device.Platform != "android" {
		t.Fatalf("platform = %q, want android", device.Platform)
	}
	if len(store.touched) != 1 || store.touched[0].DeviceID != "flutter-1" {
		t.Fatalf("store received %#v", store.touched)
	}
}

func TestTouchKeepsEarlierMetadataWhenActivityOnlyReportsAnID(t *testing.T) {
	store := newFakeDeviceStore()
	service := NewService(store)

	if _, err := service.Touch(context.Background(), 7, Info{
		DeviceID:   "flutter-1",
		DeviceName: "Pixel 8",
		Platform:   "android",
	}); err != nil {
		t.Fatalf("first Touch() error = %v", err)
	}
	// 进度上报只带 device_id,不能抹掉登录时登记的名称与平台。
	device, err := service.Touch(context.Background(), 7, Info{DeviceID: "flutter-1"})
	if err != nil {
		t.Fatalf("second Touch() error = %v", err)
	}
	if device.DeviceName != "Pixel 8" || device.Platform != "android" {
		t.Fatalf("device = %#v, want the metadata from login", device)
	}
}

func TestTouchRejectsInvalidDeviceInfo(t *testing.T) {
	tests := []struct {
		name string
		info Info
		want error
	}{
		{"empty device id", Info{}, ErrInvalidDeviceID},
		{"blank device id", Info{DeviceID: "   "}, ErrInvalidDeviceID},
		{"device id over 128 characters", Info{DeviceID: strings.Repeat("d", 129)}, ErrInvalidDeviceID},
		{"device id at 128 characters", Info{DeviceID: strings.Repeat("d", 128)}, nil},
		{"device name over 128 characters", Info{DeviceID: "d", DeviceName: strings.Repeat("n", 129)}, ErrInvalidDeviceName},
		{"platform over 32 characters", Info{DeviceID: "d", Platform: strings.Repeat("p", 33)}, ErrInvalidPlatform},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := newFakeDeviceStore()
			service := NewService(store)

			_, err := service.Touch(context.Background(), 7, test.info)
			if test.want == nil {
				if err != nil {
					t.Fatalf("Touch() error = %v, want success", err)
				}
				return
			}
			if !errors.Is(err, test.want) {
				t.Fatalf("Touch() error = %v, want %v", err, test.want)
			}
			if len(store.touched) != 0 {
				t.Fatalf("invalid info reached the store: %#v", store.touched)
			}
		})
	}
}

func TestTouchFuncRegistersTheDevice(t *testing.T) {
	store := newFakeDeviceStore()
	service := NewService(store)

	if err := service.TouchFunc()(context.Background(), 7, Info{DeviceID: "flutter-2"}); err != nil {
		t.Fatalf("TouchFunc() error = %v", err)
	}
	if len(store.touched) != 1 || store.touched[0].DeviceID != "flutter-2" {
		t.Fatalf("store received %#v", store.touched)
	}
}
