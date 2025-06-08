package utils

import (
	"os"

	"github.com/SosisterRapStar/SWP/pool"
)

var (
	DefaultConfig = pool.WorkerPoolConfig{
		MaxIdleWorkers: 10,
		InitialSize:    10,
		WaitQueueSize:  10,
		InfoWriter:     os.Stderr,
	}

	NoIdleWorkers = pool.WorkerPoolConfig{
		MaxIdleWorkers: 0,
		InitialSize:    10,
		WaitQueueSize:  10,
		InfoWriter:     os.Stderr,
	}

	NoWaitQueueNoIdleWorkers = pool.WorkerPoolConfig{
		MaxIdleWorkers: 0,
		InitialSize:    3,
		WaitQueueSize:  0,
		InfoWriter:     os.Stderr,
	}

	OneWorker = pool.WorkerPoolConfig{
		MaxIdleWorkers: 0,
		InitialSize:    1,
		WaitQueueSize:  0,
		InfoWriter:     os.Stderr,
	}
	OneIdleWorker = pool.WorkerPoolConfig{
		MaxIdleWorkers: 1,
		InitialSize:    3,
		WaitQueueSize:  1, // будет ждать пока не реюзнется воркер
		InfoWriter:     os.Stderr,
	}

	TwoIdleWorker = pool.WorkerPoolConfig{
		MaxIdleWorkers: 2,
		InitialSize:    3,
		WaitQueueSize:  1, // будет ждать пока не реюзнется воркер
		InfoWriter:     os.Stderr,
	}

	ZeroWorkersOnlyQueue = pool.WorkerPoolConfig{
		MaxIdleWorkers: 0,
		InitialSize:    0,
		WaitQueueSize:  10,
		InfoWriter:     os.Stderr,
	}
)
