#goroutine

### goroutine-related packages

## Table of Content
1. [execution](#execution)


## execution
Package execution provides ways to control the progress of statement execution
within a goroutine.
It notably includes two primitives (`execution.Controller` and
`execution.Context`) whose methods facilitate the propagation of
a cancellation signal from a task to its subtasks.
