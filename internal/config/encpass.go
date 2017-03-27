package config

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"

	"github.com/pkg/errors"
)

// passwordKey returns the shared secret used to encrypt and decrypt the
// passwords. For safety, you should change this numbers before deploying this
// project. It's important that the secret has 16 characters, so that we can
// generate an AES-128.
func passwordKey() []byte {
	a0 := []byte{0x90, 0x19, 0x14, 0xa0, 0x94, 0x23, 0xb1, 0xa4, 0x98, 0x27, 0xb5, 0xa8, 0xd3, 0x31, 0xb9, 0xe2}
	a1 := []byte{0x10, 0x91, 0x20, 0x15, 0xa1, 0x95, 0x24, 0xb2, 0xa5, 0x99, 0x28, 0xb6, 0xa9, 0xd4, 0x32, 0xf1}
	a2 := []byte{0x12, 0x11, 0x92, 0x21, 0x16, 0xa2, 0x96, 0x25, 0xb3, 0xa6, 0xd1, 0x29, 0xb7, 0xe0, 0xd5, 0x33}
	a3 := []byte{0x18, 0x13, 0x12, 0x93, 0x22, 0x17, 0xa3, 0x97, 0x26, 0xb4, 0xa7, 0xd2, 0x30, 0xb8, 0xe1, 0xd6}

	result := make([]byte, 16)
	for i := 0; i < len(result); i++ {
		result[i] = (((a0[i] & a1[i]) ^ a2[i]) | a3[i])
	}

	return result
}

// PasswordEncrypt uses the secret to encode the password. On error it
// will return an ConfigError encapsulated in a traceable error. To retrieve
// the desired error you can do:
//
//     type causer interface {
//       Cause() error
//     }
//
//     if causeErr, ok := err.(causer); ok {
//       switch specificErr := causeErr.Cause().(type) {
//       case ConfigError:
//         // handle specifically
//       default:
//         // unknown error
//       }
//     }
func PasswordEncrypt(input string) (string, error) {
	block, err := aes.NewCipher(passwordKey())
	if err != nil {
		return "", errors.WithStack(newConfigError("", ConfigErrorCodeInitCipher, err))
	}

	iv := make([]byte, block.BlockSize())
	if _, err = rand.Read(iv); err != nil {
		return "", errors.WithStack(newConfigError("", ConfigErrorCodeFillingIV, err))
	}

	output := make([]byte, len(input))
	ofbStream := cipher.NewOFB(block, iv)
	ofbStream.XORKeyStream(output, []byte(input))

	buffer := bytes.NewBuffer(iv)
	buffer.Write(output)
	return base64.StdEncoding.EncodeToString(buffer.Bytes()), nil
}

// passwordDecrypt decodes a encrypted password. On error it
// will return an ConfigError encapsulated in a traceable error. To retrieve
// the desired error you can do:
//
//     type causer interface {
//       Cause() error
//     }
//
//     if causeErr, ok := err.(causer); ok {
//       switch specificErr := causeErr.Cause().(type) {
//       case ConfigError:
//         // handle specifically
//       default:
//         // unknown error
//       }
//     }
func passwordDecrypt(input string) (string, error) {
	block, err := aes.NewCipher(passwordKey())
	if err != nil {
		return "", errors.WithStack(newConfigError("", ConfigErrorCodeInitCipher, err))
	}

	inputBytes, err := base64.StdEncoding.DecodeString(input)
	if err != nil {
		return "", errors.WithStack(newConfigError("", ConfigErrorCodeDecodeBase64, err))
	}

	if len(inputBytes) < block.BlockSize() {
		return "", errors.WithStack(newConfigError("", ConfigErrorCodePasswordSize, nil))
	}

	iv := inputBytes[:block.BlockSize()]
	inputBytes = inputBytes[block.BlockSize():]

	output := make([]byte, len(inputBytes))
	ofbStream := cipher.NewOFB(block, iv)
	ofbStream.XORKeyStream(output, inputBytes)
	return string(output), nil
}
