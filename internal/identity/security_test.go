package identity

import "testing"

func TestPasswordHashAndVerify(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if hash == "" || hash == "correct horse battery staple" {
		t.Fatalf("HashPassword returned unsafe hash %q", hash)
	}
	if !VerifyPassword(hash, "correct horse battery staple") {
		t.Fatal("VerifyPassword rejected the original password")
	}
	if VerifyPassword(hash, "wrong password") {
		t.Fatal("VerifyPassword accepted the wrong password")
	}
}

func TestRandomTokenAndTokenHash(t *testing.T) {
	token, err := NewOpaqueToken()
	if err != nil {
		t.Fatalf("NewOpaqueToken returned error: %v", err)
	}
	if len(token) < 32 {
		t.Fatalf("token length = %d, want at least 32", len(token))
	}
	hash := HashToken(token)
	if hash == "" || hash == token {
		t.Fatalf("HashToken returned unsafe hash %q", hash)
	}
	if HashToken(token) != hash {
		t.Fatal("HashToken is not deterministic")
	}
}
