package orchestrator

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/presentation"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
)

// TaskSteer handles agent.task.steer by persisting one follow-up instruction for
// a still-active task so later execution or resume paths can consume it.
func (s *Service) TaskSteer(params map[string]any) (map[string]any, error) {
	taskID := stringValue(params, "task_id", "")
	message := stringValue(params, "message", "")
	if strings.TrimSpace(taskID) == "" {
		return nil, errors.New("task_id is required")
	}
	if strings.TrimSpace(message) == "" {
		return nil, errors.New("message is required")
	}
	task, ok := s.runEngine.GetTask(taskID)
	if !ok {
		return nil, ErrTaskNotFound
	}
	if !taskCanAcceptExplicitSteering(task) {
		return nil, ErrTaskStatusInvalid
	}
	bubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", presentation.Text(presentation.MessageBubbleSteeringRecorded, nil), time.Now().Format(dateTimeLayout))
	updatedTask, changed := s.runEngine.AppendSteeringMessage(task.TaskID, message, bubble)
	if !changed {
		return nil, ErrTaskStatusInvalid
	}
	return map[string]any{
		"task":           taskMap(updatedTask),
		"bubble_message": bubble,
	}, nil
}

func taskCanAcceptExplicitSteering(task runengine.TaskRecord) bool {
	switch task.Status {
	case "processing":
		// Active processing tasks can only consume steering when the running
		// agent loop polls between rounds. Other processing paths must finish or
		// queue a separate task instead of pretending the guidance was consumed.
		return taskCanConsumeActiveSteering(task)
	case "waiting_auth", "blocked":
		// Deferred execution paths can carry explicit steering until approval or
		// queue release resumes the task. Pending-input states are intentionally
		// rejected so callers re-enter agent.input.submit and merge the text into
		// the formal continuation snapshot instead of hiding it in runtime notes.
		return true
	default:
		return false
	}
}

// TaskControl handles agent.task.control and converts user actions into runtime
// state-machine transitions. The orchestration layer owns error translation and
// post-transition follow-up such as human-loop resume handling and queue drain,
// because those behaviors depend on task-centric semantics rather than the raw
// runtime mutation alone.
func (s *Service) TaskControl(params map[string]any) (map[string]any, error) {
	taskID := stringValue(params, "task_id", "")
	if strings.TrimSpace(taskID) == "" {
		return nil, errors.New("task_id is required")
	}
	action := stringValue(params, "action", "")
	if strings.TrimSpace(action) == "" {
		return nil, errors.New("action is required")
	}
	if !isSupportedTaskControlAction(action) {
		return nil, fmt.Errorf("unsupported task control action: %s", action)
	}
	previousTask := runengine.TaskRecord{}
	if existingTask, ok := s.runEngine.GetTask(taskID); ok {
		previousTask = existingTask
	}
	wasHumanLoop := false
	var reviewDecision map[string]any
	arguments := mapValue(params, "arguments")
	if action == "resume" {
		wasHumanLoop = taskIsBlockedHumanLoop(previousTask)
		if wasHumanLoop {
			decision, decisionErr := humanReviewDecisionFromParams(arguments)
			if decisionErr != nil {
				return nil, decisionErr
			}
			reviewDecision = decision
		}
	}
	bubble := s.delivery.BuildBubbleMessage(taskID, "status", controlBubbleText(action), currentTimeFromTask(s.runEngine, taskID))
	updatedTask := runengine.TaskRecord{}
	if action == "restart" {
		preRestartTask, preparedRestartTask, restartErr := s.runEngine.PrepareRestart(taskID, bubble)
		if restartErr != nil {
			switch {
			case errors.Is(restartErr, runengine.ErrTaskNotFound):
				return nil, ErrTaskNotFound
			case errors.Is(restartErr, runengine.ErrTaskStatusInvalid):
				return nil, ErrTaskStatusInvalid
			case errors.Is(restartErr, runengine.ErrTaskAlreadyFinished):
				return nil, ErrTaskAlreadyFinished
			default:
				return nil, restartErr
			}
		}
		previousTask = preRestartTask
		updatedTask = preparedRestartTask
	} else {
		nextTask, err := s.runEngine.ControlTask(taskID, action, bubble)
		if err != nil {
			switch {
			case errors.Is(err, runengine.ErrTaskNotFound):
				return nil, ErrTaskNotFound
			case errors.Is(err, runengine.ErrTaskStatusInvalid):
				return nil, ErrTaskStatusInvalid
			case errors.Is(err, runengine.ErrTaskAlreadyFinished):
				return nil, ErrTaskAlreadyFinished
			default:
				return nil, err
			}
		}
		updatedTask = nextTask
	}
	if action == "resume" && wasHumanLoop {
		if traceResumedTask, traceBubble, _, resumed, resumeErr := s.resumeHumanLoopTask(updatedTask, reviewDecision); resumeErr != nil {
			return nil, resumeErr
		} else if resumed {
			updatedTask = traceResumedTask
			bubble = traceBubble
		}
	}
	if action == "restart" {
		restartedTask, restartBubble, restartErr := s.advanceRestartedTaskAttempt(previousTask, updatedTask)
		if restartErr != nil {
			return nil, restartErr
		}
		updatedTask = restartedTask
		if restartBubble != nil {
			bubble = restartBubble
		}
	}
	if taskIsTerminal(updatedTask.Status) {
		if queueErr := s.drainSessionQueue(updatedTask.SessionID); queueErr != nil {
			return nil, queueErr
		}
	}

	return map[string]any{
		"task":           taskMap(updatedTask),
		"bubble_message": bubble,
	}, nil
}

// advanceRestartedTaskAttempt sends a fresh restart run through the same
// pre-execution gates as a new task. Restart may allocate a new run_id, but it
// must not bypass session serialization or the authorization boundary before
// the executor receives that run.
func (s *Service) advanceRestartedTaskAttempt(previousTask, task runengine.TaskRecord) (runengine.TaskRecord, map[string]any, error) {
	if queuedTask, queueBubble, queued, queueErr := s.queueTaskIfSessionBusy(task); queueErr != nil {
		return runengine.TaskRecord{}, nil, queueErr
	} else if queued {
		return queuedTask, queueBubble, nil
	}

	governedTask, governedResponse, handled, governanceErr := s.handleTaskGovernanceDecision(task, task.Intent)
	if governanceErr != nil {
		return runengine.TaskRecord{}, nil, governanceErr
	}
	if handled {
		bubble := mapValue(governedResponse, "bubble_message")
		if len(bubble) == 0 {
			bubble = governedTask.BubbleMessage
		}
		return governedTask, bubble, nil
	}
	s.refreshTitleAfterGovernance(governedTask, snapshotFromTask(governedTask), governedTask.Intent)

	restartedTask, restartBubble, _, _, restartErr := s.executeTaskAttempt(previousTask, governedTask, snapshotFromTask(governedTask), governedTask.Intent)
	if restartErr != nil {
		return runengine.TaskRecord{}, nil, restartErr
	}
	return restartedTask, restartBubble, nil
}

// controlBubbleText returns the status bubble text for a task_control action.
func controlBubbleText(action string) string {
	switch action {
	case "pause":
		return presentation.Text(presentation.MessageBubbleTaskPaused, nil)
	case "resume":
		return presentation.Text(presentation.MessageBubbleTaskResumed, nil)
	case "cancel":
		return presentation.Text(presentation.MessageBubbleTaskCancelled, nil)
	case "restart":
		return presentation.Text(presentation.MessageBubbleTaskRestarted, nil)
	default:
		return presentation.Text(presentation.MessageBubbleTaskUpdated, nil)
	}
}

func isSupportedTaskControlAction(action string) bool {
	switch action {
	case "pause", "resume", "cancel", "restart":
		return true
	default:
		return false
	}
}
