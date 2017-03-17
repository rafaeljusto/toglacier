// Package errors adds location and log levels to errors.
package errors

// Equal compares the errors messages. This is useful in unit tests to compare
// encapsulated error messages.
func Equal(first, second error) bool {
	if first == nil || second == nil {
		return first == second
	}

	err1, ok1 := first.(traceableError)
	err2, ok2 := second.(traceableError)

	if ok1 {
		if ok2 {
			return Equal(err1.err, err2.err)
		}

		return Equal(err1.err, second)

	}

	if ok2 {
		return Equal(err2.err, first)
	}

	return first.Error() == second.Error()
}
