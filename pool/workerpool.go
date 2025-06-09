package pool

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	InfoBufferChannelSize              = 20
	DefaultIdleWorkersNumber           = 10
	DefaultWaitQueueSize               = 10
	DefaultInitSize                    = 10
	DefaultAdditionalSpaceForInnerPool = 15
)

type Config struct {
	MaxIdleWorkers int
	InitialSize    int
	WaitQueueSize  int
	InfoWriter     io.Writer
}

func DefaultConfig() Config {
	return Config{
		MaxIdleWorkers: DefaultIdleWorkersNumber,
		InitialSize:    DefaultInitSize,
		WaitQueueSize:  DefaultWaitQueueSize,
		InfoWriter:     os.Stderr,
	}
}

func validate(c *Config) error {
	if c.MaxIdleWorkers < 0 {
		return ErrorNegativeMaxIdle
	}
	if c.InitialSize < 0 {
		return ErrorNegativeInitialSize
	}
	if c.WaitQueueSize < 0 {
		return ErrorNegativeWaitQueue
	}
	if c.InitialSize == 0 {
		return ErrorZeroInitialSize
	}
	if c.MaxIdleWorkers > c.InitialSize {
		return errors.New("max idle workers cannot exceed initial size")
	}
	if c.InfoWriter == nil {
		c.InfoWriter = os.Stderr
	}
	return nil
}

type WorkerPool struct {
	wg *sync.WaitGroup
	mx *sync.RWMutex

	innerPool     map[uuid.UUID]*Worker
	stopedWorkers []uuid.UUID

	WaitQueueSize  int // размер буфера канала
	Size           int
	MaxIdleWorkers int

	tasksChan chan Task
	idChan    chan uuid.UUID // this chan is used to send concrete worker id which was stoped by delete method
	stopChan  chan struct{}

	infoWriter io.Writer
	openOnce   sync.Once
	isOpened   bool
}

func New(c Config) (*WorkerPool, error) {

	if err := validate(&c); err != nil {
		return nil, err
	}

	pool := &WorkerPool{
		wg: &sync.WaitGroup{},
		mx: &sync.RWMutex{},

		tasksChan:     make(chan Task, c.WaitQueueSize),
		innerPool:     make(map[uuid.UUID]*Worker, c.InitialSize+DefaultAdditionalSpaceForInnerPool),
		idChan:        make(chan uuid.UUID),
		stopedWorkers: make([]uuid.UUID, 0, c.MaxIdleWorkers),
		stopChan:      make(chan struct{}),

		MaxIdleWorkers: c.MaxIdleWorkers,
		WaitQueueSize:  c.WaitQueueSize,
		infoWriter:     c.InfoWriter,
		Size:           c.InitialSize,
		openOnce:       sync.Once{},
		isOpened:       false,
	}
	return pool, nil
}

func (wp *WorkerPool) Open() {
	wp.openOnce.Do(func() {
		for i := 0; i < wp.Size; i++ {
			worker := &Worker{
				id:         uuid.New(),
				tasks:      wp.tasksChan,
				infoWriter: wp.infoWriter,
				idChan:     wp.idChan,
				stop:       wp.stopChan,
			}
			wp.innerPool[worker.id] = worker
			worker.isActive.Store(true)
			wp.wg.Add(1)
			worker.Start(wp.wg)
		}
		wp.isOpened = true
		// log.Println("Workerpool started")

	})

}

// func (wp *WorkerPool) log(s string) {
// 	if wp.infoWriter != nil {
// 		wp.infoWriter.Write([]byte(s))
// 	}
// }

// Close all goroutines which was added
func (wp *WorkerPool) Close(ctx context.Context) error {
	if !wp.isOpened {
		return ErrorAlreadyClosed
	}
	close(wp.stopChan)
	done := make(chan struct{})
	go func() {
		wp.wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return ErrorOnClosing
	case <-done:
		// fmt.Println("pool closed ?")W
	}
	close(wp.tasksChan)
	if wp.infoWriter != nil {
		// wp.log("Pool was closed")
	}
	wp.isOpened = false
	return nil
}

func (wp *WorkerPool) IsOpened() bool {
	return wp.isOpened
}

// Adds provided number of workers to pool
// TODO: refactor
func (wp *WorkerPool) AddWorkers(number int) {
	wp.mx.Lock()
	defer wp.mx.Unlock()
	// log.Printf("stoped workers %v", wp.stopedWorkers)
	reused := wp.stopedWorkers[:min(number, len(wp.stopedWorkers))]
	// log.Printf("workers added to reuse %v", reused)
	wp.stopedWorkers = wp.stopedWorkers[len(reused):]
	// log.Printf("stop workers after reuse %v", wp.stopedWorkers)
	number -= len(reused)
	for _, id := range reused {
		wp.Size++
		log.Println("Reused worker started")
		wp.wg.Add(1)
		wp.innerPool[id].Start(wp.wg)
	}
	var worker *Worker
	for i := 0; i < number; i++ {
		worker = &Worker{
			id:         uuid.New(),
			tasks:      wp.tasksChan,
			infoWriter: wp.infoWriter,
			idChan:     wp.idChan,
			stop:       wp.stopChan}
		wp.innerPool[worker.id] = worker
		// log.Println("New worker added")

		wp.wg.Add(1)
		worker.Start(wp.wg)
		wp.Size++
	}
}

// Deletes random free worker
func (wp *WorkerPool) DeleteWorker(ctx context.Context) error {
	for {
		select {
		case wp.stopChan <- struct{}{}:
			id := <-wp.idChan

			wp.mx.Lock()
			if len(wp.stopedWorkers) == wp.MaxIdleWorkers {
				delete(wp.innerPool, id)
			} else {
				wp.stopedWorkers = append(wp.stopedWorkers, id)
			}
			wp.Size--
			wp.mx.Unlock()

			return nil
		case <-ctx.Done():
			return ErrorOnWorkerStop
		}
	}
}

func (wp *WorkerPool) Execute(job func() error, ctx context.Context) (<-chan error, error) {
	// log.Println("execute was started")
	// if _, ok := <-wp.stopChan; !ok {
	// 	// log.Println("error")
	// 	return nil, errors.New("pool is closed")
	// }
	// log.Println("execute was started")
	errorChan := make(chan error)
	// log.Println("execute was started")
	for {
		select {
		case wp.tasksChan <- Task{taskFunc: job, errorChan: errorChan}:
			// log.Println("Task was planned")
			return errorChan, nil
		case <-ctx.Done():
			close(errorChan)
			return nil, ErrorAllWorkersAreBusy
		default:
			// log.Println("Wait queue is full, waiting for free workers")
			time.Sleep(1 * time.Second)
		}
	}

}

func (wp *WorkerPool) GetWorkersInfo() []WorkerInfo {
	info := make([]WorkerInfo, 0, wp.Size)
	wp.mx.RLock()
	for _, wk := range wp.innerPool {
		info = append(info, WorkerInfo{ID: wk.id, Active: wk.isActive.Load()})
	}
	wp.mx.RUnlock()
	return info
}
