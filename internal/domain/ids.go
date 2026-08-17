package domain

import (
	"crypto/rand"
	"fmt"
	"time"
)

func NewID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func NewSerial() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func Timestamp() int64 {
	return time.Now().UnixNano()
}
