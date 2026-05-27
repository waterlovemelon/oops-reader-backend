package entitlement

import "testing"

func TestDefaultNormalEntitlements(t *testing.T) {
	service := NewService(nil)
	result, err := service.ListForUser(123, "normal")
	if err != nil {
		t.Fatalf("ListForUser returned error: %v", err)
	}
	if result.AccountType != "normal" {
		t.Fatalf("AccountType = %q, want normal", result.AccountType)
	}
	if !result.Has("backup.reading_data") {
		t.Fatal("normal account missing backup.reading_data entitlement")
	}
	if result.Has("sync.multi_device") {
		t.Fatal("normal account unexpectedly has sync.multi_device")
	}
}
