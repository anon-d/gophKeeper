package crypto

import (
	"testing"
	"testing/synctest"
	"time"
)

func TestVaultStoreAndGet(t *testing.T) {
	v := NewVault()
	v.Store("my-master-pass")

	got, ok := v.Get()
	if !ok {
		t.Fatal("expected vault to be available")
	}
	if got != "my-master-pass" {
		t.Fatalf("got %q, want %q", got, "my-master-pass")
	}
}

func TestVaultEmpty(t *testing.T) {
	v := NewVault()
	_, ok := v.Get()
	if ok {
		t.Fatal("expected empty vault")
	}
}

func TestVaultClear(t *testing.T) {
	v := NewVault()
	v.Store("pass")
	v.Clear()

	_, ok := v.Get()
	if ok {
		t.Fatal("expected vault to be empty after Clear")
	}
}

func TestVaultIsAvailable(t *testing.T) {
	v := NewVault()
	if v.IsAvailable() {
		t.Fatal("expected not available")
	}
	v.Store("pass")
	if !v.IsAvailable() {
		t.Fatal("expected available")
	}
}

func TestVaultRefresh(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		v := NewVault()
		v.Store("pass")

		v.mu.Lock()
		firstExpiry := v.expiresAt
		v.mu.Unlock()

		time.Sleep(1 * time.Second)
		v.Refresh()

		v.mu.Lock()
		secondExpiry := v.expiresAt
		v.mu.Unlock()

		if !secondExpiry.After(firstExpiry) {
			t.Fatal("Refresh should extend expiry")
		}
	})
}

func TestVaultOverwrite(t *testing.T) {
	v := NewVault()
	v.Store("first")
	v.Store("second")

	got, ok := v.Get()
	if !ok || got != "second" {
		t.Fatalf("got %q, want %q", got, "second")
	}
}
