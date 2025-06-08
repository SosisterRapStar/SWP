package pool

import "errors"

var (
	ErrorConfigValidation    = errors.New("invalid worker pool configuration")
	ErrorNegativeMaxIdle     = errors.New("max idle workers cannot be negative")
	ErrorNegativeInitialSize = errors.New("initial size cannot be negative")
	ErrorNegativeWaitQueue   = errors.New("wait queue size cannot be negative")
	ErrorZeroInitialSize     = errors.New("initial size cannot be zero when starting pool")
	ErrorAllWorkersAreBusy   = errors.New("all workers are busy")
	ErrorOnWorkerStop        = errors.New("failed to stop worker")
	ErrorAlreadyClosed       = errors.New("worker pool is already closed")
	ErrorOnClosing           = errors.New("timeout or cancellation during pool closing")
)
