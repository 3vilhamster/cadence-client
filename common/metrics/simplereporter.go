package metrics

import (
	"sync/atomic"
	"time"

	"code.uber.internal/devexp/minions-client-go.git/common/util"

	"github.com/uber-common/bark"
	"github.com/uber-go/tally"
)

type (
	// SimpleReporter is the reporter used to dump metric to console for stress runs
	SimpleReporter struct {
		scope tally.Scope
		tags  map[string]string

		logger bark.Logger

		startTime                 time.Time
		workflowsStartCount       int64
		activitiesTotalCount      int64
		decisionsTotalCount       int64
		decisionsTimeoutCount     int64
		workflowsCompletionCount  int64
		workflowsEndToEndLatency  int64
		activitiesEndToEndLatency int64
		decisionsEndToEndLatency  int64

		previousReportTime                time.Time
		previousWorkflowsStartCount       int64
		previousActivitiesTotalCount      int64
		previousDecisionsTotalCount       int64
		previousDecisionsTimeoutCount     int64
		previousWorkflowsCompletionCount  int64
		previousWorkflowsEndToEndLatency  int64
		previousActivitiesEndToEndLatency int64
		previousDecisionsEndToEndLatency  int64
	}

	simpleStopWatch struct {
		metricName string
		reporter   *SimpleReporter
		startTime  time.Time
		elasped    time.Duration
	}
)

// Workflow Creation metrics
const (
	WorkflowsStartTotalCounter      = "workflows-start-total"
	ActivitiesTotalCounter          = "activities-total"
	DecisionsTotalCounter           = "decisions-total"
	DecisionsTimeoutCounter         = "decisions-timeout"
	WorkflowsCompletionTotalCounter = "workflows-completion-total"
	WorkflowEndToEndLatency         = "workflows-endtoend-latency"
	ActivityEndToEndLatency         = "activities-endtoend-latency"
	DecisionsEndToEndLatency        = "decisions-endtoend-latency"
)

// NewSimpleReporter create an instance of Reporter which can be used for driver to emit metric to console
func NewSimpleReporter(scope tally.Scope, tags map[string]string, logger bark.Logger) Reporter {
	reporter := &SimpleReporter{
		scope:  scope,
		tags:   make(map[string]string),
		logger: logger,
	}

	if tags != nil {
		util.MergeDictoRight(tags, reporter.tags)
	}

	// Initialize metric
	reporter.workflowsStartCount = 0
	reporter.activitiesTotalCount = 0
	reporter.decisionsTotalCount = 0
	reporter.decisionsTimeoutCount = 0
	reporter.workflowsCompletionCount = 0
	reporter.workflowsEndToEndLatency = 0
	reporter.activitiesEndToEndLatency = 0
	reporter.decisionsEndToEndLatency = 0
	reporter.startTime = time.Now()

	return reporter
}

// InitMetrics is used to initialize the metrics map with the respective type
func (r *SimpleReporter) InitMetrics(metricMap map[MetricName]MetricType) {
	// This is a no-op for simple reporter as it is already have a static list of metric to work with
}

// GetChildReporter creates the child reporter for this parent reporter
func (r *SimpleReporter) GetChildReporter(tags map[string]string) Reporter {
	sr := NewSimpleReporter(r.GetScope(), tags, r.logger)

	// copy the parent tags as well
	util.MergeDictoRight(r.GetTags(), sr.GetTags())

	return sr
}

// GetTags returns the tags for this reporter object
func (r *SimpleReporter) GetTags() map[string]string {
	return r.tags
}

// GetScope returns the metrics scope for this reporter
func (r *SimpleReporter) GetScope() tally.Scope {
	return r.scope
}

// IncCounter reports Counter metric to M3
func (r *SimpleReporter) IncCounter(name string, tags map[string]string, delta int64) {
	switch name {
	case WorkflowsStartTotalCounter:
		atomic.AddInt64(&r.workflowsStartCount, delta)
	case ActivitiesTotalCounter:
		atomic.AddInt64(&r.activitiesTotalCount, delta)
	case DecisionsTotalCounter:
		atomic.AddInt64(&r.decisionsTotalCount, delta)
	case WorkflowsCompletionTotalCounter:
		atomic.AddInt64(&r.workflowsCompletionCount, delta)
	case WorkflowEndToEndLatency:
		atomic.AddInt64(&r.workflowsEndToEndLatency, delta)
	case ActivityEndToEndLatency:
		atomic.AddInt64(&r.activitiesEndToEndLatency, delta)
	case DecisionsEndToEndLatency:
		atomic.AddInt64(&r.decisionsEndToEndLatency, delta)
	case DecisionsTimeoutCounter:
		atomic.AddInt64(&r.decisionsTimeoutCount, delta)
	default:
		r.logger.WithField(`name`, name).Error(`Unknown metric`)
	}
}

// UpdateGauge reports Gauge type metric
func (r *SimpleReporter) UpdateGauge(name string, tags map[string]string, value int64) {
	// Not implemented
}

// StartTimer returns a Stopwatch which when stopped will report the metric to M3
func (r *SimpleReporter) StartTimer(name string, tags map[string]string) Stopwatch {
	w := newSimpleStopWatch(name, r)
	w.Start()

	return w
}

// RecordTimer should be used for measuring latency when you cannot start the stop watch.
func (r *SimpleReporter) RecordTimer(name string, tags map[string]string, d time.Duration) {
	// Record the time as counter of time in milliseconds
	timeToRecord := int64(d / time.Millisecond)
	r.IncCounter(name, tags, timeToRecord)
}

// PrintStressMetric is used by stress host to dump metric to logs
func (r *SimpleReporter) PrintStressMetric() {

	currentTime := time.Now()
	elapsed := time.Duration(0)
	if !r.previousReportTime.IsZero() {
		elapsed = currentTime.Sub(r.previousReportTime) / time.Second
	}

	totalWorkflowStarted := atomic.LoadInt64(&r.workflowsStartCount)
	creationThroughput := int64(0)
	if elapsed > 0 && totalWorkflowStarted > r.previousWorkflowsStartCount {
		creationThroughput = (totalWorkflowStarted - r.previousWorkflowsStartCount) / int64(elapsed)
	}

	totalActivitiesCount := atomic.LoadInt64(&r.activitiesTotalCount)
	activitiesThroughput := int64(0)
	if elapsed > 0 && totalActivitiesCount > r.previousActivitiesTotalCount {
		activitiesThroughput = (totalActivitiesCount - r.previousActivitiesTotalCount) / int64(elapsed)
	}

	var activityLatency int64
	activitiesEndToEndLatency := atomic.LoadInt64(&r.activitiesEndToEndLatency)
	if totalActivitiesCount > 0 && activitiesEndToEndLatency > r.previousActivitiesEndToEndLatency {
		currentLatency := activitiesEndToEndLatency - r.previousActivitiesEndToEndLatency
		activityLatency = currentLatency / totalActivitiesCount
	}

	totalDecisionsCount := atomic.LoadInt64(&r.decisionsTotalCount)
	decisionsThroughput := int64(0)
	if elapsed > 0 && totalDecisionsCount > r.previousDecisionsTotalCount {
		decisionsThroughput = (totalDecisionsCount - r.previousDecisionsTotalCount) / int64(elapsed)
	}

	totalDecisionTimeoutCount := atomic.LoadInt64(&r.decisionsTimeoutCount)

	var decisionsLatency int64
	decisionsEndToEndLatency := atomic.LoadInt64(&r.decisionsEndToEndLatency)
	if totalDecisionsCount > 0 && decisionsEndToEndLatency > r.previousDecisionsEndToEndLatency {
		currentLatency := decisionsEndToEndLatency - r.previousDecisionsEndToEndLatency
		activityLatency = currentLatency / totalDecisionsCount
	}

	totalWorkflowsCompleted := atomic.LoadInt64(&r.workflowsCompletionCount)
	completionThroughput := int64(0)
	if elapsed > 0 && totalWorkflowsCompleted > r.previousWorkflowsCompletionCount {
		completionThroughput = (totalWorkflowsCompleted - r.previousWorkflowsCompletionCount) / int64(elapsed)
	}

	var latency int64
	workflowsLatency := atomic.LoadInt64(&r.workflowsEndToEndLatency)
	if totalWorkflowsCompleted > 0 && workflowsLatency > r.previousWorkflowsEndToEndLatency {
		currentLatency := workflowsLatency - r.previousWorkflowsEndToEndLatency
		latency = currentLatency / totalWorkflowsCompleted
	}

	r.logger.Infof("Workflows Started(Count=%v, Throughput=%v)",
		totalWorkflowStarted, creationThroughput)
	r.logger.Infof("Workflows Completed(Count=%v, Throughput=%v, Average Latency: %v)",
		totalWorkflowsCompleted, completionThroughput, latency)
	r.logger.Infof("Activites(Count=%v, Throughput=%v, Average Latency: %v)",
		totalActivitiesCount, activitiesThroughput, activityLatency)
	r.logger.Infof("Decisions(Count=%v, Throughput=%v, Average Latency: %v, TimeoutCount=%v)",
		totalDecisionsCount, decisionsThroughput, decisionsLatency, totalDecisionTimeoutCount)

	r.previousWorkflowsStartCount = totalWorkflowStarted
	r.previousActivitiesTotalCount = totalActivitiesCount
	r.previousDecisionsTotalCount = totalDecisionsCount
	r.previousDecisionsTimeoutCount = totalDecisionTimeoutCount
	r.previousWorkflowsCompletionCount = totalWorkflowsCompleted
	r.previousWorkflowsEndToEndLatency = workflowsLatency
	r.previousActivitiesEndToEndLatency = activitiesEndToEndLatency
	r.previousDecisionsEndToEndLatency = decisionsEndToEndLatency
	r.previousReportTime = currentTime
}

// PrintFinalMetric prints the workflows metrics
func (r *SimpleReporter) PrintFinalMetric() {
	workflowsCount := atomic.LoadInt64(&r.workflowsStartCount)
	workflowsCompletedCount := atomic.LoadInt64(&r.workflowsCompletionCount)
	totalLatency := atomic.LoadInt64(&r.workflowsEndToEndLatency)
	activitiesCount := atomic.LoadInt64(&r.activitiesTotalCount)
	totalActivitiesLatency := atomic.LoadInt64(&r.activitiesEndToEndLatency)
	decisionsCount := atomic.LoadInt64(&r.decisionsTotalCount)
	totalDecisionsLatency := atomic.LoadInt64(&r.decisionsEndToEndLatency)
	decisionsTimeoutCount := atomic.LoadInt64(&r.decisionsTimeoutCount)

	elapsed := time.Since(r.startTime) / time.Second

	var throughput int64
	var latency int64
	if workflowsCompletedCount > 0 {
		throughput = workflowsCompletedCount / int64(elapsed)
		latency = totalLatency / workflowsCompletedCount
	}

	var activityThroughput int64
	var activityLatency int64
	if activitiesCount > 0 {
		activityThroughput = activitiesCount / int64(elapsed)
		activityLatency = totalActivitiesLatency / activitiesCount
	}

	var decisionsThroughput int64
	var decisionLatency int64
	if decisionsCount > 0 {
		decisionsThroughput = decisionsCount / int64(elapsed)
		decisionLatency = totalDecisionsLatency / decisionsCount
	}

	r.logger.Infof("Total Workflows processed:(Started=%v, Completed=%v), Throughput: %v, Average Latency: %v",
		workflowsCount, workflowsCompletedCount, throughput, time.Duration(latency))
	r.logger.Infof("Total Activites processed:(Count=%v, Throughput=%v, Average Latency=%v)",
		activitiesCount, activityThroughput, activityLatency)
	r.logger.Infof("Total Decisions processed:(Count=%v, Throughput=%v, Average Latency=%v, TimeoutCount=%v)",
		decisionsCount, decisionsThroughput, decisionLatency, decisionsTimeoutCount)
}

// IsProcessComplete  indicates if we have completed processing.
func (r *SimpleReporter) IsProcessComplete() bool {
	totalWorkflowStarted := atomic.LoadInt64(&r.workflowsStartCount)
	totalWorkflowsCompleted := atomic.LoadInt64(&r.workflowsCompletionCount)
	return totalWorkflowStarted > 0 && totalWorkflowStarted == totalWorkflowsCompleted
}

// ResetMetric resets the metric values to zero
func (r *SimpleReporter) ResetMetric() {
	// Reset workflow metric
	atomic.StoreInt64(&r.workflowsStartCount, 0)
	atomic.StoreInt64(&r.workflowsCompletionCount, 0)
	atomic.StoreInt64(&r.activitiesTotalCount, 0)
	atomic.StoreInt64(&r.decisionsTotalCount, 0)
	atomic.StoreInt64(&r.workflowsEndToEndLatency, 0)
	atomic.StoreInt64(&r.decisionsTimeoutCount, 0)
}

func newSimpleStopWatch(metricName string, reporter *SimpleReporter) *simpleStopWatch {
	watch := &simpleStopWatch{
		metricName: metricName,
		reporter:   reporter,
	}

	return watch
}

func (w *simpleStopWatch) Start() {
	w.startTime = time.Now()
}

func (w *simpleStopWatch) Stop() time.Duration {
	w.elasped = time.Since(w.startTime)
	w.reporter.IncCounter(w.metricName, nil, w.milliseconds())

	return w.elasped
}

func (w *simpleStopWatch) milliseconds() int64 {
	return int64(w.elasped / time.Millisecond)
}

func (w *simpleStopWatch) microseconds() int64 {
	return int64(w.elasped / time.Microsecond)
}
