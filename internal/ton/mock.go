package ton

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

type Mock struct {
	mu sync.Mutex
	tx map[string]string
}

func NewMock() *Mock {
	return &Mock{tx: make(map[string]string)}
}

func (m *Mock) Balance(ctx context.Context, addr string) (int64, error) {
	// deterministic fake balance
	h := sha256.Sum256([]byte(addr))
	v := int64(h[0])<<8 + int64(h[1])
	return 1_000_000 + (v % 50_000), nil
}

func (m *Mock) Send(ctx context.Context, from, to string, amount int64) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d|%d", from, to, amount, time.Now().UnixNano())))
	id := hex.EncodeToString(sum[:])
	m.tx[id] = "submitted"
	return id, nil
}

func (m *Mock) TxStatus(ctx context.Context, id string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if st, ok := m.tx[id]; ok {
		return st, nil
	}
	return "unknown", nil
}
