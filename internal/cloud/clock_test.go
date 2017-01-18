package cloud

import (
	"testing"
	"time"
)

func TestRealClock_Now(t *testing.T) {
	var r realClock
	if time.Now().Add(-10 * time.Millisecond).After(r.Now()) {
		t.Error("real clock isn't returning the current time")
	}
}
