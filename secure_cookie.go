package web

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"golang.org/x/crypto/pbkdf2"
	"io"
	"strings"
)

const (
	pbkdf2Iterations = 64000
	keySize          = 32
)

var (
	ErrMissingCookieSecret = errors.New("Secret Key for secure cookies has not been set. Assign one to web.Config.CookieSecret.")
	ErrInvalidKey          = errors.New("The keys for secure cookies have not been initialized. Ensure that a Run* method is being called")
)

func (ctx *Context) SetSecureCookie(name string, val string, age int64) error {
	server := ctx.Server
	if len(server.Config.CookieSecret) == 0 {
		return ErrMissingCookieSecret
	}
	if len(server.encKey) == 0 || len(server.signKey) == 0 {
		return ErrInvalidKey
	}
	ciphertext, err := encrypt([]byte(val), server.encKey)
	if err != nil {
		return err
	}
	sig := sign(ciphertext, server.signKey)
	data := base64.StdEncoding.EncodeToString(ciphertext) + "|" + base64.StdEncoding.EncodeToString(sig)
	ctx.SetCookie(NewCookie(name, data, age))
	return nil
}

func (ctx *Context) GetSecureCookie(name string) (string, bool) {
	for _, cookie := range ctx.Request.Cookies() {
		if cookie.Name != name {
			continue
		}
		parts := strings.SplitN(cookie.Value, "|", 2)
		if len(parts) != 2 {
			return "", false
		}
		ciphertext, err := base64.StdEncoding.DecodeString(parts[0])
		if err != nil {
			return "", false
		}
		sig, err := base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			return "", false
		}
		expectedSig := sign([]byte(ciphertext), ctx.Server.signKey)
		if !bytes.Equal(expectedSig, sig) {
			return "", false
		}
		plaintext, err := decrypt(ciphertext, ctx.Server.encKey)
		if err != nil {
			return "", false
		}
		return string(plaintext), true
	}
	return "", false
}

func genKey(password string, salt string) []byte {
	return pbkdf2.Key([]byte(password), []byte(salt), pbkdf2Iterations, keySize, sha512.New)
}

func encrypt(plaintext []byte, key []byte) ([]byte, error) {
	aesCipher, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}
	stream := cipher.NewCTR(aesCipher, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], plaintext)
	return ciphertext, nil
}

func decrypt(ciphertext []byte, key []byte) ([]byte, error) {
	if len(ciphertext) <= aes.BlockSize {
		return nil, errors.New("Invalid cipher text")
	}
	aesCipher, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	plaintext := make([]byte, len(ciphertext)-aes.BlockSize)
	stream := cipher.NewCTR(aesCipher, ciphertext[:aes.BlockSize])
	stream.XORKeyStream(plaintext, ciphertext[aes.BlockSize:])
	return plaintext, nil
}

func sign(data []byte, key []byte) []byte {
	mac := hmac.New(sha512.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}
