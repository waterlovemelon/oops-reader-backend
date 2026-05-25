package identity

import "testing"

func TestGuestLifecycleAndRefresh(t *testing.T) {
	service := NewService("test-secret")

	user, accessToken, refreshToken, err := service.CreateGuest("device-1")
	if err != nil {
		t.Fatalf("CreateGuest returned error: %v", err)
	}
	if user.ID == "" {
		t.Fatal("CreateGuest returned empty user ID")
	}
	if user.DeviceID != "device-1" {
		t.Fatalf("DeviceID = %q, want device-1", user.DeviceID)
	}
	if user.Status != UserStatusGuest {
		t.Fatalf("Status = %q, want %q", user.Status, UserStatusGuest)
	}
	if accessToken == "" || refreshToken == "" {
		t.Fatal("CreateGuest returned empty tokens")
	}

	accessClaims, err := service.ValidateToken(accessToken, TokenTypeAccess)
	if err != nil {
		t.Fatalf("ValidateToken(access) returned error: %v", err)
	}
	if accessClaims.UserID != user.ID {
		t.Fatalf("access claims user ID = %q, want %q", accessClaims.UserID, user.ID)
	}

	newAccessToken, newRefreshToken, err := service.Refresh(refreshToken)
	if err != nil {
		t.Fatalf("Refresh returned error: %v", err)
	}
	if newAccessToken == "" || newRefreshToken == "" {
		t.Fatal("Refresh returned empty tokens")
	}
	if newAccessToken == accessToken {
		t.Fatal("Refresh returned same access token")
	}
	if _, _, err := service.Refresh(refreshToken); err == nil {
		t.Fatal("Refresh with rotated token succeeded, want error")
	}
}

func TestBindPreservesUserID(t *testing.T) {
	service := NewService("test-secret")
	user, _, _, err := service.CreateGuest("device-1")
	if err != nil {
		t.Fatalf("CreateGuest returned error: %v", err)
	}

	bound, err := service.Bind(user.ID, "reader@example.com", "+15551234567", "Reader")
	if err != nil {
		t.Fatalf("Bind returned error: %v", err)
	}
	if bound.ID != user.ID {
		t.Fatalf("bound user ID = %q, want %q", bound.ID, user.ID)
	}
	if bound.Status != UserStatusRegistered {
		t.Fatalf("Status = %q, want %q", bound.Status, UserStatusRegistered)
	}
	if bound.Email != "reader@example.com" || bound.Phone != "+15551234567" || bound.Nickname != "Reader" {
		t.Fatalf("Bind did not persist profile fields: %+v", bound)
	}

	fetched, err := service.GetUser(user.ID)
	if err != nil {
		t.Fatalf("GetUser returned error: %v", err)
	}
	if fetched.ID != user.ID || fetched.Status != UserStatusRegistered {
		t.Fatalf("GetUser returned %+v, want registered user %q", fetched, user.ID)
	}
}

func TestCreateGuestRequiresDeviceID(t *testing.T) {
	service := NewService("test-secret")
	if _, _, _, err := service.CreateGuest(" "); err == nil {
		t.Fatal("CreateGuest with blank device ID succeeded, want error")
	}
}
