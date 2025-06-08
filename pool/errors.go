package pool

import "errors"

var (
	ErrorAlreadyClosed     = errors.New("pool already closed")
	ErrorOnClosing         = errors.New("error occured closing pool")
	ErrorAllWorkersAreBusy = errors.New("can not execute task because all workers are busy")
	ErrorOnWorkerStop      = errors.New("error occured during worker stop")
	ErrorConfigValidation  = errors.New("config validation error")
)
