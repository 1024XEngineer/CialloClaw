package orchestrator

import (
	"time"
)

const (
	executionSegmentInitial     = "initial"
	executionSegmentResume      = "resume"
	executionSegmentRestart     = "restart"
	defaultTaskExecutionTimeout = 5 * time.Minute
	subjectPreviewMaxLength     = 24
	resultPreviewMaxLength      = 120
)
