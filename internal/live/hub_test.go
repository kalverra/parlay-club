package live_test

import (
	"sync"
	"testing"
	"time"

	"github.com/kalverra/parlay-club/internal/live"
)

// expectSignal waits for one published signal.
func expectSignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()

	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for a signal")
	}
}

// expectNoSignal fails when a signal is queued or arrives shortly.
func expectNoSignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()

	select {
	case <-ch:
		t.Fatal("got a signal when none was expected")
	case <-time.After(20 * time.Millisecond):
	}
}

func TestSubscribeDeliversPublishesForItsTopic(t *testing.T) {
	t.Parallel()

	var hub live.Hub
	ch, unsubscribe := hub.Subscribe("slip:1")
	defer unsubscribe()

	hub.Publish("slip:1")

	expectSignal(t, ch)
	expectNoSignal(t, ch)
}

func TestSubscribeWithTheSameTopicEachGetTheSignal(t *testing.T) {
	t.Parallel()

	var hub live.Hub
	first, unsubscribeFirst := hub.Subscribe("slip:1")
	defer unsubscribeFirst()
	second, unsubscribeSecond := hub.Subscribe("slip:1")
	defer unsubscribeSecond()

	hub.Publish("slip:1")

	expectSignal(t, first)
	expectSignal(t, second)
}

func TestPublishOnlyReachesSubscribersOfThatTopic(t *testing.T) {
	t.Parallel()

	var hub live.Hub
	slip, unsubscribeSlip := hub.Subscribe("slip:1")
	defer unsubscribeSlip()
	other, unsubscribeOther := hub.Subscribe("week:1")
	defer unsubscribeOther()

	hub.Publish("slip:1")

	expectSignal(t, slip)
	expectNoSignal(t, other)
}

func TestPublishCoalescesABurst(t *testing.T) {
	t.Parallel()

	var hub live.Hub
	ch, unsubscribe := hub.Subscribe("slip:1")
	defer unsubscribe()

	for range 5 {
		hub.Publish("slip:1")
	}

	expectSignal(t, ch)
	expectNoSignal(t, ch)
}

func TestUnsubscribeStopsDelivery(t *testing.T) {
	t.Parallel()

	var hub live.Hub
	ch, unsubscribe := hub.Subscribe("slip:1")

	unsubscribe()
	unsubscribe() // Idempotent, so a deferred call cannot panic or break others.

	hub.Publish("slip:1")

	expectNoSignal(t, ch)
}

func TestUnsubscribeLeavesOtherSubscribersAlone(t *testing.T) {
	t.Parallel()

	var hub live.Hub
	_, unsubscribeFirst := hub.Subscribe("slip:1")
	second, unsubscribeSecond := hub.Subscribe("slip:1")
	defer unsubscribeSecond()

	unsubscribeFirst()
	hub.Publish("slip:1")

	expectSignal(t, second)
}

func TestSubscribersCanStillJoinAfterTheTopicEmpties(t *testing.T) {
	t.Parallel()

	var hub live.Hub
	_, unsubscribe := hub.Subscribe("slip:1")
	unsubscribe()

	ch, unsubscribeAgain := hub.Subscribe("slip:1")
	defer unsubscribeAgain()

	hub.Publish("slip:1")

	expectSignal(t, ch)
}

func TestConcurrentPublishAndUnsubscribe(t *testing.T) {
	t.Parallel()

	var hub live.Hub
	const listeners = 32

	var wg sync.WaitGroup
	for range listeners {
		_, unsubscribe := hub.Subscribe("slip:1")
		wg.Go(unsubscribe)
	}
	for range 4 {
		wg.Go(func() {
			for range 100 {
				hub.Publish("slip:1")
			}
		})
	}
	wg.Wait()
}
