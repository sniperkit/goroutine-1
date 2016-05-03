package execution

import (
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	v := NewController()

	select {
	case cancelled := <-v.sigKill:
		t.Errorf("New task is wrongly terminated. Got %v on child channel, expected nothing", cancelled)
	default:
	}
}

func TestCancel(t *testing.T) {
	v := NewController()
	errch := make(chan error)
	v.Cancel()
	select {
	case <-v.sigKill:
	default:
		t.Errorf("The subtasks should have received a cancellation signal but didn't")
	}

	select {
	case <-v.WasCancelled(errch):
	default:
		t.Errorf("Cancel was called. child channel should have been closed")
	}

	// Now let's try canceling from a goroutine
	p := NewController().CancelAfter(Timeout(1 * time.Second))
	c := p.Spawn()
	go func(c Controller, errc chan error) {
		select {
		case <-c.WasCancelled(errc):
			select {
			case err := <-errc:
				if err != ErrCancelled {
					t.Errorf("Woops! Expected: %v but got: %v", ErrCancelled, err)
				}
			default:
				t.Error("could not retrieve any error value from the error channel.")
			}

		case <-time.After(30 * time.Millisecond):
			t.Errorf("Well it was supposed to get canceled but seems it didn't.")
		}
	}(c.Spawn(), errch)

	time.Sleep(3 * time.Millisecond)
	go func(parent Controller) {
		parent.Cancel()
	}(p)
	time.Sleep(1 * time.Millisecond)
}

func TestCancelIdempotence(t *testing.T) {
	v := NewController()
	errch := make(chan error)

	v.Cancel()
	v.Cancel() // x2
	select {
	case <-v.sigKill:
	default:
		t.Errorf("The subtasks should have received a cancellation signal but didn't")
	}

	select {
	case <-v.WasCancelled(errch):
	default:
		t.Errorf("Cancel was called. children channel should have been closed")
	}
}

func TestConcurrentCancellations(t *testing.T) {
	v := NewController()
	errch := make(chan error)

	for i := 1; i < 11; i++ {
		go func() {
			v.Cancel()
		}()
	}

	// The below makes sure that all the goroutines above have been launched.
	// i.e. there has effectively been concurrent calls for cancellation.
	time.Sleep(1 * time.Millisecond)

	select {
	case <-v.sigKill:
	default:
		t.Errorf("The subtasks should have received a cancellation signal but didn't")
	}

	select {
	case <-v.WasCancelled(errch):
	default:
		t.Errorf("Cancel was called. children channel should have been closed")
	}
}

func TestSpawn(t *testing.T) {
	w := NewController()
	v := w.Spawn()

	select {
	case smthg := <-v.parentSigKill:
		t.Errorf("Bad instantiation. Got %v but expected nothing", smthg)
	case cancelled := <-v.sigKill:
		t.Errorf("Subtask was wrongly terminated. Got %v on children chan, expected nothing", cancelled)
	default:
	}
}

func TestSpawnCopyCancellation(t *testing.T) {
	c := NewController()
	errCh := make(chan error)

	ctrl1 := c.Spawn()
	ctrl1Sibling := ctrl1.CancelAfter(Timeout(10 * time.Millisecond))

	ctrl2 := Controller(ctrl1) // type conversion makes a copy
	ctrl3 := Controller(ctrl1Sibling.Spawn())

	// Let's cancel the subtasks under the control of ctrl1Sibling first
	// Only ctrl3 should be expected to receive a cancellation signal
	ctrl1Sibling.Cancel()

	select {
	case <-ctrl3.WasCancelled(errCh):
		// ok
	default:
		t.Error("Was expecting this controller to receive the cancellations signal but it did not.")
	}

	select {
	case <-ctrl2.WasCancelled(errCh):
		t.Error("Unexpected reception of a cancellation signal.")
	default:
		// ok
	}

	// Now let's cancel the subtask using the execution.Controller c.
	// We should expect the controllers of all its subtasks to receive
	// the cancellation signal.
	c.Cancel()

	select {
	case <-ctrl1.WasCancelled(errCh):
		//ok
	default:
		t.Error("Cancellation was expected to be propagated to the copy. It wasn't")
	}

	select {
	case <-ctrl1Sibling.WasCancelled(errCh):
		//ok
	default:
		t.Error("Cancellation was expected to be propagated to the copy. It wasn't")
	}

	select {
	case <-ctrl2.WasCancelled(errCh):
		//ok
	default:
		t.Error("Cancellation was expected to be propagated to the copy. It wasn't")
	}

	select {
	case <-ctrl3.WasCancelled(errCh):
		//ok
	default:
		t.Error("Cancellation was expected to be propagated to the copy. It wasn't")
	}
}

func TestParentTaskCancellation(t *testing.T) {
	w := NewController()
	errch := make(chan error)

	v := w.Spawn()
	z := v.Spawn()
	w.Cancel()

	select {
	case <-w.WasCancelled(errch):
	default:
		t.Errorf("Cancel was called. children channel should have been closed")
	}
	select {
	case <-v.WasCancelled(errch):
	default:
		t.Errorf("Cancel was called on parent. Children's tasks should have expired")
	}

	select {
	case <-z.WasCancelled(errch):
	default:
		t.Errorf("Cancel was called on grandparent. Grandchild's tasks should have expired")
	}
}

func TestSpawnCancellation(t *testing.T) {
	w := NewController()
	errch := make(chan error)

	v := w.Spawn()
	z := v.Spawn()
	v.Cancel()

	select {
	case <-w.WasCancelled(errch):
		t.Errorf("Terminating a child task should not close its brothers and sisters")
	default:
		// ok
	}

	select {
	case <-v.WasCancelled(errch):
		// ok
	default:
		t.Errorf("Terminating a task should close its children.")
	}

	select {
	case <-z.WasCancelled(errch):
		// ok
	default:
		t.Errorf("The parent subtask should have been terminated. z should not be alive.")
	}

}

func TestCancelAfter(t *testing.T) {
	var lapse = 10 * time.Millisecond
	var wait = lapse + 60*time.Millisecond
	errch := make(chan error)

	c := NewController() // not cancelled
	deadline := Timeout(lapse)
	waittime := Timeout(wait)
	v := c.CancelAfter(deadline).Spawn()

	if v.deadline != deadline {
		t.Errorf("CancelAfter was not set correctly, expected %v but got %v", deadline, v.deadline)
	}

	select {
	case <-v.WasCancelled(errch):
	case <-time.After(wait):
		t.Errorf("It should have timedout after %v ms. It didn't even after %v ms", deadline, waittime)
	}

	// checking error channel value

	select {
	case e := <-errch:
		if e != ErrTimedOut {
			t.Errorf("\n Expected:\n %v \n Got:\n %v \n", ErrTimedOut, e)
		}
		//default:
		//t.Error("could not retrieve any error value from the error channel.")
	}

	select {
	case <-c.WasCancelled(errch):
		t.Errorf("Was not supposed to get cancelled")
	case <-time.After(lapse - 1*time.Millisecond):
		// ok
	}
}

func TestWasCancelled(t *testing.T) {
	w := NewController()
	v := w.Spawn()
	z := v.Spawn()
	v.Cancel()

	select {
	case <-v.parentSigKill:
		t.Errorf("v should not be able to terminate itself.")
	default:
		// ok
	}

	select {
	case <-v.parentSigKill:
		t.Errorf("Cancel was called. v should have expired i.e v.sigKill should be closed.")
	default:
		// ok
	}

	select {
	case <-v.sigKill:
	default:
		t.Errorf("Cancel was called. Channel should not be live")
	}

	select {
	case <-z.parentSigKill:
	default:
		t.Errorf("Cancel was called on z's parent. z should be have been terminated")
	}

	select {
	case <-z.parentSigKill:
	// put that here because in reality, we never check z.sigKill in isolation
	// z.sigKill is never closed unless we check for timeout or parent's status.
	case <-z.sigKill:
	default:
		t.Errorf("Cancel was called. All descendant channels should have been closed")
	}

}

func TestTimeoutFunc(t *testing.T) {
	timeout := Timeout(10 * time.Millisecond)
	c := NewController().CancelAfter(timeout)
	errCh := make(chan error)
	select {
	case <-c.WasCancelled(errCh):
		//ok
	case <-time.After(30 * time.Millisecond):
		t.Error("Well seems that the deadline function does not work as intended.")

	}
}
