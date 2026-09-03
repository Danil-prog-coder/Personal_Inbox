package events

import "testing"

func TestBusGivesOnlyNewEventsOfThisUser(t *testing.T) {
	bus := New(10)
	bus.Publish(1, "message.created", "старое")
	cursor := bus.Cursor()

	bus.Publish(1, "message.created", "своё")
	bus.Publish(2, "message.created", "чужое")

	received, next := bus.Since(1, cursor)
	if len(received) != 1 {
		t.Fatalf("подписчик получил %d событий вместо одного", len(received))
	}
	if received[0].Data != "своё" {
		t.Fatalf("пришло чужое событие: %v", received[0].Data)
	}
	if next <= cursor {
		t.Fatal("курсор не сдвинулся")
	}
	again, _ := bus.Since(1, next)
	if len(again) != 0 {
		t.Fatal("событие пришло дважды")
	}
}

func TestBusKeepsRingBufferBounded(t *testing.T) {
	bus := New(3)
	for index := 0; index < 10; index++ {
		bus.Publish(1, "message.created", index)
	}
	received, _ := bus.Since(1, 0)
	if len(received) != 3 {
		t.Fatalf("буфер разросся до %d событий", len(received))
	}
	if received[0].Data != 7 {
		t.Fatalf("в буфере остались не последние события: %v", received[0].Data)
	}
}

func TestClearResetsBus(t *testing.T) {
	bus := New(5)
	bus.Publish(1, "message.created", "старое")
	bus.Clear()
	if bus.Cursor() != 0 {
		t.Fatal("курсор не сброшен")
	}
	received, _ := bus.Since(1, 0)
	if len(received) != 0 {
		t.Fatal("события не очищены")
	}
}
