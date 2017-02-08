package cloud

import (
	"testing"
	"testing/quick"
)

func TestPasswordEncrypt(t *testing.T) {
	f := func(password string) bool {
		encrypted, err := PasswordEncrypt(password)
		if err != nil {
			t.Logf("error encrypting password. details: %s", err)
			return false
		}

		decrypted, err := passwordDecrypt(encrypted)
		if err != nil {
			t.Logf("error decrypting password. details: %s", err)
			return false
		}

		return password == decrypted
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 50}); err != nil {
		t.Error(err)
	}
}
