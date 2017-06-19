// +build windows

package archive

import (
	"path"
	"regexp"
)

// volumeLetterRX matchs the volume letter in Windows.
var volumeLetterRX = regexp.MustCompile(`^[a-zA-Z]:`)

// cleanPathToJoin normalize the path and drops the volume name from the
// filename.
func cleanPathToJoin(filename string) string {
	filename = path.Clean(filename)
	return volumeLetterRX.ReplaceAllString(filename, "")
}
