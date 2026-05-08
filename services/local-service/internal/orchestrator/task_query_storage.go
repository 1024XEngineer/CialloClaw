package orchestrator

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	contextsvc "github.com/cialloclaw/cialloclaw/services/local-service/internal/context"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
)

// taskMap converts a runengine task record into the protocol-facing task shape.
func taskMap(record runengine.TaskRecord) map[string]any {
	result := map[string]any{
		"task_id":          record.TaskID,
		"session_id":       taskSessionValue(record.SessionID),
		"title":            record.Title,
		"source_type":      record.SourceType,
		"status":           record.Status,
		"intent":           cloneMap(record.Intent),
		"current_step":     record.CurrentStep,
		"risk_level":       record.RiskLevel,
		"loop_stop_reason": record.LoopStopReason,
		"started_at":       record.StartedAt.Format(dateTimeLayout),
		"updated_at":       record.UpdatedAt.Format(dateTimeLayout),
		"finished_at":      nil,
	}
	if record.FinishedAt != nil {
		result["finished_at"] = record.FinishedAt.Format(dateTimeLayout)
	}
	return result
}

func taskIsTerminal(status string) bool {
	switch status {
	case "completed", "cancelled", "ended_unfinished", "failed":
		return true
	default:
		return false
	}
}

// timelineMap converts internal timeline records into protocol-facing values.
func timelineMap(timeline []runengine.TaskStepRecord) []map[string]any {
	result := make([]map[string]any, 0, len(timeline))
	for _, step := range timeline {
		result = append(result, map[string]any{
			"step_id":        step.StepID,
			"task_id":        step.TaskID,
			"name":           step.Name,
			"status":         step.Status,
			"order_index":    step.OrderIndex,
			"input_summary":  step.InputSummary,
			"output_summary": step.OutputSummary,
		})
	}
	return result
}

// pageMap builds the shared paging payload used by list endpoints.
func pageMap(limit, offset, total int) map[string]any {
	return map[string]any{
		"limit":    limit,
		"offset":   offset,
		"total":    total,
		"has_more": offset+limit < total,
	}
}

func (s *Service) listTasksFromStructuredStorage(group, sortBy, sortOrder string, limit, offset int) ([]runengine.TaskRecord, int, bool) {
	records, total, err := s.storage.TaskStore().ListTasksForTaskList(context.Background(), group, sortBy, sortOrder, limit, offset)
	if err != nil {
		return nil, 0, false
	}
	if len(records) == 0 {
		return []runengine.TaskRecord{}, total, total > 0
	}
	tasks := make([]runengine.TaskRecord, 0, len(records))
	for _, record := range records {
		task, ok := s.structuredTaskRecordToRuntime(record, false)
		if !ok {
			continue
		}
		tasks = append(tasks, task)
	}
	return tasks, total, true
}

func (s *Service) taskListRecords(group, sortBy, sortOrder string, limit, offset int) ([]runengine.TaskRecord, int) {
	runtimeTasks, _ := s.runEngine.ListTasks(group, sortBy, sortOrder, 0, 0)
	storageTasks, storageTotal, storageReady := s.listTaskPageFromStorage(group, sortBy, sortOrder, limit, offset, runtimeTasks)
	if !storageReady {
		total := len(runtimeTasks)
		if offset >= total {
			return []runengine.TaskRecord{}, total
		}
		end := offset + limit
		if limit <= 0 || end > total {
			end = total
		}
		return runtimeTasks[offset:end], total
	}
	merged := mergeTaskLists(runtimeTasks, storageTasks)
	if len(merged) > 0 {
		runengineSortTaskRecords(merged, sortBy, sortOrder)
	}
	total := storageTotal + s.countRuntimeOnlyTasksForList(group, runtimeTasks)
	if offset >= len(merged) {
		return []runengine.TaskRecord{}, total
	}
	end := offset + limit
	if limit <= 0 || end > len(merged) {
		end = len(merged)
	}
	return merged[offset:end], total
}

func (s *Service) loadAllTaskListFromStorage() []runengine.TaskRecord {
	if s.storage == nil {
		return nil
	}
	structuredTasks := []runengine.TaskRecord(nil)
	if s.storage.TaskStore() != nil {
		structuredTasks = s.loadAllTasksFromStructuredStorage(false)
	}
	if len(structuredTasks) == 0 {
		return s.loadAllTasksFromTaskRunStorage()
	}
	legacyTasks := s.loadLegacyTaskRunsFromStorage(structuredTasks)
	if len(legacyTasks) == 0 {
		return structuredTasks
	}
	return mergeStructuredTaskListCompatibility(structuredTasks, legacyTasks)
}

func (s *Service) listTaskPageFromStorage(group, sortBy, sortOrder string, limit, offset int, runtimeTasks []runengine.TaskRecord) ([]runengine.TaskRecord, int, bool) {
	if s.storage == nil {
		return nil, 0, false
	}
	window := offset + limit + len(runtimeTasks)
	if limit <= 0 {
		window = 0
	}
	structuredTasks := []runengine.TaskRecord(nil)
	structuredTotal := 0
	structuredReady := false
	if s.storage.TaskStore() != nil {
		tasks, total, ok := s.listTasksFromStructuredStorage(group, sortBy, sortOrder, window, 0)
		if ok {
			structuredTasks = tasks
			structuredTotal = total
			structuredReady = true
		}
	}
	legacyTasks := []runengine.TaskRecord(nil)
	legacyTotal := 0
	legacyReady := false
	if s.storage.TaskRunStore() != nil {
		records, total, err := s.storage.TaskRunStore().ListLegacyTaskRunsForTaskList(context.Background(), group, sortBy, sortOrder, window, 0)
		if err == nil {
			legacyTasks = make([]runengine.TaskRecord, 0, len(records))
			for _, record := range records {
				legacyTasks = append(legacyTasks, taskRecordFromStorage(record))
			}
			legacyTotal = total
			legacyReady = true
		}
	}
	if structuredReady || legacyReady {
		return mergeStructuredTaskListCompatibility(structuredTasks, legacyTasks), structuredTotal + legacyTotal, true
	}
	storageTasks := filterAndSortTasks(s.loadAllTaskListFromStorage(), group, sortBy, sortOrder)
	return storageTasks, len(storageTasks), len(storageTasks) != 0
}

func (s *Service) loadAllTasksFromStorage() []runengine.TaskRecord {
	if s.storage == nil {
		return nil
	}
	structuredTasks := []runengine.TaskRecord(nil)
	if s.storage.TaskStore() != nil {
		structuredTasks = s.loadAllTasksFromStructuredStorage(true)
	}
	if len(structuredTasks) == 0 {
		return s.loadAllTasksFromTaskRunStorage()
	}
	legacyTasks := s.loadLegacyTaskRunsFromStorage(structuredTasks)
	if len(legacyTasks) == 0 {
		return structuredTasks
	}
	return mergeStructuredTaskListCompatibility(structuredTasks, legacyTasks)
}

func (s *Service) loadAllTasksFromTaskRunStorage() []runengine.TaskRecord {
	if s.storage == nil || s.storage.TaskRunStore() == nil {
		return nil
	}
	records, err := s.storage.TaskRunStore().LoadLegacyTaskRuns(context.Background(), nil)
	if err != nil || len(records) == 0 {
		return nil
	}
	tasks := make([]runengine.TaskRecord, 0, len(records))
	for _, record := range records {
		tasks = append(tasks, taskRecordFromStorage(record))
	}
	return tasks
}

func (s *Service) loadLegacyTaskRunsFromStorage(structuredTasks []runengine.TaskRecord) []runengine.TaskRecord {
	if s.storage == nil || s.storage.TaskRunStore() == nil {
		return nil
	}
	structuredTaskIDs := make([]string, 0, len(structuredTasks))
	for _, task := range structuredTasks {
		if strings.TrimSpace(task.TaskID) == "" {
			continue
		}
		structuredTaskIDs = append(structuredTaskIDs, task.TaskID)
	}
	records, err := s.storage.TaskRunStore().LoadLegacyTaskRuns(context.Background(), structuredTaskIDs)
	if err != nil || len(records) == 0 {
		return nil
	}
	tasks := make([]runengine.TaskRecord, 0, len(records))
	for _, record := range records {
		tasks = append(tasks, taskRecordFromStorage(record))
	}
	return tasks
}

// mergeStructuredTaskListCompatibility keeps first-class task rows authoritative
// while still appending legacy task_run-only entries so partially migrated
// databases do not lose pre-structured history in task-centric overview queries.
func mergeStructuredTaskListCompatibility(structuredTasks, taskRunTasks []runengine.TaskRecord) []runengine.TaskRecord {
	if len(structuredTasks) == 0 {
		return taskRunTasks
	}
	if len(taskRunTasks) == 0 {
		return structuredTasks
	}
	merged := make([]runengine.TaskRecord, 0, len(structuredTasks)+len(taskRunTasks))
	seen := make(map[string]struct{}, len(structuredTasks)+len(taskRunTasks))
	for _, task := range structuredTasks {
		merged = append(merged, task)
		seen[task.TaskID] = struct{}{}
	}
	for _, task := range taskRunTasks {
		if _, ok := seen[task.TaskID]; ok {
			continue
		}
		merged = append(merged, task)
	}
	return merged
}

func (s *Service) loadAllTasksFromStructuredStorage(includeCompatibility bool) []runengine.TaskRecord {
	records, _, err := s.storage.TaskStore().ListTasks(context.Background(), 0, 0)
	if err != nil || len(records) == 0 {
		return nil
	}
	tasks := make([]runengine.TaskRecord, 0, len(records))
	for _, record := range records {
		task, ok := s.structuredTaskRecordToRuntime(record, includeCompatibility)
		if !ok {
			continue
		}
		tasks = append(tasks, task)
	}
	return tasks
}

func (s *Service) countRuntimeOnlyTasksForList(group string, runtimeTasks []runengine.TaskRecord) int {
	if s == nil || s.storage == nil || s.storage.TaskStore() == nil {
		return len(runtimeTasks)
	}
	count := 0
	for _, runtimeTask := range runtimeTasks {
		record, err := s.storage.TaskStore().GetTask(context.Background(), runtimeTask.TaskID)
		if err != nil {
			if storage.IsTaskRecordNotFound(err) {
				count++
			}
			continue
		}
		storedTask, ok := s.structuredTaskRecordToRuntime(record, false)
		if !ok || !matchesTaskGroup(storedTask, group) {
			count++
		}
	}
	return count
}

// taskQueryViews caches runtime and storage-backed task snapshots for one
// request so overview endpoints can reuse one merged task-centric read model
// without reloading the full task table for every widget.
type taskQueryViews struct {
	service      *Service
	runtimeTasks map[string][]runengine.TaskRecord
	mergedTasks  map[string][]runengine.TaskRecord
	storageTasks []runengine.TaskRecord
	storageReady bool
}

func newTaskQueryViews(service *Service) *taskQueryViews {
	return &taskQueryViews{
		service:      service,
		runtimeTasks: make(map[string][]runengine.TaskRecord, 2),
		mergedTasks:  make(map[string][]runengine.TaskRecord, 2),
	}
}

// tasks returns one merged task-centric view for the requested group and sort
// order, reusing the same storage snapshot for the whole RPC request.
func (q *taskQueryViews) tasks(group, sortBy, sortOrder string) []runengine.TaskRecord {
	key := strings.Join([]string{group, sortBy, sortOrder}, "|")
	if tasks, ok := q.mergedTasks[key]; ok {
		return tasks
	}
	runtimeTasks := q.runtime(group, sortBy, sortOrder)
	storageTasks := filterAndSortTasks(q.loadStorage(), group, sortBy, sortOrder)
	merged := mergeTaskLists(runtimeTasks, storageTasks)
	if len(merged) > 0 {
		runengineSortTaskRecords(merged, sortBy, sortOrder)
	}
	q.mergedTasks[key] = merged
	return merged
}

func (q *taskQueryViews) hasRuntimeState() bool {
	return len(q.runtime("unfinished", "updated_at", "desc")) > 0 ||
		len(q.runtime("finished", "finished_at", "desc")) > 0
}

func (q *taskQueryViews) runtime(group, sortBy, sortOrder string) []runengine.TaskRecord {
	key := strings.Join([]string{group, sortBy, sortOrder}, "|")
	if tasks, ok := q.runtimeTasks[key]; ok {
		return tasks
	}
	tasks, _ := q.service.runEngine.ListTasks(group, sortBy, sortOrder, 0, 0)
	q.runtimeTasks[key] = tasks
	return tasks
}

func (q *taskQueryViews) loadStorage() []runengine.TaskRecord {
	if q.storageReady {
		return q.storageTasks
	}
	q.storageTasks = q.service.loadAllTasksFromStorage()
	q.storageReady = true
	return q.storageTasks
}

func filterAndSortTasks(tasks []runengine.TaskRecord, group, sortBy, sortOrder string) []runengine.TaskRecord {
	if len(tasks) == 0 {
		return nil
	}
	filtered := make([]runengine.TaskRecord, 0, len(tasks))
	for _, task := range tasks {
		if matchesTaskGroup(task, group) {
			filtered = append(filtered, task)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	runengineSortTaskRecords(filtered, sortBy, sortOrder)
	return filtered
}

func mergeTaskLists(runtimeTasks, storageTasks []runengine.TaskRecord) []runengine.TaskRecord {
	if len(runtimeTasks) == 0 {
		return storageTasks
	}
	if len(storageTasks) == 0 {
		return runtimeTasks
	}
	runtimeByID := make(map[string]runengine.TaskRecord, len(runtimeTasks))
	for _, task := range runtimeTasks {
		runtimeByID[task.TaskID] = task
	}
	merged := make([]runengine.TaskRecord, 0, len(runtimeTasks)+len(storageTasks))
	seen := make(map[string]struct{}, len(runtimeTasks)+len(storageTasks))
	for _, task := range storageTasks {
		if runtimeTask, ok := runtimeByID[task.TaskID]; ok {
			merged = append(merged, fresherTaskRecord(runtimeTask, task))
			seen[task.TaskID] = struct{}{}
			continue
		}
		merged = append(merged, task)
		seen[task.TaskID] = struct{}{}
	}
	for _, task := range runtimeTasks {
		if _, ok := seen[task.TaskID]; ok {
			continue
		}
		merged = append(merged, task)
	}
	return merged
}

func fresherTaskRecord(runtimeTask, storageTask runengine.TaskRecord) runengine.TaskRecord {
	selected := storageTask
	if runtimeTask.UpdatedAt.After(storageTask.UpdatedAt) {
		selected = runtimeTask
	} else if storageTask.UpdatedAt.After(runtimeTask.UpdatedAt) {
		selected = storageTask
	} else if runtimeTask.FinishedAt != nil && storageTask.FinishedAt == nil {
		selected = runtimeTask
	} else if storageTask.FinishedAt != nil && runtimeTask.FinishedAt == nil {
		selected = storageTask
	}
	return taskRecordWithSnapshotAnchors(selected, runtimeTask, storageTask)
}

func taskRecordWithSnapshotAnchors(selected, runtimeTask, storageTask runengine.TaskRecord) runengine.TaskRecord {
	// Snapshot anchors are continuation evidence, not freshness state; keep the
	// fresher task fields and only fill missing anchors from the alternate
	// copies. A partial fresher snapshot can still carry text or files while
	// missing the page/window anchors needed for follow-up routing.
	selected.Snapshot = snapshotWithMissingAnchors(selected.Snapshot, runtimeTask.Snapshot)
	selected.Snapshot = snapshotWithMissingAnchors(selected.Snapshot, storageTask.Snapshot)
	return selected
}

func snapshotWithMissingAnchors(selected, fallback contextsvc.TaskContextSnapshot) contextsvc.TaskContextSnapshot {
	if isEmptySnapshot(selected) {
		if isEmptySnapshot(fallback) {
			return selected
		}
		return cloneTaskSnapshot(fallback)
	}
	if isEmptySnapshot(fallback) {
		return cloneTaskSnapshot(selected)
	}
	merged := cloneTaskSnapshot(selected)
	if isShellBallIntakeAnchor(merged) && !isShellBallIntakeAnchor(fallback) {
		// Shell-ball intake context is not a task-specific anchor. Treat it as
		// missing when another persisted copy still has the real page/window
		// anchors needed for continuation routing.
		merged.PageTitle = ""
		merged.PageURL = ""
		merged.AppName = ""
		merged.WindowTitle = ""
	}
	if strings.TrimSpace(merged.PageTitle) == "" {
		merged.PageTitle = fallback.PageTitle
	}
	if strings.TrimSpace(merged.PageURL) == "" {
		merged.PageURL = fallback.PageURL
	}
	if strings.TrimSpace(merged.AppName) == "" {
		merged.AppName = fallback.AppName
	}
	if strings.TrimSpace(merged.BrowserKind) == "" {
		merged.BrowserKind = fallback.BrowserKind
	}
	if strings.TrimSpace(merged.ProcessPath) == "" {
		merged.ProcessPath = fallback.ProcessPath
	}
	if merged.ProcessID == 0 {
		merged.ProcessID = fallback.ProcessID
	}
	if strings.TrimSpace(merged.WindowTitle) == "" {
		merged.WindowTitle = fallback.WindowTitle
	}
	if strings.TrimSpace(merged.HoverTarget) == "" {
		merged.HoverTarget = fallback.HoverTarget
	}
	return merged
}

func (s *Service) taskDetailFromStorage(taskID string) (runengine.TaskRecord, bool) {
	if s.storage == nil || strings.TrimSpace(taskID) == "" {
		return runengine.TaskRecord{}, false
	}
	if s.storage.TaskStore() != nil {
		if task, record, ok := s.taskDetailFromStructuredStorage(taskID); ok {
			if structuredTaskNeedsTaskRunFallback(record, task) {
				if taskRunTask, taskRunOK := s.taskDetailFromTaskRunStorage(taskID); taskRunOK {
					task = mergeStructuredTaskDetailCompatibility(task, taskRunTask)
				}
			}
			return task, true
		}
	}
	if taskRunTask, ok := s.taskDetailFromTaskRunStorage(taskID); ok {
		return taskRunTask, true
	}
	return runengine.TaskRecord{}, false
}

func (s *Service) taskDetailFromTaskRunStorage(taskID string) (runengine.TaskRecord, bool) {
	if s.storage == nil || s.storage.TaskRunStore() == nil || strings.TrimSpace(taskID) == "" {
		return runengine.TaskRecord{}, false
	}
	record, err := s.storage.TaskRunStore().GetTaskRun(context.Background(), taskID)
	if err != nil {
		return runengine.TaskRecord{}, false
	}
	return taskRecordFromStorage(record), true
}

// structuredTaskNeedsTaskRunFallback keeps task-run reads as a recovery path
// whenever snapshot_json is missing or malformed because several legacy detail
// fields still only exist in compatibility snapshots today.
func structuredTaskNeedsTaskRunFallback(record storage.TaskRecord, _ runengine.TaskRecord) bool {
	if strings.TrimSpace(record.SnapshotJSON) != "" {
		if _, err := storageTaskRunRecordFromSnapshotJSON(record.SnapshotJSON); err == nil {
			return false
		}
	}
	return true
}

// mergeStructuredTaskDetailCompatibility fills task-detail fields that are
// still sourced from task-run snapshots while the first-class task tables are
// being rolled out. The structured row stays authoritative and the task-run
// snapshot only backfills fields the structured read could not rebuild.
func mergeStructuredTaskDetailCompatibility(task, taskRunTask runengine.TaskRecord) runengine.TaskRecord {
	attemptScopedFormalReads := taskUsesAttemptScopedFormalReads(task)
	sameAttemptSnapshot := strings.TrimSpace(task.RunID) != "" && strings.TrimSpace(task.RunID) == strings.TrimSpace(taskRunTask.RunID)
	if task.FinishedAt == nil && taskRunTask.FinishedAt != nil {
		task.FinishedAt = cloneTimePointer(taskRunTask.FinishedAt)
	}
	if len(task.Timeline) == 0 {
		task.Timeline = append([]runengine.TaskStepRecord(nil), taskRunTask.Timeline...)
	}
	if isEmptySnapshot(task.Snapshot) {
		task.Snapshot = cloneTaskSnapshot(taskRunTask.Snapshot)
	}
	if len(task.BubbleMessage) == 0 {
		task.BubbleMessage = cloneMap(taskRunTask.BubbleMessage)
	}
	if len(task.DeliveryResult) == 0 && (!attemptScopedFormalReads || sameAttemptSnapshot) {
		task.DeliveryResult = cloneMap(taskRunTask.DeliveryResult)
	}
	if len(task.Artifacts) == 0 && (!attemptScopedFormalReads || sameAttemptSnapshot) {
		task.Artifacts = cloneMapSlice(taskRunTask.Artifacts)
	}
	if len(task.Citations) == 0 && (!attemptScopedFormalReads || sameAttemptSnapshot) {
		task.Citations = cloneMapSlice(taskRunTask.Citations)
	}
	if len(task.AuditRecords) == 0 && (!attemptScopedFormalReads || sameAttemptSnapshot) {
		task.AuditRecords = cloneMapSlice(taskRunTask.AuditRecords)
	}
	if len(task.MirrorReferences) == 0 {
		task.MirrorReferences = cloneMapSlice(taskRunTask.MirrorReferences)
	}
	if len(task.SecuritySummary) == 0 {
		task.SecuritySummary = cloneMap(taskRunTask.SecuritySummary)
	} else {
		for key, value := range taskRunTask.SecuritySummary {
			if _, exists := task.SecuritySummary[key]; !exists {
				task.SecuritySummary[key] = value
			}
		}
	}
	if len(task.ApprovalRequest) == 0 {
		task.ApprovalRequest = cloneMap(taskRunTask.ApprovalRequest)
	}
	if len(task.PendingExecution) == 0 {
		task.PendingExecution = cloneMap(taskRunTask.PendingExecution)
	}
	if len(task.Authorization) == 0 && (!attemptScopedFormalReads || sameAttemptSnapshot) {
		task.Authorization = cloneMap(taskRunTask.Authorization)
	}
	if len(task.ImpactScope) == 0 {
		task.ImpactScope = cloneMap(taskRunTask.ImpactScope)
	}
	if len(task.TokenUsage) == 0 {
		task.TokenUsage = cloneMap(taskRunTask.TokenUsage)
	}
	if len(task.LatestEvent) == 0 && !attemptScopedFormalReads {
		task.LatestEvent = cloneMap(taskRunTask.LatestEvent)
	}
	if len(task.LatestToolCall) == 0 && !attemptScopedFormalReads {
		task.LatestToolCall = cloneMap(taskRunTask.LatestToolCall)
	}
	if strings.TrimSpace(task.LoopStopReason) == "" && !attemptScopedFormalReads {
		task.LoopStopReason = taskRunTask.LoopStopReason
	}
	if len(task.SteeringMessages) == 0 {
		task.SteeringMessages = append([]string(nil), taskRunTask.SteeringMessages...)
	}
	if strings.TrimSpace(task.CurrentStepStatus) == "" {
		task.CurrentStepStatus = taskRunTask.CurrentStepStatus
	}
	return task
}

// taskUsesAttemptScopedFormalReads keeps task detail pinned to the active run
// once restart allocates a fresh attempt under the same task_id.
func taskUsesAttemptScopedFormalReads(task runengine.TaskRecord) bool {
	runID := strings.TrimSpace(task.RunID)
	if runID == "" {
		return false
	}
	primaryRunID := strings.TrimSpace(task.PrimaryRunID)
	if primaryRunID != "" {
		if runID != primaryRunID {
			return true
		}
		// Legacy task_run snapshots may collapse the original primary run onto the
		// current run_id during reload. Keep the execution-attempt fallback active
		// for that shape so restart attempts do not reopen task-scoped formal reads.
		return task.ExecutionAttempt > 1
	}
	return task.ExecutionAttempt > 1
}

func taskAttemptRunIDFilter(task runengine.TaskRecord) string {
	if !taskUsesAttemptScopedFormalReads(task) {
		return ""
	}
	return task.RunID
}

// isPreparedRestartAttempt reports whether the caller is working with a staged
// restart snapshot whose run_id is not yet the live runtime record.
func (s *Service) isPreparedRestartAttempt(task runengine.TaskRecord) bool {
	if s == nil || s.runEngine == nil || strings.TrimSpace(task.TaskID) == "" {
		return false
	}
	currentTask, ok := s.runEngine.GetTask(task.TaskID)
	if !ok {
		return false
	}
	return currentTask.RunID != task.RunID
}

func formalReadTask(taskID string, engine *runengine.Engine, loadFromStorage func(string) (runengine.TaskRecord, bool)) (runengine.TaskRecord, bool) {
	if engine != nil {
		if task, ok := engine.GetTask(taskID); ok {
			return task, true
		}
	}
	if loadFromStorage == nil {
		return runengine.TaskRecord{}, false
	}
	return loadFromStorage(taskID)
}

// latestAttemptDeliveryResultFromStorage restores the newest first-class
// delivery_result for the task detail attempt that is currently active. Restart
// attempts must not rehydrate a previous run's formal output while the new run
// is still processing the same task_id.
func (s *Service) latestAttemptDeliveryResultFromStorage(task runengine.TaskRecord) map[string]any {
	if s == nil || s.storage == nil || s.storage.LoopRuntimeStore() == nil || strings.TrimSpace(task.TaskID) == "" {
		return nil
	}
	record, ok, err := s.storage.LoopRuntimeStore().GetLatestDeliveryResult(context.Background(), task.TaskID, taskAttemptRunIDFilter(task))
	if err != nil || !ok {
		return nil
	}
	payload := map[string]any{}
	if strings.TrimSpace(record.PayloadJSON) != "" {
		if err := json.Unmarshal([]byte(record.PayloadJSON), &payload); err != nil {
			payload = map[string]any{}
		}
	}
	return map[string]any{
		"type":         record.Type,
		"title":        record.Title,
		"payload":      payload,
		"preview_text": record.PreviewText,
	}
}

// loadAttemptTaskCitationsFromStorage restores the current formal citation chain
// for the active task attempt when task_run snapshots are unavailable. Restarted
// tasks keep previous attempts under the same task_id, so task detail must not
// reuse older run evidence once a fresh run_id exists.
func (s *Service) loadAttemptTaskCitationsFromStorage(task runengine.TaskRecord) []map[string]any {
	if s == nil || s.storage == nil || s.storage.LoopRuntimeStore() == nil || strings.TrimSpace(task.TaskID) == "" {
		return nil
	}
	records, err := s.storage.LoopRuntimeStore().ListTaskCitations(context.Background(), task.TaskID, taskAttemptRunIDFilter(task))
	if err != nil {
		return nil
	}
	citations := make([]map[string]any, 0, len(records))
	for _, record := range records {
		citation := map[string]any{
			"citation_id": record.CitationID,
			"task_id":     record.TaskID,
			"run_id":      record.RunID,
			"source_type": record.SourceType,
			"source_ref":  record.SourceRef,
			"label":       record.Label,
		}
		if strings.TrimSpace(record.ArtifactID) != "" {
			citation["artifact_id"] = record.ArtifactID
		}
		if strings.TrimSpace(record.ArtifactType) != "" {
			citation["artifact_type"] = record.ArtifactType
		}
		if strings.TrimSpace(record.EvidenceRole) != "" {
			citation["evidence_role"] = record.EvidenceRole
		}
		if strings.TrimSpace(record.ExcerptText) != "" {
			citation["excerpt_text"] = record.ExcerptText
		}
		if strings.TrimSpace(record.ScreenSessionID) != "" {
			citation["screen_session_id"] = record.ScreenSessionID
		}
		citations = append(citations, citation)
	}
	return citations
}

func (s *Service) taskDetailFromStructuredStorage(taskID string) (runengine.TaskRecord, storage.TaskRecord, bool) {
	record, err := s.storage.TaskStore().GetTask(context.Background(), taskID)
	if err != nil {
		if storage.IsTaskRecordNotFound(err) {
			return runengine.TaskRecord{}, storage.TaskRecord{}, false
		}
		return runengine.TaskRecord{}, storage.TaskRecord{}, false
	}
	task, ok := s.structuredTaskRecordToRuntime(record, true)
	return task, record, ok
}

func matchesTaskGroup(task runengine.TaskRecord, group string) bool {
	switch group {
	case "finished":
		return isFinishedTaskStatus(task.Status)
	default:
		return !isFinishedTaskStatus(task.Status)
	}
}

func isFinishedTaskStatus(status string) bool {
	switch status {
	case "completed", "cancelled", "ended_unfinished", "failed":
		return true
	default:
		return false
	}
}

func runengineSortTaskRecords(tasks []runengine.TaskRecord, sortBy, sortOrder string) {
	switch sortBy {
	case "started_at", "finished_at", "updated_at":
	default:
		sortBy = "updated_at"
	}
	if sortOrder != "asc" {
		sortOrder = "desc"
	}
	sort.SliceStable(tasks, func(i, j int) bool {
		left := taskSortTime(tasks[i], sortBy)
		right := taskSortTime(tasks[j], sortBy)
		if left.Equal(right) {
			leftUpdated := tasks[i].UpdatedAt
			rightUpdated := tasks[j].UpdatedAt
			if leftUpdated.Equal(rightUpdated) {
				if sortOrder == "asc" {
					return tasks[i].TaskID < tasks[j].TaskID
				}
				return tasks[i].TaskID > tasks[j].TaskID
			}
			if sortOrder == "asc" {
				return leftUpdated.Before(rightUpdated)
			}
			return leftUpdated.After(rightUpdated)
		}
		if sortOrder == "asc" {
			return left.Before(right)
		}
		return left.After(right)
	})
}

func countPendingApprovalTasks(tasks []runengine.TaskRecord) int {
	count := 0
	for _, task := range tasks {
		if task.Status == "waiting_auth" && len(task.ApprovalRequest) != 0 {
			count++
		}
	}
	return count
}

func taskSortTime(task runengine.TaskRecord, sortBy string) time.Time {
	switch sortBy {
	case "started_at":
		return task.StartedAt
	case "finished_at":
		if task.FinishedAt != nil {
			return *task.FinishedAt
		}
		return time.Time{}
	default:
		return task.UpdatedAt
	}
}

func taskRecordFromStorage(record storage.TaskRunRecord) runengine.TaskRecord {
	executionAttempt := record.ExecutionAttempt
	if executionAttempt <= 0 {
		executionAttempt = 1
	}
	return runengine.TaskRecord{
		TaskID:            record.TaskID,
		SessionID:         record.SessionID,
		RunID:             record.RunID,
		PrimaryRunID:      record.RunID,
		RequestSource:     firstNonEmptyString(strings.TrimSpace(record.RequestSource), strings.TrimSpace(record.Snapshot.Source)),
		RequestTrigger:    firstNonEmptyString(strings.TrimSpace(record.RequestTrigger), strings.TrimSpace(record.Snapshot.Trigger)),
		ExecutionAttempt:  executionAttempt,
		Title:             record.Title,
		SourceType:        record.SourceType,
		Status:            record.Status,
		Intent:            cloneMap(record.Intent),
		PreferredDelivery: record.PreferredDelivery,
		FallbackDelivery:  record.FallbackDelivery,
		CurrentStep:       record.CurrentStep,
		RiskLevel:         record.RiskLevel,
		StartedAt:         record.StartedAt,
		UpdatedAt:         record.UpdatedAt,
		FinishedAt:        cloneTimePointer(record.FinishedAt),
		Timeline:          timelineFromStorage(record.Timeline),
		BubbleMessage:     cloneMap(record.BubbleMessage),
		DeliveryResult:    cloneMap(record.DeliveryResult),
		Artifacts:         cloneMapSlice(record.Artifacts),
		Citations:         cloneMapSlice(record.Citations),
		AuditRecords:      cloneMapSlice(record.AuditRecords),
		MirrorReferences:  cloneMapSlice(record.MirrorReferences),
		SecuritySummary:   cloneMap(record.SecuritySummary),
		ApprovalRequest:   cloneMap(record.ApprovalRequest),
		PendingExecution:  cloneMap(record.PendingExecution),
		Authorization:     cloneMap(record.Authorization),
		ImpactScope:       cloneMap(record.ImpactScope),
		TokenUsage:        cloneMap(record.TokenUsage),
		MemoryReadPlans:   cloneMapSlice(record.MemoryReadPlans),
		MemoryWritePlans:  cloneMapSlice(record.MemoryWritePlans),
		StorageWritePlan:  cloneMap(record.StorageWritePlan),
		ArtifactPlans:     cloneMapSlice(record.ArtifactPlans),
		LatestEvent:       cloneMap(record.LatestEvent),
		LatestToolCall:    cloneMap(record.LatestToolCall),
		LoopStopReason:    record.LoopStopReason,
		SteeringMessages:  append([]string(nil), record.SteeringMessages...),
		CurrentStepStatus: record.CurrentStepStatus,
	}
}

// structuredTaskRecordToRuntime builds one task-centric read model. List callers
// pass includeCompatibility=false so each row only uses first-class task fields;
// detail callers opt into timeline, formal artifacts, governance, and snapshot
// compatibility reads.
func (s *Service) structuredTaskRecordToRuntime(record storage.TaskRecord, includeCompatibility bool) (runengine.TaskRecord, bool) {
	startedAt, err := time.Parse(time.RFC3339Nano, record.StartedAt)
	if err != nil {
		return runengine.TaskRecord{}, false
	}
	updatedAt, err := time.Parse(time.RFC3339Nano, record.UpdatedAt)
	if err != nil {
		return runengine.TaskRecord{}, false
	}
	var finishedAt *time.Time
	if strings.TrimSpace(record.FinishedAt) != "" {
		parsedFinishedAt, err := time.Parse(time.RFC3339Nano, record.FinishedAt)
		if err == nil {
			finishedAt = &parsedFinishedAt
		}
	}
	intentArguments := map[string]any{}
	if strings.TrimSpace(record.IntentArgumentsJSON) != "" {
		if err := json.Unmarshal([]byte(record.IntentArgumentsJSON), &intentArguments); err != nil {
			intentArguments = map[string]any{}
		}
	}
	runtime := runengine.TaskRecord{
		TaskID:            record.TaskID,
		SessionID:         record.SessionID,
		RunID:             strings.TrimSpace(record.RunID),
		PrimaryRunID:      firstNonEmptyString(strings.TrimSpace(record.PrimaryRunID), strings.TrimSpace(record.RunID)),
		RequestSource:     record.RequestSource,
		RequestTrigger:    record.RequestTrigger,
		Title:             record.Title,
		SourceType:        record.SourceType,
		Status:            record.Status,
		Intent:            map[string]any{"name": record.IntentName, "arguments": intentArguments},
		PreferredDelivery: record.PreferredDelivery,
		FallbackDelivery:  record.FallbackDelivery,
		CurrentStep:       record.CurrentStep,
		RiskLevel:         record.RiskLevel,
		StartedAt:         startedAt,
		UpdatedAt:         updatedAt,
		FinishedAt:        finishedAt,
		CurrentStepStatus: record.CurrentStepStatus,
	}
	if strings.TrimSpace(runtime.Title) == "" || strings.TrimSpace(runtime.SessionID) == "" || strings.TrimSpace(runtime.LoopStopReason) == "" {
		s.hydrateStructuredTaskSessionAndRun(&runtime)
	}
	if !includeCompatibility {
		return runtime, true
	}

	runtime.Timeline = s.taskTimelineFromStructuredStorage(record.TaskID)
	s.hydrateStructuredTaskFormalArtifacts(&runtime)
	s.hydrateStructuredTaskGovernance(&runtime)

	var snapshotCompatibility runengine.TaskRecord
	var snapshotCompatibilityOK bool
	if strings.TrimSpace(record.SnapshotJSON) != "" {
		snapshot, err := storageTaskRunRecordFromSnapshotJSON(record.SnapshotJSON)
		if err == nil {
			snapshotCompatibility = taskRecordFromStorage(snapshot)
			snapshotCompatibilityOK = true
		}
	}
	if snapshotCompatibilityOK {
		runtime = mergeStructuredTaskDetailCompatibility(runtime, snapshotCompatibility)
	}
	return runtime, true
}

// hydrateStructuredTaskFormalArtifacts rebuilds task-facing evidence fields from
// first-class stores before any task_run compatibility fallback is considered.
func (s *Service) hydrateStructuredTaskFormalArtifacts(task *runengine.TaskRecord) {
	if s == nil || s.storage == nil || task == nil {
		return
	}
	task.Artifacts = s.loadAttemptArtifactsFromStorage(*task, 0, 0)
	task.Citations = s.loadAttemptTaskCitationsFromStorage(*task)
	task.AuditRecords = s.loadAttemptAuditRecordsFromStorage(*task, 0, 0)
	task.LatestToolCall = s.latestToolCallFromStorage(task.TaskID, task.RunID)
	if deliveryResult := s.latestAttemptDeliveryResultFromStorage(*task); deliveryResult != nil {
		task.DeliveryResult = deliveryResult
	}
}

// hydrateStructuredTaskSessionAndRun uses the first-class sessions/runs stores
// to keep the formal `session -> task -> run` linkage queryable even when the
// legacy task_run snapshot bridge is absent.
func (s *Service) hydrateStructuredTaskSessionAndRun(task *runengine.TaskRecord) {
	if s == nil || s.storage == nil || task == nil {
		return
	}
	if s.storage.SessionStore() != nil && strings.TrimSpace(task.SessionID) != "" {
		if session, err := s.storage.SessionStore().GetSession(context.Background(), task.SessionID); err == nil {
			if strings.TrimSpace(task.Title) == "" {
				task.Title = session.Title
			}
			if strings.TrimSpace(task.SessionID) == "" {
				task.SessionID = session.SessionID
			}
		}
	}
	if s.storage.LoopRuntimeStore() != nil && strings.TrimSpace(task.RunID) != "" {
		if runRecord, err := s.storage.LoopRuntimeStore().GetRun(context.Background(), task.RunID); err == nil {
			if strings.TrimSpace(task.SessionID) == "" {
				task.SessionID = runRecord.SessionID
			}
			if strings.TrimSpace(task.LoopStopReason) == "" {
				task.LoopStopReason = runRecord.StopReason
			}
		}
	}
}

// hydrateStructuredTaskGovernance rebuilds the task-facing governance fields
// from first-class stores when the snapshot bridge is unavailable.
func (s *Service) hydrateStructuredTaskGovernance(task *runengine.TaskRecord) {
	if s == nil || s.storage == nil || task == nil {
		return
	}
	if authorizationRecord := s.latestAttemptAuthorizationRecordFromStorage(*task); authorizationRecord != nil {
		task.Authorization = authorizationRecord
	}
	if deliveryResult := s.latestAttemptDeliveryResultFromStorage(*task); len(deliveryResult) > 0 {
		task.DeliveryResult = deliveryResult
	}
	if citations := s.loadAttemptTaskCitationsFromStorage(*task); len(citations) > 0 {
		task.Citations = citations
	}
	securitySummary := cloneMap(task.SecuritySummary)
	if securitySummary == nil {
		securitySummary = map[string]any{}
	}
	if approvalRequest := s.pendingApprovalRequestFromStorage(task.TaskID, task.RiskLevel); approvalRequest != nil {
		task.ApprovalRequest = approvalRequest
		securitySummary["pending_authorizations"] = 1
		if strings.TrimSpace(stringValue(approvalRequest, "risk_level", "")) != "" {
			securitySummary["security_status"] = "pending_confirmation"
		}
	} else if task.Status == "waiting_auth" {
		securitySummary["pending_authorizations"] = 0
	}
	if latestRestorePoint := s.latestRestorePointFromStorage(task.TaskID); latestRestorePoint != nil {
		securitySummary["latest_restore_point"] = latestRestorePoint
	}
	task.SecuritySummary = securitySummary
}

// selectTaskDetailAuthorizationRecord prefers the newest formal authorization
// record so task detail does not regress to snapshot-era governance anchors once
// first-class authorization storage is available.
func selectTaskDetailAuthorizationRecord(taskID string, runtimeRecord map[string]any, storageRecord map[string]any) map[string]any {
	normalizedRuntime := normalizeTaskDetailAuthorizationRecord(taskID, runtimeRecord)
	normalizedStorage := normalizeTaskDetailAuthorizationRecord(taskID, storageRecord)
	return preferNewerTaskDetailRecord(normalizedRuntime, normalizedStorage, "created_at")
}

// selectTaskDetailAuditRecord keeps screen tasks anchored to the screen-evidence
// audit chain even when newer generic delivery/runtime audits exist later in the
// same task. Non-screen tasks still use the latest normalized audit record.
func selectTaskDetailAuditRecord(task runengine.TaskRecord, runtimeAuditRecords []map[string]any, storageAuditRecords []map[string]any) map[string]any {
	latestOverall := latestNormalizedTaskAuditRecord(task.TaskID, runtimeAuditRecords, storageAuditRecords)
	if !isScreenTaskDetail(task) {
		return latestOverall
	}
	latestScreen := latestScreenTaskAuditRecord(task.TaskID, runtimeAuditRecords, storageAuditRecords)
	if latestScreen == nil {
		return latestOverall
	}
	if shouldPreferLatestTaskAuditOverScreenAudit(latestOverall, latestScreen) {
		return latestOverall
	}
	return latestScreen
}

// shouldPreferLatestTaskAuditOverScreenAudit keeps screen tasks anchored to
// screen evidence by default, but lets newer terminal governance records such as
// failures or restore_apply outcomes override stale screen-capture success logs.
func shouldPreferLatestTaskAuditOverScreenAudit(latestOverall map[string]any, latestScreen map[string]any) bool {
	if len(latestOverall) == 0 {
		return false
	}
	if len(latestScreen) == 0 {
		return true
	}
	if !parseTaskDetailRecordTime(stringValue(latestOverall, "created_at", "")).After(parseTaskDetailRecordTime(stringValue(latestScreen, "created_at", ""))) {
		return false
	}
	if isScreenTaskAuditRecord(latestOverall) {
		return true
	}
	return isTerminalGovernanceAuditRecord(latestOverall)
}

func latestNormalizedTaskAuditRecord(taskID string, auditGroups ...[]map[string]any) map[string]any {
	var latest map[string]any
	for _, group := range auditGroups {
		for _, auditRecord := range group {
			normalized := normalizeTaskDetailAuditRecord(taskID, auditRecord)
			if normalized == nil {
				continue
			}
			latest = preferNewerTaskDetailRecord(latest, normalized, "created_at")
		}
	}
	return latest
}

func latestScreenTaskAuditRecord(taskID string, auditGroups ...[]map[string]any) map[string]any {
	var latest map[string]any
	for _, group := range auditGroups {
		for _, auditRecord := range group {
			normalized := normalizeTaskDetailAuditRecord(taskID, auditRecord)
			if normalized == nil || !isScreenTaskAuditRecord(normalized) {
				continue
			}
			latest = preferNewerTaskDetailRecord(latest, normalized, "created_at")
		}
	}
	return latest
}

func isScreenTaskAuditRecord(auditRecord map[string]any) bool {
	if len(auditRecord) == 0 {
		return false
	}
	if strings.TrimSpace(stringValue(auditRecord, "type", "")) == "screen_capture" {
		return true
	}
	if strings.HasPrefix(strings.TrimSpace(stringValue(auditRecord, "action", "")), "screen.capture.") {
		return true
	}
	target := strings.ToLower(strings.TrimSpace(stringValue(auditRecord, "target", "")))
	return strings.Contains(target, "screen")
}

func isTerminalGovernanceAuditRecord(auditRecord map[string]any) bool {
	if len(auditRecord) == 0 {
		return false
	}
	result := strings.TrimSpace(stringValue(auditRecord, "result", ""))
	if result != "" && result != "success" {
		return true
	}
	action := strings.TrimSpace(stringValue(auditRecord, "action", ""))
	if strings.HasPrefix(action, "restore_") || strings.HasPrefix(action, "authorization_") {
		return true
	}
	return strings.TrimSpace(stringValue(auditRecord, "type", "")) == "recovery"
}

func isScreenTaskDetail(task runengine.TaskRecord) bool {
	if stringValue(task.Intent, "name", "") == "screen_analyze" || strings.TrimSpace(task.SourceType) == "screen_capture" {
		return true
	}
	if strings.TrimSpace(stringValue(task.PendingExecution, "kind", "")) == "screen_analysis" {
		return true
	}
	for _, artifact := range task.Artifacts {
		if strings.TrimSpace(stringValue(artifact, "artifact_type", "")) == "screen_capture" {
			return true
		}
	}
	for _, citation := range task.Citations {
		if strings.TrimSpace(stringValue(citation, "artifact_type", "")) == "screen_capture" || strings.TrimSpace(stringValue(citation, "screen_session_id", "")) != "" {
			return true
		}
	}
	return strings.TrimSpace(stringValue(task.ApprovalRequest, "operation_name", "")) == "screen_capture"
}

func preferNewerTaskDetailRecord(left map[string]any, right map[string]any, timeKey string) map[string]any {
	if len(left) == 0 {
		return cloneMap(right)
	}
	if len(right) == 0 {
		return cloneMap(left)
	}
	leftTime := parseTaskDetailRecordTime(stringValue(left, timeKey, ""))
	rightTime := parseTaskDetailRecordTime(stringValue(right, timeKey, ""))
	if rightTime.After(leftTime) {
		return cloneMap(right)
	}
	return cloneMap(left)
}

func parseTaskDetailRecordTime(value string) time.Time {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}
	}
	if parsed, err := time.Parse(time.RFC3339Nano, trimmed); err == nil {
		return parsed
	}
	if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return parsed
	}
	return time.Time{}
}

func (s *Service) pendingApprovalRequestFromStorage(taskID, fallbackRiskLevel string) map[string]any {
	if s == nil || s.storage == nil || s.storage.ApprovalRequestStore() == nil || strings.TrimSpace(taskID) == "" {
		return nil
	}
	records, _, err := s.storage.ApprovalRequestStore().ListApprovalRequests(context.Background(), taskID, 0, 0)
	if err != nil || len(records) == 0 {
		return nil
	}
	for _, record := range records {
		approvalRequest := normalizeTaskDetailApprovalRequest(taskID, fallbackRiskLevel, approvalRequestRecordToMap(record))
		if approvalRequest != nil {
			return approvalRequest
		}
	}
	return nil
}

func (s *Service) latestAttemptAuthorizationRecordFromStorage(task runengine.TaskRecord) map[string]any {
	if s == nil || s.storage == nil || s.storage.AuthorizationRecordStore() == nil || strings.TrimSpace(task.TaskID) == "" {
		return nil
	}
	items, _, err := s.storage.AuthorizationRecordStore().ListAuthorizationRecords(context.Background(), task.TaskID, taskAttemptRunIDFilter(task), 1, 0)
	if err != nil || len(items) == 0 {
		return nil
	}
	return normalizeTaskDetailAuthorizationRecord(task.TaskID, authorizationRecordRecordToMap(items[0]))
}

func approvalRequestRecordToMap(record storage.ApprovalRequestRecord) map[string]any {
	result := map[string]any{
		"approval_id":    record.ApprovalID,
		"task_id":        record.TaskID,
		"operation_name": record.OperationName,
		"risk_level":     record.RiskLevel,
		"target_object":  record.TargetObject,
		"reason":         record.Reason,
		"status":         record.Status,
		"created_at":     record.CreatedAt,
		"updated_at":     record.UpdatedAt,
	}
	if strings.TrimSpace(record.ImpactScopeJSON) != "" {
		var scope map[string]any
		if err := json.Unmarshal([]byte(record.ImpactScopeJSON), &scope); err == nil && len(scope) > 0 {
			result["impact_scope"] = scope
		}
	}
	return result
}

func authorizationRecordRecordToMap(record storage.AuthorizationRecordRecord) map[string]any {
	return map[string]any{
		"authorization_record_id": record.AuthorizationRecordID,
		"task_id":                 record.TaskID,
		"run_id":                  record.RunID,
		"approval_id":             record.ApprovalID,
		"decision":                record.Decision,
		"remember_rule":           record.RememberRule,
		"operator":                record.Operator,
		"created_at":              record.CreatedAt,
	}
}

func (s *Service) taskTimelineFromStructuredStorage(taskID string) []runengine.TaskStepRecord {
	if s.storage == nil || s.storage.TaskStepStore() == nil {
		return nil
	}
	records, _, err := s.storage.TaskStepStore().ListTaskSteps(context.Background(), taskID, 0, 0)
	if err != nil || len(records) == 0 {
		return nil
	}
	result := make([]runengine.TaskStepRecord, 0, len(records))
	for _, step := range records {
		result = append(result, runengine.TaskStepRecord{
			StepID:        step.StepID,
			TaskID:        step.TaskID,
			Name:          step.Name,
			Status:        step.Status,
			OrderIndex:    step.OrderIndex,
			InputSummary:  step.InputSummary,
			OutputSummary: step.OutputSummary,
		})
	}
	return result
}
