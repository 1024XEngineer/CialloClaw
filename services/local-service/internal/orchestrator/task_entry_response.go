package orchestrator

import "github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"

// persistTaskPresentation stores the current lightweight presentation on the
// task record before the response leaves the orchestrator. Confirmation and
// waiting-input branches use this path because no execution result will later
// refresh the task projection for the frontend.
func (s *Service) persistTaskPresentation(task runengine.TaskRecord, bubble map[string]any) runengine.TaskRecord {
	if _, ok := s.runEngine.SetPresentation(task.TaskID, bubble, nil, nil); !ok {
		return task
	}
	updatedTask, ok := s.runEngine.GetTask(task.TaskID)
	if !ok {
		return task
	}
	return updatedTask
}

// buildTaskEntryResponse centralizes the protocol-facing result shape shared
// by agent.input.submit and agent.task.start. Business branches should return
// task state and delivery objects, not hand-build response maps.
func buildTaskEntryResponse(task runengine.TaskRecord, bubble map[string]any, deliveryResult map[string]any) map[string]any {
	response := map[string]any{
		"task":            taskMap(task),
		"bubble_message":  bubble,
		"delivery_result": nil,
	}
	if len(deliveryResult) > 0 {
		response["delivery_result"] = deliveryResult
	}
	return response
}
