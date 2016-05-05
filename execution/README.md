#execution

[![GoDoc](https://godoc.org/github.com/atdiar/goroutine/execution?status.svg)](https://godoc.org/github.com/atdiar/goroutine/execution)

Control of the execution flow of subtasks launched in goroutines.
--------------------------------------------------------------

A goroutine is stateless for the Go user.
It finishes when there is no more instruction to execute (task).

This package provides a way to cancel subtasks launched in goroutines early.
It may constitute a stepping stone toward the construction
of more elaborate execution contexts.

Typically, from within a given task, the user wants to be able to :  

* cancel a subtask if it takes too long (longer than a user-provided time limit)  
* cancel a subtask if an error occurred that makes running a task useless
(e.g. maybe one of the other subtasks failed)
* propagate the cancellation to the subtasks launched on behalf of a task automatically
* store and retrieve shared data between dependent tasks easily

An **execution.Controller** allows to trigger, monitor and propagate those cancellation events.
An **execution.Context** adds a facility for the storage of shared data between dependent tasks.

##How to use it?

You just need to pass an execution.Controller or execution.Context to any goroutine
whose instruction flow you are interested in controlling.
The control of the execution flow will occur at user-defined spots:
the `select` statements.

You may want to have a look at the package documentation on [godoc.org] for a meatier example.

To install the package (via CLI): go get github.com/atdiar/goroutine/execution

##Overview (contrived)

Create a new execution.Context which limits the execution of its subtasks time to 30ms.

``` go
ctx := execution.NewContext().CancelAfter(execution.Timeout(30 * time.Millisecond))
```

Launch a child goroutine whose execution flow will be controlled.

``` go
go func(c execution.Context){
	// processing...
}(ctx.Spawn())
```

Within that goroutine, a select statement can be used to control execution.
For instance, with three cases, it will look similar to this:

``` go
select{

case <-c.WasCancelled(errorChannel):
	// errorChannel is provided by the user to retrieve
	// the reason for which a task got cancelled early.

case result := <-resultChannel:
	// everything went OK and no cancellation
	// resultChannel is a channel whose purpose is to capture the result of some
	// concurrent computation.

	// let's cancel the subtasks if any are still running
	// we don't need them anymore
	c.Cancel()
	// Then we can continue what we are supposed to do.

case err := <-errorChannel:
	// no cancellation but the task encountered an error that was reported.
	// errorChannel is a channel whose purpose is to capture any concurrent
	// computation error.

	// let's cancel the subtasks if any are still running
	// we don't need them anymore
	c.Cancel()
	// ... and continue the handling of that situation.

}
```

Again, for completeness, please refer to the package [documentation].


[godoc.org]:https://godoc.org/github.com/atdiar/goroutine/execution/#example-package
[documentation]:https://godoc.org/github.com/atdiar/goroutine/execution
