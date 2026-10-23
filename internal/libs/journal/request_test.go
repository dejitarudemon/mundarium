package journal_test

import (
	"testing"
	"time"

	"github.com/dejitarudemon/mundarium/internal/libs/journal"
)

func TestRequest_IsExpired(t *testing.T) {
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)

	t.Run("not expired without soft or hard", func(t *testing.T) {
		req := journal.Request{CreatedAt: now}
		if req.IsExpired(now, 0) {
			t.Fatal("want false")
		}
	})

	t.Run("soft deadline in the future", func(t *testing.T) {
		deadline := now.Add(time.Second)
		req := journal.Request{CreatedAt: now, ExpiresAt: &deadline}
		if req.IsExpired(now, 0) {
			t.Fatal("want false before soft deadline")
		}
	})

	t.Run("soft deadline equal to now", func(t *testing.T) {
		deadline := now
		req := journal.Request{CreatedAt: now, ExpiresAt: &deadline}
		if !req.IsExpired(now, 0) {
			t.Fatal("want true at soft deadline")
		}
	})

	t.Run("soft deadline in the past", func(t *testing.T) {
		deadline := now.Add(-time.Second)
		req := journal.Request{CreatedAt: now, ExpiresAt: &deadline}
		if !req.IsExpired(now, 0) {
			t.Fatal("want true after soft deadline")
		}
	})

	t.Run("hardTTL not reached", func(t *testing.T) {
		req := journal.Request{CreatedAt: now.Add(-time.Minute)}
		if req.IsExpired(now, 2*time.Minute) {
			t.Fatal("want false before hardTTL")
		}
	})

	t.Run("hardTTL reached", func(t *testing.T) {
		req := journal.Request{CreatedAt: now.Add(-2 * time.Minute)}
		if !req.IsExpired(now, time.Minute) {
			t.Fatal("want true when CreatedAt+hardTTL <= now")
		}
	})

	t.Run("hardTTL ignored when non-positive", func(t *testing.T) {
		req := journal.Request{CreatedAt: now.Add(-time.Hour)}
		if req.IsExpired(now, 0) {
			t.Fatal("want false with hardTTL <= 0 and no soft expiry")
		}
	})
}
