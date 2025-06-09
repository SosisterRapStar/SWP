package pool

import "errors"

var (
	ErrorConfigValidation    = errors.New("invalid worker pool configuration")
	ErrorNegativeMaxIdle     = errors.New("max idle workers cannot be negative")
	ErrorNegativeInitialSize = errors.New("initial size cannot be negative")
	ErrorNegativeWaitQueue   = errors.New("wait queue size cannot be negative")
	ErrorAllWorkersAreBusy   = errors.New("all workers are busy")
	ErrorOnWorkerStop        = errors.New("failed to stop worker")
	ErrorPoolClosed          = errors.New("worker pool is already closed")
	ErrorOnClosing           = errors.New("timeout or cancellation during pool closing")
	ErrorValidationError     = errors.New("error during config validation")
	ErrorOnDeleteWorker      = errors.New("can not delete worker")
)
