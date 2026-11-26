package settings

import "testing"

func TestValidateIdleLock(t *testing.T) {
	for _, ok := range []float64{1, 15, 480} {
		if err := Validate(map[string]any{KeyIdleLockMinutes: ok}); err != nil {
			t.Errorf("%v harus valid: %v", ok, err)
		}
	}
	for _, bad := range []any{0.0, 481.0, 2.5, "10", nil, true} {
		if err := Validate(map[string]any{KeyIdleLockMinutes: bad}); err == nil {
			t.Errorf("%v harus ditolak", bad)
		}
	}
}
