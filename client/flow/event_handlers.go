package flow

import (
	"fmt"

	log "github.com/Sirupsen/logrus"

	m "code.uber.internal/devexp/minions-client-go.git/.gen/go/minions"
	"code.uber.internal/devexp/minions-client-go.git/common"
)

type (
	// CompletionHandler Handler to indicate completion result
	CompletionHandler func(result []byte, err Error)

	// workflowExecutionEventHandler handler to handle workflowExecutionEventHandler
	workflowExecutionEventHandler struct {
		*workflowContext
		contextLogger      *log.Entry
		workflowDefinition WorkflowDefinition
	}

	// workflowContext an implementation of WorkflowContext represents a context for workflow execution.
	workflowContext struct {
		workflowInfo              *WorkflowInfo
		workflowDefinitionFactory WorkflowDefinitionFactory

		scheduledActivites           map[string]ResultHandler // Map of Activities(activity ID ->) and their response handlers
		scheduledEventIDToActivityID map[int64]string         // Mapping from scheduled event ID to activity ID
		counterID                    int32                    // To generate activity IDs
		executeDecisions             []*m.Decision            // Decisions made during the execute of the workflow
		completeHandler              CompletionHandler        // events completion handler
		contextLogger                *log.Entry
	}
)

func newWorkflowExecutionEventHandler(workflowInfo *WorkflowInfo, workflowDefinitionFactory WorkflowDefinitionFactory,
	completionHandler CompletionHandler, logger *log.Entry) *workflowExecutionEventHandler {
	context := &workflowContext{
		workflowInfo:                 workflowInfo,
		workflowDefinitionFactory:    workflowDefinitionFactory,
		scheduledActivites:           make(map[string]ResultHandler),
		scheduledEventIDToActivityID: make(map[int64]string),
		executeDecisions:             make([]*m.Decision, 0),
		completeHandler:              completionHandler,
		contextLogger:                logger}
	return &workflowExecutionEventHandler{context, logger, nil}
}

func (wc *workflowContext) WorkflowInfo() *WorkflowInfo {
	return wc.workflowInfo
}

func (wc *workflowContext) Complete(result []byte, err Error) {
	wc.completeHandler(result, err)
}

func (wc *workflowContext) GenerateActivityID() string {
	activityID := wc.counterID
	wc.counterID++
	return fmt.Sprintf("%d", activityID)
}

func (wc *workflowContext) SwapExecuteDecisions(decisions []*m.Decision) []*m.Decision {
	oldDecisions := wc.executeDecisions
	wc.executeDecisions = decisions
	return oldDecisions
}

func (wc *workflowContext) CreateNewDecision(decisionType m.DecisionType) *m.Decision {
	return &m.Decision{
		DecisionType: common.DecisionTypePtr(decisionType),
	}
}

func (wc *workflowContext) ExecuteActivity(parameters ExecuteActivityParameters, callback ResultHandler) {
	scheduleTaskAttr := &m.ScheduleActivityTaskDecisionAttributes{}
	if parameters.ActivityID == nil {
		scheduleTaskAttr.ActivityId = common.StringPtr(wc.GenerateActivityID())
	} else {
		scheduleTaskAttr.ActivityId = parameters.ActivityID
	}
	scheduleTaskAttr.ActivityType = common.ActivityTypePtr(parameters.ActivityType)
	scheduleTaskAttr.TaskList = common.TaskListPtr(m.TaskList{Name: common.StringPtr(parameters.TaskListName)})
	scheduleTaskAttr.Input = parameters.Input
	scheduleTaskAttr.ScheduleToCloseTimeoutSeconds = common.Int32Ptr(parameters.ScheduleToCloseTimeoutSeconds)
	scheduleTaskAttr.StartToCloseTimeoutSeconds = common.Int32Ptr(parameters.StartToCloseTimeoutSeconds)
	scheduleTaskAttr.ScheduleToStartTimeoutSeconds = common.Int32Ptr(parameters.ScheduleToStartTimeoutSeconds)

	decision := wc.CreateNewDecision(m.DecisionType_ScheduleActivityTask)
	decision.ScheduleActivityTaskDecisionAttributes = scheduleTaskAttr

	wc.executeDecisions = append(wc.executeDecisions, decision)
	wc.scheduledActivites[scheduleTaskAttr.GetActivityId()] = callback
	wc.contextLogger.Debugf("ExectueActivity: %s: %+v", scheduleTaskAttr.GetActivityId(), scheduleTaskAttr)
}

func (weh *workflowExecutionEventHandler) ProcessEvent(event *m.HistoryEvent) ([]*m.Decision, error) {
	if event == nil {
		return nil, fmt.Errorf("nil event provided")
	}

	switch event.GetEventType() {
	case m.EventType_WorkflowExecutionStarted:
		return weh.handleWorkflowExecutionStarted(event.WorkflowExecutionStartedEventAttributes)

	case m.EventType_WorkflowExecutionCompleted:
		// No Operation
	case m.EventType_WorkflowExecutionFailed:
		// No Operation
	case m.EventType_WorkflowExecutionTimedOut:
		// TODO:
	case m.EventType_DecisionTaskScheduled:
		// No Operation
	case m.EventType_DecisionTaskStarted:
		// No Operation
	case m.EventType_DecisionTaskTimedOut:
		// TODO:
	case m.EventType_DecisionTaskCompleted:
		// TODO:
	case m.EventType_ActivityTaskScheduled:
		attributes := event.ActivityTaskScheduledEventAttributes
		weh.scheduledEventIDToActivityID[event.GetEventId()] = attributes.GetActivityId()

	case m.EventType_ActivityTaskStarted:
		// No Operation
	case m.EventType_ActivityTaskCompleted:
		return weh.handleActivityTaskCompleted(event.ActivityTaskCompletedEventAttributes)

	case m.EventType_ActivityTaskFailed:
		return weh.handleActivityTaskFailed(event.ActivityTaskFailedEventAttributes)

	case m.EventType_ActivityTaskTimedOut:
		return weh.handleActivityTaskTimedOut(event.ActivityTaskTimedOutEventAttributes)

	case m.EventType_TimerStarted:
		// TODO:
	case m.EventType_TimerFired:
		// TODO:
	default:
		return nil, fmt.Errorf("missing event handler for event type: %v", event)
	}
	return nil, nil
}

func (weh *workflowExecutionEventHandler) StackTrace() string {
	return weh.workflowDefinition.StackTrace()
}

func (weh *workflowExecutionEventHandler) Close() {
}

func (weh *workflowExecutionEventHandler) handleWorkflowExecutionStarted(
	attributes *m.WorkflowExecutionStartedEventAttributes) (decisions []*m.Decision, err error) {
	weh.workflowDefinition, err = weh.workflowDefinitionFactory(weh.workflowInfo.workflowType)
	if err != nil {
		return nil, err
	}

	// Invoke the workflow.
	weh.workflowDefinition.Execute(weh, attributes.Input)
	return weh.SwapExecuteDecisions([]*m.Decision{}), nil
}

func (weh *workflowExecutionEventHandler) handleActivityTaskCompleted(
	attributes *m.ActivityTaskCompletedEventAttributes) ([]*m.Decision, error) {

	activityID, ok := weh.scheduledEventIDToActivityID[attributes.GetScheduledEventId()]
	if !ok {
		return nil, fmt.Errorf("unable to find activity ID for the event: %v", attributes)
	}
	handler, ok := weh.scheduledActivites[activityID]
	if !ok {
		return nil, fmt.Errorf("unable to find callback handler for the event: %v with activity ID: %v", attributes, activityID)
	}

	if handler != nil {
		// Invoke the callback
		handler(attributes.GetResult_(), nil)
	}
	return weh.SwapExecuteDecisions([]*m.Decision{}), nil
}

func (weh *workflowExecutionEventHandler) handleActivityTaskFailed(
	attributes *m.ActivityTaskFailedEventAttributes) ([]*m.Decision, error) {

	activityID, ok := weh.scheduledEventIDToActivityID[attributes.GetScheduledEventId()]
	if !ok {
		return nil, fmt.Errorf("unable to find activity ID for the event: %v", attributes)
	}
	handler, ok := weh.scheduledActivites[activityID]
	if !ok {
		return nil, fmt.Errorf("unable to find callback handler for the event: %v", attributes)
	}

	if handler != nil {
		err := &ActivityTaskFailedError{
			reason:  *attributes.Reason,
			details: attributes.Details}
		// Invoke the callback
		handler(nil, err)
	}
	return weh.SwapExecuteDecisions([]*m.Decision{}), nil
}

func (weh *workflowExecutionEventHandler) handleActivityTaskTimedOut(
	attributes *m.ActivityTaskTimedOutEventAttributes) ([]*m.Decision, error) {

	activityID, ok := weh.scheduledEventIDToActivityID[attributes.GetScheduledEventId()]
	if !ok {
		return nil, fmt.Errorf("unable to find activity ID for the event: %v", attributes)
	}
	handler, ok := weh.scheduledActivites[activityID]
	if !ok {
		return nil, fmt.Errorf("unable to find callback handler for the event: %v", attributes)
	}

	if handler != nil {
		err := &ActivityTaskTimeoutError{TimeoutType: attributes.GetTimeoutType()}
		// Invoke the callback
		handler(nil, err)
	}
	return weh.SwapExecuteDecisions([]*m.Decision{}), nil
}
