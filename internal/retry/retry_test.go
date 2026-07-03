package retry

import (
	"context"
	"testing"
	"time"
)

func TestPolicy_Delay(t *testing.T) {
	p := Policy{BaseDelay: 100 * time.Millisecond, MaxDelay: 1 * time.Second}

	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 0},
		{1, 100 * time.Millisecond},
		{2, 200 * time.Millisecond},
		{3, 400 * time.Millisecond},
		{4, 800 * time.Millisecond},
		{5, 1 * time.Second}, // capped
		{6, 1 * time.Second}, // still capped
	}

	for _, tt := range tests {
		got := p.Delay(tt.attempt)
		if got != tt.want {
			t.Errorf("Delay(%d) = %v, want %v", tt.attempt, got, tt.want)
		}
	}
}

func TestSleep_CompletesNormally(t *testing.T) {
	err := Sleep(context.Background(), 1*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSleep_ZeroDuration(t *testing.T) {
	err := Sleep(context.Background(), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSleep_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Sleep(ctx, 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestSleep_ContextDeadlineExceeded(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	err := Sleep(ctx, 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected error from expired context deadline")
	}
}
