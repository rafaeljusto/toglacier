// +build !windows

package archive

import "path"

// shouldIgnore returns true when the file should be excluded from the tarball.
func shouldIgnore(filename string) bool {
	return false
}

// cleanPathToJoin normalize the path. For unix users there's nothing special
// here, but in Windows environment we need to drop the volume letter before
// joining the path.
func cleanPathToJoin(filename string) string {
	return path.Clean(filename)
}
