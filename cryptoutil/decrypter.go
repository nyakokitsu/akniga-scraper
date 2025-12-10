package cryptoutil

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	//decrypt-password got from js script of akniga.
	//i've got all string parts and tried to brute password.
	password    = "EKxtcg46V"
	keyLen      = 32
	ivLen       = 16
	kdfHashAlgo = "md5"
)

func DecodeURL(inputJSON string) (string, error) {
	var data CipherParams
	err := json.Unmarshal([]byte(inputJSON), &data)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	ctBase64 := strings.ReplaceAll(data.CT, "\\/", "/")
	ciphertext, err := base64.StdEncoding.DecodeString(ctBase64)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 ciphertext: %w", err)
	}

	iv, err := hex.DecodeString(data.IV)
	if err != nil {
		return "", fmt.Errorf("failed to decode hex IV: %w", err)
	}
	if len(iv) != aes.BlockSize {
		return "", fmt.Errorf("invalid IV length: expected %d, got %d", aes.BlockSize, len(iv))
	}

	salt, err := hex.DecodeString(data.S)
	if err != nil {
		return "", fmt.Errorf("failed to decode hex salt: %w", err)
	}

	key, err := evpBytesToKey([]byte(password), salt, keyLen, ivLen)
	if err != nil {
		return "", fmt.Errorf("failed to derive key: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create AES cipher: %w", err)
	}

	if len(ciphertext) < aes.BlockSize {
		return "", errors.New("ciphertext too short")
	}
	if len(ciphertext)%aes.BlockSize != 0 {
		return "", errors.New("ciphertext is not a multiple of the block size")
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	decryptedPadded := make([]byte, len(ciphertext))
	mode.CryptBlocks(decryptedPadded, ciphertext)

	decryptedData, err := pkcs7Unpad(decryptedPadded)
	if err != nil {
		return "", fmt.Errorf("failed to unpad data (likely wrong key/ciphertext): %w", err)
	}

	finalURL := string(decryptedData)
	finalURL = strings.ReplaceAll(finalURL, "\\", "")
	finalURL = strings.ReplaceAll(finalURL, "\"", "")

	if finalURL == "" {
		return "", errors.New("decryption resulted in an empty string")
	}

	return finalURL, nil
}

func evpBytesToKey(password, salt []byte, keyLen, ivLen int) ([]byte, error) {
	var derivedBytes []byte
	var block []byte
	hasher := md5.New()
	var err error

	totalLen := keyLen + ivLen

	for len(derivedBytes) < totalLen {
		if len(block) > 0 {
			_, err = hasher.Write(block)
			if err != nil {
				return nil, fmt.Errorf("kdf hash write error (block): %w", err)
			}
		}

		_, err = hasher.Write(password)
		if err != nil {
			return nil, fmt.Errorf("kdf hash write error (password): %w", err)
		}
		_, err = hasher.Write(salt)
		if err != nil {
			return nil, fmt.Errorf("kdf hash write error (salt): %w", err)
		}

		block = hasher.Sum(nil)
		derivedBytes = append(derivedBytes, block...)
		hasher.Reset()
	}

	if len(derivedBytes) < keyLen {
		return nil, fmt.Errorf("derived bytes length (%d) is less than required key length (%d)", len(derivedBytes), keyLen)
	}
	return derivedBytes[:keyLen], nil
}

func pkcs7Unpad(data []byte) ([]byte, error) {
	length := len(data)
	if length == 0 {
		return nil, errors.New("pkcs7: data is empty")
	}

	paddingLen := int(data[length-1])

	if paddingLen > length || paddingLen > aes.BlockSize || paddingLen == 0 {
		return nil, fmt.Errorf("pkcs7: invalid padding length %d (data length %d)", paddingLen, length)
	}

	pad := data[length-paddingLen:]
	for i := 0; i < paddingLen; i++ {
		if pad[i] != byte(paddingLen) {
			return nil, errors.New("pkcs7: invalid padding bytes")
		}
	}

	return data[:length-paddingLen], nil
}
