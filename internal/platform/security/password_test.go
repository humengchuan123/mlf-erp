package security

import "testing"

func TestPasswordHashAndCheck(t *testing.T) {
	hash, err := HashPassword("Admin@123456")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if hash == "Admin@123456" {
		t.Fatal("password hash must not equal the plain password")
	}
	if !CheckPassword(hash, "Admin@123456") {
		t.Fatal("CheckPassword() = false, want true")
	}
	if CheckPassword(hash, "wrong-password") {
		t.Fatal("CheckPassword() = true for wrong password")
	}
}
