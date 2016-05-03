package execution_test

import (
	"errors"
	"fmt"
	"time"

	"github.com/atdiar/goroutine/execution"
)

func ExampleSequentialToConcurrent() {
	// Counting is a function that increments a counter every millisecond.
	// The max number of increments is provided by the user as input.
	// This function is purely sequential, meaning it can't be stopped
	//
	// We build a version of that Counting function which accepts
	// a execution.Controller as argument, allowing us to specify a time limit
	// after we consider that the Counting operation took too long and therefore
	// failed.
	//
	// This an example of a concurrent wrapper thrown around a sequential
	// function. It keeps the whole program more reponsive even though the
	// the sequential part may end up using some resources for an indefinite
	// amount of time.
	//
	// Firstly, a time limit of 20ms is set for Counting(30).
	// We expect a timeout error.
	//
	// Secondly, a time limit of 60 ms is set for Counting(30).
	// We expect to get the result as enough time was provided.
	//
	// Thirdly, we pass an already cancelled execution.Controller.
	// We expect to see an error stating that the parent task was aborted.
	//
	// Lastly, the increment is negative.
	// We expect an "Increment below zero." error to be returned by
	// 'cancelableCounting'.
	Counting := func(inc int) (int, error) {
		if inc < 0 {
			return 0, errors.New("Increment below zero.")
		}
		for i := 0; i < inc; i++ {
			time.Sleep(1 * time.Millisecond)
			continue
		}
		return inc, nil
	}

	cancelableCounting := func(c execution.Controller, inc int) (int, error) {
		result := make(chan int)
		errc := make(chan error)
		var noresult int

		// Background calculation. As soon as the result is ready, it is sent.
		go func(r chan int, errch chan error) {
			res, err := Counting(inc)
			if err != nil {
				errch <- err
			}
			r <- res
		}(result, errc)

		// This select statement allows us to decide whether to wait for the
		// result or move on because it takes too much time or something failed.
		select {
		case <-c.WasCancelled(errc):
			return noresult, <-errc
		case r := <-result:
			c.Cancel() // needed to make sure subtasks are cancelled.
			return r, nil
		case err := <-errc:
			c.Cancel() // ditto
			return noresult, err
		}
	}

	res, err := cancelableCounting(execution.NewController().CancelAfter(execution.Timeout(20*time.Millisecond)), 30)
	fmt.Println(res, err) // Expect: 0 Time ran out!

	res2, err2 := cancelableCounting(execution.NewController().CancelAfter(execution.Timeout(60*time.Millisecond)), 30)
	fmt.Println(res2, err2) // Expect: 30 <nil>

	parent := execution.NewController()
	child := parent.Spawn()
	parent.Cancel() // Cancels the parent's subtasks, i.e. child.

	res3, err3 := cancelableCounting(child, 30)
	fmt.Println(res3, err3) // Expect: 0 Subtask was aborted!

	res4, err4 := cancelableCounting(execution.NewController(), -1)
	fmt.Println(res4, err4) // Expect: 0 Increment below zero.

	// Output:
	// 0 Time ran out!
	// 30 <nil>
	// 0 Subtask was aborted!
	// 0 Increment below zero.
}

func ExampleConcurrentFunction() {
	// As in the previous example, Counting is a function that increments
	// a counter every millisecond.
	// The max number of increments is provided by the user as input.
	//
	// Here we are not having a blackbox sequential function that we are
	// wrapping in order to use in a sequential fashion.
	// Rather we build a concurrent Counting function from the getgo.

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
