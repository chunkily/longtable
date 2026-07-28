package auth

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	if !VerifyPassword(hash, "correct horse battery staple") {
		t.Fatal("VerifyPassword: correct password did not verify")
	}
	if VerifyPassword(hash, "wrong password") {
		t.Fatal("VerifyPassword: incorrect password verified")
	}
}

func TestNewToken(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 20; i++ {
		token, err := NewToken()
		if err != nil {
			t.Fatalf("NewToken: %v", err)
		}
		if token == "" {
			t.Fatal("expected a non-empty token")
		}
		if seen[token] {
			t.Fatalf("duplicate token generated: %q", token)
		}
		seen[token] = true
	}
}
