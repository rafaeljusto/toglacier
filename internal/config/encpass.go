package config

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"

	"github.com/pkg/errors"
)

// PasswordEncrypt uses the secret to encode the password. On error it
// will return an Error type encapsulated in a traceable error. To retrieve the
// desired error you can do:
//
//     type causer interface {
//       Cause() error
//     }
//
//     if causeErr, ok := err.(causer); ok {
//       switch specificErr := causeErr.Cause().(type) {
//       case *config.Error:
//         // handle specifically
//       default:
//         // unknown error
//       }
//     }
func PasswordEncrypt(input string) (string, error) {
	block, err := aes.NewCipher(passwordKey())
	if err != nil {
		return "", errors.WithStack(newError("", ErrorCodeInitCipher, err))
	}

	iv := make([]byte, block.BlockSize())
	if _, err = rand.Read(iv); err != nil {
		return "", errors.WithStack(newError("", ErrorCodeFillingIV, err))
	}

	output := make([]byte, len(input))
	ofbStream := cipher.NewOFB(block, iv)
	ofbStream.XORKeyStream(output, []byte(input))

	buffer := bytes.NewBuffer(iv)
	buffer.Write(output)
	return base64.StdEncoding.EncodeToString(buffer.Bytes()), nil
}

// passwordDecrypt decodes a encrypted password. On error it
// will return an Error type encapsulated in a traceable error. To retrieve the
// desired error you can do:
//
//     type causer interface {
//       Cause() error
//     }
//
//     if causeErr, ok := err.(causer); ok {
//       switch specificErr := causeErr.Cause().(type) {
//       case *config.Error:
//         // handle specifically
//       default:
//         // unknown error
//       }
//     }
func passwordDecrypt(input string) (string, error) {
	block, err := aes.NewCipher(passwordKey())
	if err != nil {
		return "", errors.WithStack(newError("", ErrorCodeInitCipher, err))
	}

	inputBytes, err := base64.StdEncoding.DecodeString(input)
	if err != nil {
		return "", errors.WithStack(newError("", ErrorCodeDecodeBase64, err))
	}

	if len(inputBytes) < block.BlockSize() {
		return "", errors.WithStack(newError("", ErrorCodePasswordSize, nil))
	}

	iv := inputBytes[:block.BlockSize()]
	inputBytes = inputBytes[block.BlockSize():]

	output := make([]byte, len(inputBytes))
	ofbStream := cipher.NewOFB(block, iv)
	ofbStream.XORKeyStream(output, inputBytes)
	return string(output), nil
}
