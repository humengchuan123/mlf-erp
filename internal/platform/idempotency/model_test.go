package idempotency

import (
	"strings"
	"testing"
)

func TestValidateKey(t *testing.T) {
	valid := []string{
		"550e8400-e29b-41d4-a716-446655440000",
		"order.retry:1_A-B",
		"abcdefgh",
		strings.Repeat("a", 128),
		"  abcdefgh  ",
	}
	for _, key := range valid {
		if err := ValidateKey(key); err != nil {
			t.Errorf("ValidateKey(%q) = %v, want nil", key, err)
		}
	}

	invalid := []string{
		"",
		"short",
		"abcdefg",
		strings.Repeat("a", 129),
		"has space",
		"中文中文中文中文",
		"with/slash",
		"tab\tkey",
	}
	for _, key := range invalid {
		if err := ValidateKey(key); err == nil {
			t.Errorf("ValidateKey(%q) = nil, want error", key)
		}
	}
}

func TestNormalizeKey(t *testing.T) {
	if got := NormalizeKey("  key-123456  "); got != "key-123456" {
		t.Fatalf("NormalizeKey() = %q, want %q", got, "key-123456")
	}
}

func TestRequestHash(t *testing.T) {
	const expected = "34b6bb45857eab51a9de8e85678e3ab1856006e3d7fdc0b8960c11caa51fe745"

	hash := RequestHash("POST", "/test", []byte(`{"amount":"10"}`))
	if hash != expected {
		t.Fatalf("RequestHash() = %q, want %q", hash, expected)
	}
	if again := RequestHash("POST", "/test", []byte(`{"amount":"10"}`)); again != hash {
		t.Fatalf("RequestHash() is not stable: %q != %q", again, hash)
	}
	if len(hash) != 64 {
		t.Fatalf("RequestHash() length = %d, want 64", len(hash))
	}

	cases := []struct {
		method string
		path   string
		body   string
	}{
		{"PUT", "/test", `{"amount":"10"}`},
		{"POST", "/other", `{"amount":"10"}`},
		{"POST", "/test", `{"amount":"11"}`},
		{"POST", "/test", `{"amount":"10"} `},
	}
	for _, tc := range cases {
		if got := RequestHash(tc.method, tc.path, []byte(tc.body)); got == hash {
			t.Errorf("RequestHash(%q, %q, %q) unexpectedly matched base hash", tc.method, tc.path, tc.body)
		}
	}
}
