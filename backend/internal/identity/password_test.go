package identity

import "testing"

func TestValidatePassword(t *testing.T) {
	cases := map[string]bool{
		"short1":                 false,
		"onlyletters":            false,
		"12345678":               false,
		"goodpass1":              true,
		string(make([]byte, 73)): false,
	}
	for pw, ok := range cases {
		if err := ValidatePassword(pw); (err == nil) != ok {
			t.Errorf("ValidatePassword(%q) err=%v, want ok=%t", pw, err, ok)
		}
	}
}

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("correct1horse")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(hash, "correct1horse") {
		t.Fatal("expected password to match")
	}
	if CheckPassword(hash, "wrong1horse") {
		t.Fatal("expected mismatch")
	}
}
