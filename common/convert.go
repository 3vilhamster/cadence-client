package common

import (
	m "code.uber.internal/devexp/minions-client-go.git/.gen/go/minions"
)

// Int32Ptr makes a copy and returns the pointer to an int32.
func Int32Ptr(v int32) *int32 {
	return &v
}

// Int64Ptr makes a copy and returns the pointer to an int64.
func Int64Ptr(v int64) *int64 {
	return &v
}

// StringPtr makes a copy and returns the pointer to a string.
func StringPtr(v string) *string {
	return &v
}

// TaskListPtr makes a copy and returns the pointer to a TaskList.
func TaskListPtr(v m.TaskList) *m.TaskList {
	return &v
}

// ActivityTypePtr makes a copy and returns the pointer to a ActivityType.
func ActivityTypePtr(v m.ActivityType) *m.ActivityType {
	return &v
}

// DecisionTypePtr makes a copy and returns the pointer to a DecisionType.
func DecisionTypePtr(t m.DecisionType) *m.DecisionType {
	return &t
}

// EventTypePtr makes a copy and returns the pointer to a EventType.
func EventTypePtr(t m.EventType) *m.EventType {
	return &t
}

// WorkflowTypePtr makes a copy and returns the pointer to a WorkflowType.
func WorkflowTypePtr(t m.WorkflowType) *m.WorkflowType {
	return &t
}
