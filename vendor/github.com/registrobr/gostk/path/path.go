// Package path implements utility routines for manipulating slash-separated paths.
package path

import "strings"

// RelevantPath returns the right n directories from the path. If n is bigger
// than the number of directories the full path is returned.
func RelevantPath(path string, n int) string {
	tokens := strings.Split(path, "/")
	total := len(tokens)

	if n <= 0 || n >= total {
		return path
	}

	var result string
	for i := total - n; i < total; i++ {
		result += tokens[i] + "/"
	}
	return strings.TrimSuffix(result, "/")
}
