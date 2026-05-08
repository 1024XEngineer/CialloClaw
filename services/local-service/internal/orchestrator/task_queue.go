package orchestrator

import (
	"fmt"
	"strings"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
)

func taskSessionValue(sessionID string) any {
	if strings.TrimSpace(sessionID) == "" {
		return nil
	}
	return strings.TrimSpace(sessionID)
}

func (s *Service) queueTaskIfSessionBusy(task runengine.TaskRecord) (runengine.TaskRecord, map[string]any, bool, error) {
	activeTask, ok := s.runEngine.ActiveSessionTask(task.SessionID, task.TaskID)
	if !ok {
		return runengine.TaskRecord{}, nil, false, nil
	}

	bubble := s.delivery.BuildBubbleMessage(
		task.TaskID,
		"status",
		fmt.Sprintf("当前会话已有任务 %s 正在执行，本任务已排队等待。", truncateText(activeTask.Title, 24)),
		task.UpdatedAt.Format(dateTimeLayout),
	)
	var queuedTask runengine.TaskRecord
	changed := false
	if s.isPreparedRestartAttempt(task) {
		queuedTask, changed = s.runEngine.QueuePreparedTaskForSession(task, activeTask.TaskID, bubble)
	} else {
		queuedTask, changed = s.runEngine.QueueTaskForSession(task.TaskID, activeTask.TaskID, bubble)
	}
	if !changed {
		return runengine.TaskRecord{}, nil, false, ErrTaskNotFound
	}
	return queuedTask, bubble, true, nil
}

func (s *Service) drainSessionQueue(sessionID string) error {
	for {
		nextTask, ok := s.runEngine.NextQueuedTaskForSession(sessionID)
		if !ok {
			return nil
		}
		if activeTask, busy := s.runEngine.ActiveSessionTask(sessionID, nextTask.TaskID); busy && activeTask.TaskID != "" {
			return nil
		}

		bubble := s.delivery.BuildBubbleMessage(
			nextTask.TaskID,
			"status",
			"前序任务已完成，当前会话中的下一个任务开始执行。",
			nextTask.UpdatedAt.Format(dateTimeLayout),
		)
		resumedTask, changed := s.runEngine.ResumeQueuedTask(nextTask.TaskID, s.activeExecutionStepName(nextTask.Intent), bubble)
		if !changed {
			return ErrTaskNotFound
		}
		resumedTask, handled, controlledErr := s.resumeQueuedControlledTask(resumedTask)
		if controlledErr != nil {
			return controlledErr
		}
		if handled {
			if taskIsTerminal(resumedTask.Status) {
				continue
			}
			return nil
		}

		governedTask, _, handled, governanceErr := s.handleTaskGovernanceDecision(resumedTask, resumedTask.Intent)
		if governanceErr != nil {
			return governanceErr
		}
		if handled {
			if taskIsTerminal(governedTask.Status) {
				continue
			}
			return nil
		}

		updatedTask, _, _, _, err := s.executeTask(governedTask, snapshotFromTask(governedTask), governedTask.Intent)
		if err != nil {
			return err
		}
		if !taskIsTerminal(updatedTask.Status) {
			return nil
		}
	}
}
