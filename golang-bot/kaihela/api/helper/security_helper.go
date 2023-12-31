package helper

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/hex"
	log "github.com/sirupsen/logrus"
	"strings"
)

func DecryptData(data string, encryptKey string) (error, []byte) {
	encryptKeyUsed := strings.Builder{}
	encryptKeyUsed.WriteString(encryptKey)
	if len(encryptKey) < 32 {
		encryptKeyUsed.Write(bytes.Repeat([]byte{byte(0)}, 32-len(encryptKey)))
	}
	rawBase64Decoded, err := base64.StdEncoding.DecodeString(data)

	if err != nil {
		log.Error(err)
		return err, nil
	}
	iv := rawBase64Decoded[:16]
	rawContent, err := base64.StdEncoding.DecodeString(string(rawBase64Decoded[16:]))
	if err != nil {
		log.Error(err)
		return err, nil
	}
	return Ase256Decode(rawContent, encryptKeyUsed.String(), iv)
}

func Ase256Encode(plaintext string, key string, iv string, blockSize int) (error, string) {
	bKey := []byte(key)
	bIV := []byte(iv)
	bPlaintext := PKCS5Padding([]byte(plaintext), blockSize, len(plaintext))
	block, err := aes.NewCipher(bKey)
	if err != nil {
		return err, ""
	}
	ciphertext := make([]byte, len(bPlaintext))
	mode := cipher.NewCBCEncrypter(block, bIV)
	mode.CryptBlocks(ciphertext, bPlaintext)
	return nil, hex.EncodeToString(ciphertext)
}

func Ase256Decode(cipherText []byte, encKey string, iv []byte) (error, []byte) {
	bKey := []byte(encKey)
	bIV := iv
	var cipherTextDecoded = cipherText

	block, err := aes.NewCipher(bKey)
	if err != nil {
		log.Error(err)
		return err, nil
	}

	mode := cipher.NewCBCDecrypter(block, bIV)
	mode.CryptBlocks([]byte(cipherTextDecoded), []byte(cipherTextDecoded))
	return nil, PKCS5Trimming(cipherTextDecoded)
}

func PKCS5Padding(ciphertext []byte, blockSize int, after int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

func PKCS5Trimming(encrypt []byte) []byte {
	padding := encrypt[len(encrypt)-1]
	return encrypt[:len(encrypt)-int(padding)]
}
