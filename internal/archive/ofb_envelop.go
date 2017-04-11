package archive

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"io"
	"io/ioutil"
	"os"

	"github.com/pkg/errors"
	"github.com/rafaeljusto/toglacier/internal/log"
)

// RandomSource defines from where we are going to read random values to encrypt
// the archives.
var RandomSource = rand.Reader

// encryptedLabel is used to identify if an archive was encrypted or not.
const encryptedLabel = "encrypted:"

// OFBEnvelop manages the security of an archive using block cipher with output
// feedback mode.
type OFBEnvelop struct {
	logger log.Logger
}

// NewOFBEnvelop build a new OFBEnvelop with all necessary initializations.
func NewOFBEnvelop(logger log.Logger) *OFBEnvelop {
	return &OFBEnvelop{
		logger: logger,
	}
}

// Encrypt do what we expect, encrypting the content with a shared secret. It
// adds authentication using HMAC-SHA256. It will return the encrypted
// filename or an Error type encapsulated in a traceable error. To retrieve
// the desired error you can do:
//
//     type causer interface {
//       Cause() error
//     }
//
//     if causeErr, ok := err.(causer); ok {
//       switch specificErr := causeErr.Cause().(type) {
//       case *archive.Error:
//         // handle specifically
//       default:
//         // unknown error
//       }
//     }
func (o OFBEnvelop) Encrypt(filename, secret string) (string, error) {
	o.logger.Debugf("archive: encrypting file “%s”", filename)

	archive, err := os.Open(filename)
	if err != nil {
		return "", errors.WithStack(newError(filename, ErrorCodeOpeningFile, err))
	}
	defer archive.Close()

	o.logger.Debug("archive: creating temporary file for encryption")

	encryptedArchive, err := ioutil.TempFile("", "toglacier-")
	if err != nil {
		return "", errors.WithStack(newError(filename, ErrorCodeTmpFileCreation, err))
	}
	defer encryptedArchive.Close()

	o.logger.Debug("archive: calculating archive hash")

	hash, err := hmacSHA256(archive, secret)
	if err != nil {
		return "", errors.WithStack(err)
	}

	iv := make([]byte, aes.BlockSize)

	n, err := io.ReadFull(RandomSource, iv)
	if err != nil {
		return "", errors.WithStack(newError(filename, ErrorCodeGenerateRandomNumbers, err))
	}

	o.logger.Debugf("archive: generated %d random bytes for iv", n)

	n, err = encryptedArchive.WriteString(encryptedLabel)
	if err != nil {
		return "", errors.WithStack(newError(filename, ErrorCodeWritingLabel, err))
	}

	o.logger.Debugf("archive: wrote %d bytes to file (encrypted label)", n)

	n, err = encryptedArchive.Write(hash)
	if err != nil {
		return "", errors.WithStack(newError(filename, ErrorCodeWritingAuth, err))
	}

	o.logger.Debugf("archive: wrote %d bytes to file (auth)", n)

	n, err = encryptedArchive.Write(iv)
	if err != nil {
		return "", errors.WithStack(newError(filename, ErrorCodeWritingIV, err))
	}

	o.logger.Debugf("archive: wrote %d bytes to file (iv)", n)

	block, err := aes.NewCipher([]byte(secret))
	if err != nil {
		return "", errors.WithStack(newError(filename, ErrorCodeInitCipher, err))
	}

	writer := cipher.StreamWriter{
		S: cipher.NewOFB(block, iv),
		W: encryptedArchive,
	}
	defer writer.Close()

	written, err := io.Copy(&writer, archive)
	if err != nil {
		return "", errors.WithStack(newError(filename, ErrorCodeEncryptingFile, err))
	}

	o.logger.Debugf("archive: wrote %d bytes to file (encrypted content)", written)
	o.logger.Infof("archive: file “%s” encrypted", filename)
	return encryptedArchive.Name(), nil
}

// Decrypt do what we expect, decrypting the content with a shared secret. It
// authenticates the data using HMAC-SHA256. It will return the decrypted
// filename or an Error type encapsulated in a traceable error. To retrieve
// the desired error you can do:
//
//     type causer interface {
//       Cause() error
//     }
//
//     if causeErr, ok := err.(causer); ok {
//       switch specificErr := causeErr.Cause().(type) {
//       case *archive.Error:
//         // handle specifically
//       default:
//         // unknown error
//       }
//     }
func (o OFBEnvelop) Decrypt(encryptedFilename, secret string) (string, error) {
	o.logger.Debugf("archive: decrypting file “%s”", encryptedFilename)

	encryptedArchive, err := os.Open(encryptedFilename)
	if err != nil {
		return "", errors.WithStack(newError(encryptedFilename, ErrorCodeOpeningFile, err))
	}
	defer encryptedArchive.Close()

	archive, err := ioutil.TempFile("", "toglacier-")
	if err != nil {
		return "", errors.WithStack(newError(encryptedFilename, ErrorCodeTmpFileCreation, err))
	}
	defer archive.Close()

	encryptedLabelBuffer := make([]byte, len(encryptedLabel))
	n, err := encryptedArchive.Read(encryptedLabelBuffer)

	if err == io.EOF || string(encryptedLabelBuffer) != encryptedLabel {
		// if we couldn't read the encrypted label, maybe the file isn't encrypted,
		// so let's return it as it is
		return encryptedFilename, nil

	} else if err != nil {
		return "", errors.WithStack(newError(encryptedFilename, ErrorCodeReadingLabel, err))
	}

	o.logger.Debugf("archive: read %d bytes from encrypted file (encrypted label)", n)

	authHash := make([]byte, sha256.Size)

	n, err = encryptedArchive.Read(authHash)
	if err != nil {
		return "", errors.WithStack(newError(encryptedFilename, ErrorCodeReadingAuth, err))
	}

	o.logger.Debugf("archive: read %d bytes from encrypted file (auth)", n)

	iv := make([]byte, aes.BlockSize)

	n, err = encryptedArchive.Read(iv)
	if err != nil {
		return "", errors.WithStack(newError(encryptedFilename, ErrorCodeReadingIV, err))
	}

	o.logger.Debugf("archive: read %d bytes from encrypted file (iv)", n)

	block, err := aes.NewCipher([]byte(secret))
	if err != nil {
		return "", errors.WithStack(newError(encryptedFilename, ErrorCodeInitCipher, err))
	}

	reader := cipher.StreamReader{
		S: cipher.NewOFB(block, iv),
		R: encryptedArchive,
	}

	written, err := io.Copy(archive, reader)
	if err != nil {
		return "", errors.WithStack(newError(encryptedFilename, ErrorCodeDecryptingFile, err))
	}

	o.logger.Debugf("archive: decrypted %d bytes", written)

	hash, err := hmacSHA256(archive, secret)
	if err != nil {
		return "", errors.WithStack(err)
	}

	if !hmac.Equal(authHash, hash) {
		return "", errors.WithStack(newError("", ErrorCodeAuthFailed, nil))
	}

	o.logger.Infof("archive: file “%s” decrypted", archive.Name())
	return archive.Name(), nil
}

func hmacSHA256(f *os.File, secret string) ([]byte, error) {
	if _, err := f.Seek(0, 0); err != nil {
		return nil, errors.WithStack(newError(f.Name(), ErrorCodeRewindingFile, err))
	}

	hash := hmac.New(sha256.New, []byte(secret))
	if _, err := io.Copy(hash, f); err != nil {
		return nil, errors.WithStack(newError(f.Name(), ErrorCodeCalculateHMACSHA256, err))
	}

	if _, err := f.Seek(0, 0); err != nil {
		return nil, errors.WithStack(newError(f.Name(), ErrorCodeRewindingFile, err))
	}

	return hash.Sum(nil), nil
}
