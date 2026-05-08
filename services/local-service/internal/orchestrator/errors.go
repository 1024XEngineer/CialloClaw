package orchestrator

import (
	"errors"
	"sync/atomic"
)

// ErrTaskNotFound indicates that the provided task_id does not exist in the
// current runtime or hydrated query state.
var (
	ErrTaskNotFound           = errors.New("task not found")
	ErrArtifactNotFound       = errors.New("artifact not found")
	ErrTaskStatusInvalid      = errors.New("task status invalid")
	ErrTaskAlreadyFinished    = errors.New("task already finished")
	ErrStorageQueryFailed     = errors.New("storage query failed")
	ErrStrongholdAccessFailed = errors.New("stronghold access failed")
	ErrRecoveryPointNotFound  = errors.New("recovery point not found")
	persistedToolCallEventSeq atomic.Uint64
)
