package alarm_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/dejitarudemon/mundarium/internal/libs/alarm"
)

func TestWait_AlreadyRung(t *testing.T) {
	a := alarm.NewAlarm(1)
	if err := a.Ring(context.Background()); err != nil {
		t.Fatalf("Ring: %v", err)
	}

	done := make(chan struct{})
	go func() {
		if err := a.Wait(context.Background()); err != nil {
			t.Errorf("Wait: %v", err)
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Wait blocked after Ring")
	}
}

func TestWait_BlocksUntilRing(t *testing.T) {
	a := alarm.NewAlarm(1)

	var wg sync.WaitGroup
	wg.Add(1)
	started := make(chan struct{})
	go func() {
		defer wg.Done()
		close(started)
		if err := a.Wait(context.Background()); err != nil {
			t.Errorf("Wait: %v", err)
		}
	}()

	<-started
	time.Sleep(20 * time.Millisecond)
	if err := a.Ring(context.Background()); err != nil {
		t.Fatalf("Ring: %v", err)
	}
	wg.Wait()
}

func TestWait_ContextCanceled(t *testing.T) {
	a := alarm.NewAlarm(1)
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	started := make(chan struct{})
	go func() {
		close(started)
		errCh <- a.Wait(ctx)
	}()

	<-started
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("want context.Canceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Wait did not return after cancel")
	}
}

func TestRing_ContextCanceled(t *testing.T) {
	a := alarm.NewAlarm(1)
	if err := a.Ring(context.Background()); err != nil {
		t.Fatalf("Ring fill: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := a.Ring(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled on full buffer, got %v", err)
	}
}
