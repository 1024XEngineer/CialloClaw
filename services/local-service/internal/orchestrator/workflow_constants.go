package orchestrator

import (
	"time"
)

const (
	executionSegmentInitial     = "initial"
	executionSegmentResume      = "resume"
	executionSegmentRestart     = "restart"
	defaultTaskExecutionTimeout = 95 * time.Second
	subjectPreviewMaxLength     = 24
	resultPreviewMaxLength      = 120
)
