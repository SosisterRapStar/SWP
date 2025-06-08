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

	NoWaitQueue = pool.WorkerPoolConfig{
		MaxIdleWorkers: 0,
		InitialSize:    10,
		WaitQueueSize:  0,
		InfoWriter:     os.Stderr,
	}

	OneWorker = pool.WorkerPoolConfig{
		MaxIdleWorkers: 0,
		InitialSize:    1,
		WaitQueueSize:  0,
		InfoWriter:     os.Stderr,
	}

	ZeroConfig = pool.WorkerPoolConfig{
		MaxIdleWorkers: 0,
		InitialSize:    1,
		WaitQueueSize:  0,
		InfoWriter:     os.Stderr,
	}
)
