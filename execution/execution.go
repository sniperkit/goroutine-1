// Package execution provides ways to control program execution of goroutines.
// It notably includes primitives whose methods facilitate the propagation of
// a cancellation signal from a task to its subtasks.
//
// We call a task a unit of work that runs in a dedicated goroutine.
// A subtask is a subunit of work that is being processed in its own goroutine
// on behalf of a task.
//
// An execution.Controller or an execution.Context enables to manually trigger
// the cancellation of a subtask.
// They may also specify automatic cancellation by providing a deadline.
package execution

import (
	"errors"
	"sync"
	"time"
)

var (
	// ErrTimedOut is returned when a task did not complete in time.
	ErrTimedOut = errors.New("Time ran out!")
	// ErrCancelled is returned when a subtask was aborted by its parent task.
	ErrCancelled = errors.New("Subtask was aborted!")
)

// A Controller provides methods used to control the execution flow of a
// goroutine at user-defined spots (select statements).
type Controller struct {
	sigKill       chan struct{}
	once          *sync.Once
	parentSigKill chan struct{}
	deadline      time.Time
}

// NewController invokes the creation of a new task Controller.
func NewController() Controller {
	return Controller{
		sigKill:       newsignalchan(),
		once:          new(sync.Once),
		parentSigKill: none,
		deadline:      time.Time{},
	}
}

// newsignalchan creates a new signaling channel.
func newsignalchan() chan struct{} {
	return make(chan struct{})
}

// none defines a signaling channel that can never be triggered (nil channel)
// and is used to model non-cancellability.
var none chan struct{}

// Cancel aborts the hierarchy of subtasks running in child goroutines.
// A task cannot cancel itself. It can only cancel its own subtasks.
func (c Controller) Cancel() {
	select {
	case <-c.sigKill:
	default:
		c.once.Do(func() {
			close(c.sigKill)
		})
	}
}

// Spawn creates a child Controller.
// Spawned controllers are used by subtasks running in child goroutines.
func (c Controller) Spawn() Controller {
	return Controller{
		sigKill:       newsignalchan(),
		once:          new(sync.Once),
		parentSigKill: c.sigKill,
		deadline:      c.deadline,
	}
}

// CancelAfter will clone and alter a Controller, providing a date
// for the automatic dispatch of a cancellation signal.
// A date that has already passed implies immediate cancellation.
//
// It enables sibling tasks with different cancellation policies.
func (c Controller) CancelAfter(t time.Time) Controller {
	c.deadline = t
	c.sigKill = newsignalchan()
	c.once = new(sync.Once)
	return c
}

func (c Controller) timedout() *time.Timer {
	if !c.deadline.IsZero() {
		return time.NewTimer(c.deadline.UTC().Sub(time.Now().UTC()))
	}
	return nil
}

// WasCancelled returns a channel which allows to be notified
// when a task has been aborted.
//
// An error channel should be passed as argument.
// If non-nil, it will be used to communicate the specific reason for which
// a task was cancelled.
// It's the responsibility of the caller to make sure that the channel will not
// be closed.
//
// The reasons for the signal to trigger can be twofold:
// the task ran out of time, or its parent task cancelled it.
func (task Controller) WasCancelled(errCh chan error) <-chan struct{} {

	timer := task.timedout()
	var expired <-chan time.Time
	if timer != nil {
		expired = timer.C
	}

	go func(c Controller, timer *time.Timer, expired <-chan time.Time, errchan chan error) {
		if c.parentSigKill != none { // i.e. it is a spawned task (aka subtask, not top level.)
			select {
			case <-c.parentSigKill:
				c.Cancel()
				if timer != nil {
					timer.Stop()
				}
				if errchan != nil {
					errchan <- ErrCancelled
				}
			case <-expired:
				c.Cancel()
				if errchan != nil {
					errchan <- ErrTimedOut
				}
			}
		} else { // this is a root task i.e. it has no parent.
			if expired != nil {
				select {
				case <-expired:
					c.Cancel()
					if errchan != nil {
						errchan <- ErrTimedOut
					}
				}
			}
		}

	}(task, timer, expired, errCh)

	// Use of select needed here to make sure all channel communications
	// are synchronized (for task.sigKill)
	// Especially with the goroutine we launched above.
	if task.parentSigKill != none {
		select {
		case <-task.parentSigKill:
			task.Cancel()
			if timer != nil {
				timer.Stop()
			}
			return task.sigKill
		case <-expired:
			task.Cancel()
			return task.sigKill
		default:
			return task.sigKill
		}
	} else {
		select {
		case <-expired:
			task.Cancel()
			return task.sigKill
		default:
			return task.sigKill
		}
	}
}

// Timeout returns a deadline from a duration input.
// This deadline is a time.Time value in UTC.
// This function is not idempotent.
func Timeout(t time.Duration) time.Time {
	return time.Now().Add(t).UTC()
}

// #############################################################################

// Context is a wrapper around an execution Controller.
// In addition to the cancellation features, it provides an interface for
// the storage of shared data between a task and its subtasks.
type Context struct {
	Storer
	Controller
}

// NewContext creates and returns an execution Context.
// If you do not need storage, you should probably use a Controller instead.
// The Storer should be safe for concurrent use.
// Do not pass nil, it will panic.
func NewContext(s Storer) Context {
	if s == nil {
		panic("A Storer is required. Otherwise, just use a task.Controller.")
	}
	return Context{
		Storer:     s,
		Controller: NewController(),
	}
}

// Storer defines the interface that a type should implement to be able to be
// used as a storing datatype for an execution Context.
type Storer interface {
	// Get retrieves an element from the datastore if present.
	// Otherwise, it returns an error.
	Get(key interface{}) (interface{}, error)

	// Put inserts an element in the datastore.
	Put(key, value interface{})

	// Delete withdraws an element from the datastore.
	Delete(key interface{})

	// Clear empties the datastore.
	Clear()
}

// As a wrapper around a Controller, a Context
// provides a similar API to control task execution.
// However, the methods hereinafter need to be specified
// as we want to return a Context instance instead of a Controller one.

// Spawn creates a child context object.
func (c Context) Spawn() Context {
	c.Controller = c.Controller.Spawn()
	return c
}

// CancelAfter will clone and alter a Context, providing a time limit
// before the automatic dispatch of a cancellation signal.
// A zero or negative duration means there is no time limit.
func (c Context) CancelAfter(t time.Time) Context {
	c.Controller = c.Controller.CancelAfter(t)
	return c
}
