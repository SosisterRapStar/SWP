package pool

import (
	"context"
	"fmt"
	"io"
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

// A Config is used for setting up parametres of worker pool
type Config struct {
	//MaxIdleWorkers represents max amount of inactive workers
	MaxIdleWorkers int
	//InitialSize size of worker pool it's initial because pool can be shrinked or extended dynamicly
	InitialSize int
	//WaitQueueSize determines the size of buffered channel inside pool,
	//you can use it to plan tasks which will be put in buffer waiting for workers
	WaitQueueSize int
	//InfoWriter log writer for workers
	InfoWriter io.Writer
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
		return fmt.Errorf("%w:%v", ErrorValidationError, "max idle workrs can not be negative")
	}
	if c.InitialSize <= 0 {
		return fmt.Errorf("%w:%v", ErrorValidationError, "size of the pool can not be less or equal zero")
	}
	if c.WaitQueueSize < 0 {
		return fmt.Errorf("%w:%v", ErrorValidationError, "queue size can not be negative")
	}

	if c.MaxIdleWorkers > c.InitialSize {
		return fmt.Errorf("%w:%v", ErrorValidationError, "max idle workers cannot exceed initial size")
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
		return ErrorPoolClosed
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
		// log.Println("pool closed ?")
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
func (wp *WorkerPool) AddWorkers(number int) error {
	if !wp.isOpened {
		return ErrorPoolClosed
	}

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
		// log.Println("Reused worker started")
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
	return nil
}

// Deletes random free worker
func (wp *WorkerPool) DeleteWorker(ctx context.Context) error {
	if !wp.isOpened {
		return ErrorPoolClosed
	}
	if wp.Size <= 1 {
		return fmt.Errorf("%w:%v", ErrorOnDeleteWorker, "there are too few workers left")
	}
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
	if !wp.isOpened {
		return nil, ErrorPoolClosed
	}
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
