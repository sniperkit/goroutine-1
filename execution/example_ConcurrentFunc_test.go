package execution_test

import (
	"errors"
	"fmt"
	"time"

	"github.com/atdiar/goroutine/execution"
)

func Example_concurrentFunc() {
	// As in the SequentialToConcurrentFunc example, Counting is a function that
	// increments a counter every millisecond.
	// The max number of increments is provided by the user as input.
	//
	// Here we are not having a blackbox sequential function that we are
	// calling within a goroutine in order to avoid it blocking the main thread.
	// Rather we build a Counting function that can be stopped, from the get-go.

	Counting := func(c execution.Controller, inc int) (int, error) {
		if inc < 0 {
			return 0, errors.New("Increment below zero.")
		}
		errCh := make(chan error)
		var noresult int

		for i := 0; i < inc; i++ {
			select {
			case <-c.WasCancelled(errCh):
				return noresult, <-errCh
			default:
				time.Sleep(1 * time.Millisecond)
				continue
			}
		}
		return inc, nil
	}

	res, err := Counting(execution.NewController().CancelAfter(execution.Timeout(20*time.Millisecond)), 30)
	fmt.Println(res, err) // Expect: 0 Time ran out!

	res2, err2 := Counting(execution.NewController().CancelAfter(execution.Timeout(60*time.Millisecond)), 30)
	fmt.Println(res2, err2) // Expect: 30 <nil>

	parent := execution.NewController()
	child := parent.Spawn()
	parent.Cancel() // Cancels the parent's subtasks, i.e. child.

	res3, err3 := Counting(child, 30)
	fmt.Println(res3, err3) // Expect: 0 Subtask was aborted!

	res4, err4 := Counting(execution.NewController(), -1)
	fmt.Println(res4, err4) // Expect: 0 Increment below zero.

	// Output:
	// 0 Time ran out!
	// 30 <nil>
	// 0 Subtask was aborted!
	// 0 Increment below zero.
}
