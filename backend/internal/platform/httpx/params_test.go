package httpx

import "testing"

func TestIsUUID(t *testing.T) {
	good := []string{"2b2fa9fb-15b8-4d4e-9765-3049fbb8a3cf", "2B2FA9FB-15B8-4D4E-9765-3049FBB8A3CF"}
	bad := []string{"", "1", "2b2fa9fb15b84d4e97653049fbb8a3cf", "2b2fa9fb-15b8-4d4e-9765-3049fbb8a3cg", "2b2fa9fb-15b8-4d4e-9765_3049fbb8a3cf"}
	for _, s := range good {
		if !IsUUID(s) {
			t.Errorf("IsUUID(%q) = false", s)
		}
	}
	for _, s := range bad {
		if IsUUID(s) {
			t.Errorf("IsUUID(%q) = true", s)
		}
	}
}
