package tokenx

import (
	"crypto/rand"
	"encoding/base64"
	"io"
)

func New(byteLen int) (string, error) {
	if byteLen < 32 {
		byteLen = 32
	}
	buf := make([]byte, byteLen)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
