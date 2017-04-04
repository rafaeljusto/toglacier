package archive

// Builder creates an archive joining all paths in a file.
type Builder interface {
	Build(backupPaths ...string) (string, error)
}

// Envelop manages the security of an archive encrypting and decrypting the
// content.
type Envelop interface {
	Encrypt(filename, secret string) (string, error)
	Decrypt(encryptedFilename, secret string) (string, error)
}
