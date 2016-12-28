package flow

import (
	"fmt"

	"code.uber.internal/devexp/minions-client-go.git/common/coroutine"
)

// Error to return from Workflow and Activity implementations.
type Error interface {
	error
	Reason() string
	Details() []byte
}

// NewError creates Error instance
func NewError(reason string, details []byte) Error {
	return &errorImpl{reason: reason, details: details}
}

// ActivityClient is used to invoke activities from a workflow definition
type ActivityClient interface {
	ExecuteActivity(parameters ExecuteActivityParameters) (result []byte, err Error)
}

// Func is a function used to spawn workflow execution through Context.Go
type Func func(ctx Context)

// Context of a workflow execution
type Context interface {
	coroutine.Context
	ActivityClient
	WorkflowInfo() *WorkflowInfo
	Go(f Func) // Must be used to create goroutines inside a workflow code
}

// Workflow is an interface that any workflow should implement.
// Code of a workflow must use coroutine.Channel, coroutine.Selector, and Context.Go instead of
// native channels, select and go.
type Workflow interface {
	Execute(ctx Context, input []byte) (result []byte, err Error)
}

// NewWorkflowDefinition creates a  WorkflowDefinition from a Workflow
func NewWorkflowDefinition(workflow Workflow) WorkflowDefinition {
	return &workflowDefinition{workflow: workflow}
}

type workflowDefinition struct {
	workflow   Workflow
	dispatcher coroutine.Dispatcher
}

type workflowResult struct {
	workflowResult []byte
	error          Error
}

type contextImpl struct {
	coroutine.Context
	wc         workflowContext
	dispatcher coroutine.Dispatcher
	result     **workflowResult
}

type activityClient struct {
	dispatcher  coroutine.Dispatcher
	asyncClient asyncActivityClient
}

// errorImpl implements Error
type errorImpl struct {
	reason  string
	details []byte
}

func (e *errorImpl) Error() string {
	return e.reason
}

// Reason is from Error interface
func (e *errorImpl) Reason() string {
	return e.reason
}

// Details is from Error interface
func (e *errorImpl) Details() []byte {
	return e.details
}

func (d *workflowDefinition) Execute(wc workflowContext, input []byte) {
	var resultPtr *workflowResult
	c := &contextImpl{
		wc:     wc,
		result: &resultPtr,
	}
	c.dispatcher = coroutine.NewDispatcher(func(ctx coroutine.Context) {
		c.Context = ctx
		r := &workflowResult{}
		r.workflowResult, r.error = d.workflow.Execute(c, input)
		*c.result = r
	})
	d.dispatcher = c.dispatcher
	c.executeDispatcher()
}

func (d *workflowDefinition) StackTrace() string {
	return d.dispatcher.StackTrace()
}

// executeDispatcher executed coroutines in the calling thread and calls workflow completion callbacks
// if root workflow function returned
func (c *contextImpl) executeDispatcher() {
	panicErr := c.dispatcher.ExecuteUntilAllBlocked()
	if panicErr != nil {
		c.wc.Complete(nil, NewError(panicErr.Error(), []byte(panicErr.StackTrace())))
		c.dispatcher.Close()
		return
	}
	r := *c.result
	if r == nil {
		return
	}
	// Cannot cast nil values from interface to interface
	var err Error
	if r.error != nil {
		err = r.error.(Error)
	}
	c.wc.Complete(r.workflowResult, err)
	c.dispatcher.Close()
}

func (c *contextImpl) ExecuteActivity(parameters ExecuteActivityParameters) (result []byte, err Error) {
	channelName := fmt.Sprintf("\"activity %v\"", parameters.ActivityID)
	resultChannel := coroutine.NewNamedBufferedChannel(c.Context, channelName, 1)
	c.wc.ExecuteActivity(parameters, func(r []byte, e Error) {
		result = r
		if e != nil {
			err = e.(Error)
		}
		ok := resultChannel.SendAsync(true)
		if !ok {
			panic("unexpected")
		}
		c.executeDispatcher()
	})
	_, _ = resultChannel.Recv(c)
	return
}

func (c *contextImpl) Go(f Func) {
	coroutine.NewCoroutine(c.Context, func(ctx coroutine.Context) {
		context := &contextImpl{
			Context:    ctx,
			wc:         c.wc,
			dispatcher: c.dispatcher,
			result:     c.result,
		}
		f(context)
	})
}

func (c *contextImpl) WorkflowInfo() *WorkflowInfo {
	return c.wc.WorkflowInfo()
}

// GetContext from flow.ContextProvider interface
func (c *contextImpl) GetContext() coroutine.Context {
	return c.Context
}
