package ton

import (
	"context"
	"testing"
)

func TestMockBalance(t *testing.T) {
	m := NewMock()
	ctx := context.Background()

	addr1 := "EQTestAddr1"
	addr2 := "EQTestAddr2"

	// Test balance is deterministic
	bal1, err := m.Balance(ctx, addr1)
	if err != nil {
		t.Fatalf("Balance() error = %v", err)
	}
	if bal1 < 1_000_000 || bal1 > 1_050_000 {
		t.Errorf("Balance() = %d, want between 1000000 and 1050000", bal1)
	}

	// Same address should return same balance
	bal1Again, err := m.Balance(ctx, addr1)
	if err != nil {
		t.Fatalf("Balance() error = %v", err)
	}
	if bal1 != bal1Again {
		t.Errorf("Balance() inconsistent: first=%d, second=%d", bal1, bal1Again)
	}

	// Different address should return different balance
	bal2, err := m.Balance(ctx, addr2)
	if err != nil {
		t.Fatalf("Balance() error = %v", err)
	}
	if bal1 == bal2 {
		t.Errorf("Balance() same for different addresses: %d", bal1)
	}
}

func TestMockSend(t *testing.T) {
	m := NewMock()
	ctx := context.Background()

	from := "EQTestFrom"
	to := "EQTestTo"
	amount := int64(5000)

	id, err := m.Send(ctx, from, to, amount)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if id == "" {
		t.Fatal("Send() returned empty ID")
	}
	if len(id) != 64 {
		t.Errorf("Send() ID length = %d, want 64 (SHA256 hex)", len(id))
	}

	// Check transaction status
	status, err := m.TxStatus(ctx, id)
	if err != nil {
		t.Fatalf("TxStatus() error = %v", err)
	}
	if status != "submitted" {
		t.Errorf("TxStatus() = %q, want 'submitted'", status)
	}
}

func TestMockTxStatus(t *testing.T) {
	m := NewMock()
	ctx := context.Background()

	// Unknown transaction
	status, err := m.TxStatus(ctx, "unknown-tx-id")
	if err != nil {
		t.Fatalf("TxStatus() error = %v", err)
	}
	if status != "unknown" {
		t.Errorf("TxStatus() = %q, want 'unknown'", status)
	}
}

func TestMockConcurrency(t *testing.T) {
	m := NewMock()
	ctx := context.Background()

	// Test concurrent sends don't race
	done := make(chan string, 10)
	for i := 0; i < 10; i++ {
		go func(n int) {
			id, err := m.Send(ctx, "EQFrom", "EQTo", int64(n*1000))
			if err != nil {
				t.Errorf("Send() error = %v", err)
			}
			done <- id
		}(i)
	}

	ids := make(map[string]bool)
	for i := 0; i < 10; i++ {
		id := <-done
		if ids[id] {
			t.Errorf("Duplicate transaction ID: %s", id)
		}
		ids[id] = true
	}
}
