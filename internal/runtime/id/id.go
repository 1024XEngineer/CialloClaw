package id

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"time"
)

var counter uint64

func New(prefix string) string {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		v := atomic.AddUint64(&counter, 1)
		return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixNano(), v)
	}
	v := atomic.AddUint64(&counter, 1)
	return fmt.Sprintf("%s-%d-%s-%d", prefix, time.Now().UnixMilli(), hex.EncodeToString(buf[:]), v)
}
