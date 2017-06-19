// +build !windows

package archive

import "path"

// cleanPathToJoin normalize the path. For unix users there's nothing special
// here, but in Windows environment we need to drop the volume letter before
// joining the path.
func cleanPathToJoin(filename string) string {
	return path.Clean(filename)
}
