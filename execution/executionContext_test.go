package execution

import (
	"testing"
	"time"
)

// Dummy is a throwaway type that implements the Storer interface.
type Dummy struct{}

func (d Dummy) Get(k interface{}) (interface{}, error) { return "Dummy", nil }
func (d Dummy) Put(k, v interface{})                   {}
func (d Dummy) Delete(k interface{})                   {}
func (d Dummy) Clear()                                 {}

func TestContext(t *testing.T) {
	// Let's create a NewContext execution context
	v := NewContext(Dummy{})

	// Let's make sure the Storer was correctly instantiated.
	res, err := v.Get("whatever")
	if err != nil || res != interface{}("Dummy") {
		t.Error("Context object was not created properly")
	}

	// w is a child context relatively to v.
	// The operations it controls can be cancelled from v.
	w := v.Spawn()

	// z is a copy of w, only, a deadline has been provided.
	z := w.CancelAfter(Timeout(3 * time.Millisecond))

	// Let's cancel the operations that are derived from the top level context.
	v.Cancel()

	errc := make(chan error)

	// Let's check whether the cancellation was correctly propagated to the child context.
	select {
	case <-w.WasCancelled(errc):
		//fine
	default:
		t.Error("The spawned Context should have been cancelled")
	}

	select {
	case <-z.WasCancelled(errc):
		//fine
	default:
		t.Error("The spawned Context should have been cancelled")
	}

	select {
	case <-z.WasCancelled(errc):
		//fine
	case <-time.After(10 * time.Millisecond):
		t.Error("The spawned Context should have been cancelled already")
	}
}
