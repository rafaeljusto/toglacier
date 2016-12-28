package cloud

import "time"

// Clock used to retrieve the current time. Useful for mocking in test
// environments, or if you want you own implementation of clock to be used.
type Clock interface {
	// Now returns the current date and time.
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time {
	return time.Now()
}
