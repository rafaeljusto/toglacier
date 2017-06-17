// +build windows

package archive

import (
	"path"
	"regexp"
)

// ownerFileRX matches identifies a Owner File (Same Directory as Source File).
//
// When a previously saved file is opened for editing, for printing, or for
// review, Word creates a temporary file that has a .doc file name extension.
// This file name extension begins with a tilde (~) that is followed by a dollar
// sign ($) that is followed by the remainder of the original file name. This
// temporary file holds the logon name of person who opens the file. This
// temporary file is called the "owner file."
//
// When you try to open a file that is available on a network and that is
// already opened by someone else, this file supplies the user name for the
// following error message: This file is already opened by user name. Would you
// like to make a copy of this file for your use?
var ownerFileRX = regexp.MustCompile(`^.*\~\$.*$`)

// volumeLetterRX matchs the volume letter in Windows.
var volumeLetterRX = regexp.MustCompile(`^[a-zA-Z]:`)

// shouldIgnore returns true when the file should be excluded from the tarball.
func shouldIgnore(filename string) bool {
	return ownerFileRX.MatchString(filename)
}

// cleanPathToJoin normalize the path and drops the volume name from the
// filename.
func cleanPathToJoin(filename string) string {
	filename = path.Clean(filename)
	return volumeLetterRX.ReplaceAllString(filename, "")
}
