package attendance

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"time"
)

var (
	ErrTokenInvalid = errors.New("attendance token is invalid")
	ErrTokenExpired = errors.New("attendance token has expired")
)

const macSize = 16

func NewSecret() ([]byte, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	return b, err
}

func step(t time.Time, ttl time.Duration) uint64 {
	return uint64(t.UnixNano() / int64(ttl))
}

func mac(secret, windowID []byte, counter uint64) []byte {
	h := hmac.New(sha256.New, secret)
	h.Write(windowID)
	var c [8]byte
	binary.BigEndian.PutUint64(c[:], counter)
	h.Write(c[:])
	return h.Sum(nil)[:macSize]
}

func uuidBytes(id string) ([]byte, error) {
	if len(id) != 36 {
		return nil, ErrTokenInvalid
	}
	raw := id[0:8] + id[9:13] + id[14:18] + id[19:23] + id[24:36]
	b, err := hex.DecodeString(raw)
	if err != nil {
		return nil, ErrTokenInvalid
	}
	return b, nil
}

func uuidString(b []byte) string {
	s := hex.EncodeToString(b)
	return s[0:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:32]
}

type Token struct {
	Value     string
	ExpiresAt time.Time
}

func IssueToken(windowID string, secret []byte, ttl time.Duration, now time.Time) (Token, error) {
	id, err := uuidBytes(windowID)
	if err != nil {
		return Token{}, err
	}
	counter := step(now, ttl)
	buf := make([]byte, 0, 16+8+macSize)
	buf = append(buf, id...)
	buf = binary.BigEndian.AppendUint64(buf, counter)
	buf = append(buf, mac(secret, id, counter)...)
	expires := time.Unix(0, int64(counter+1)*int64(ttl)).UTC()
	return Token{Value: base64.RawURLEncoding.EncodeToString(buf), ExpiresAt: expires}, nil
}

func ParseWindowID(token string) (string, error) {
	buf, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil || len(buf) != 16+8+macSize {
		return "", ErrTokenInvalid
	}
	return uuidString(buf[:16]), nil
}

func VerifyToken(token string, secret []byte, ttl time.Duration, now time.Time) error {
	buf, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil || len(buf) != 16+8+macSize {
		return ErrTokenInvalid
	}
	id := buf[:16]
	counter := binary.BigEndian.Uint64(buf[16:24])
	if !hmac.Equal(buf[24:], mac(secret, id, counter)) {
		return ErrTokenInvalid
	}
	current := step(now, ttl)
	if counter > current {
		return ErrTokenInvalid
	}
	if current-counter > 1 {
		return ErrTokenExpired
	}
	return nil
}
