package pool

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
)

const (
	infoBufferChannelSize              = 20
	defaultIdleWorkersNumber           = 10
	defaultWaitQueueSize               = 10
	defaultInitSize                    = 10
	defaultAdditionalSpaceForInnerPool = 15
)

var once sync.Once

type WorkerInfo struct {
	ID     uuid.UUID `json:"ID"`
	Active bool      `json:"Active"`
}

type Worker struct {
	isActive atomic.Bool

	ID uuid.UUID

	infoStream  chan<- string
	stop        <-chan struct{}
	idChan      chan<- uuid.UUID
	tasksStream <-chan Task
	// wg          *sync.WaitGroup
}

type Task struct {
	errorChan chan<- error
	taskFunc  func() error
}

// func (w *Worker) getState() bool {
// 	return w.isActive.Load()
// }

// func (w *Worker) setState(newState bool) {
// 	w.isActive.Store(newState)
// }

// Worker
func (w *Worker) Start(wg *sync.WaitGroup) {
	go func() {
		defer wg.Done()
		w.isActive.Store(true)
		for {
			select {
			case task := <-w.tasksStream:
				if w.infoStream != nil {
					w.sendMessage(fmt.Sprintf("Worker %v: started the job", w.ID))
				}
				if err := task.taskFunc(); err != nil {
					w.sendMessage(fmt.Sprintf("Worker %v: job was executed with error", w.ID))
					task.errorChan <- err
				} else {
					w.sendMessage(fmt.Sprintf("Worker %v: job done", w.ID))
				}
				close(task.errorChan)

			case _, ok := <-w.stop:
				w.isActive.Store(false)
				if !ok {
					w.sendMessage(fmt.Sprintf("Worker %v closed by worker pool closing", w.ID))
					return
				}
				w.sendMessage(fmt.Sprintf("Worker %v closed by deleting worker", w.ID))
				w.idChan <- w.ID
				return
			}
		}
	}()
}

func (w *Worker) sendMessage(message string) {
	if w.infoStream != nil {
		w.infoStream <- message
	}
}

type WorkerPool struct {
	wg *sync.WaitGroup
	mx *sync.RWMutex

	innerPool     map[uuid.UUID]*Worker
	stopedWorkers []uuid.UUID

	WaitQueueSize  int // размер буфера канала
	size           int
	MaxIdleWorkers int

	ctx        context.Context
	tasksChan  chan Task
	infoStream chan string
	idChan     chan uuid.UUID // this chan is used to send concrete worker id which was stoped by poo
	stopChan   chan struct{}

	infoWriter io.Writer
	stopCtx    context.CancelFunc
}

type WorkerPoolConfig struct {
	MaxIdleWorkers int
	InitialSize    int
	WaitQueueSize  int

	InfoWriter io.Writer
}

func New(c WorkerPoolConfig) *WorkerPool {
	// if c.MaxIdleWorkers == nil {
	// 	c.MaxIdleWorkers = defaultIdleWorkersNumber
	// }
	// if c.InitialSize == nil {
	// 	c.InitialSize = defaultInitSize
	// }
	// if c.WaitQueueSize == nil {
	// 	c.WaitQueueSize = defaultWaitQueueSize
	// }

	ctx, cancel := context.WithCancel(context.Background())
	pool := &WorkerPool{
		wg: &sync.WaitGroup{},
		mx: &sync.RWMutex{},

		tasksChan:     make(chan Task, c.WaitQueueSize),
		innerPool:     make(map[uuid.UUID]*Worker, c.InitialSize+defaultAdditionalSpaceForInnerPool),
		idChan:        make(chan uuid.UUID),
		stopedWorkers: make([]uuid.UUID, 0, c.MaxIdleWorkers),
		stopChan:      make(chan struct{}),

		MaxIdleWorkers: c.MaxIdleWorkers,
		WaitQueueSize:  c.WaitQueueSize,
		infoWriter:     c.InfoWriter,
		size:           c.InitialSize,

		ctx:     ctx,
		stopCtx: cancel,
	}
	return pool
}

// Starts background goroutine which can write info to any stream
func (wp *WorkerPool) startInfoWriter() {
	wp.wg.Add(1)
	go func() {
		defer wp.wg.Done()
		for info := range wp.infoStream {
			_, err := wp.infoWriter.Write([]byte(info))
			if err != nil {
				log.Fatal("пусть пока так будет")
			}
		}
	}()
}

func (wp *WorkerPool) Size() int {
	return len(wp.innerPool) - len(wp.stopedWorkers)
}

func (wp *WorkerPool) Open() {
	once.Do(func() {
		if wp.infoWriter != nil {
			wp.infoStream = make(chan string, infoBufferChannelSize)
			wp.startInfoWriter()
		}
		for i := 0; i < wp.size; i++ {
			worker := &Worker{
				ID:          uuid.New(),
				tasksStream: wp.tasksChan,
				infoStream:  wp.infoStream,
				idChan:      wp.idChan,
				stop:        wp.stopChan,
			}
			wp.innerPool[worker.ID] = worker
			worker.isActive.Store(true)
			wp.wg.Add(1)
			worker.Start(wp.wg)
		}
	})

}

// Close all goroutines which was added
func (wp *WorkerPool) Close(ctx context.Context) error {
	close(wp.stopChan)
	wp.wg.Wait()
	return nil
}

// Get idle worker from stoped queue
func (wp *WorkerPool) reuseWorker(number int) []uuid.UUID {
	reusableId := make([]uuid.UUID, 0, number)
	for len(wp.stopedWorkers) > 0 && number > 0 {
		number--
		idleWorkerID := wp.stopedWorkers[len(wp.stopedWorkers)-1]
		wp.stopedWorkers = wp.stopedWorkers[:len(wp.stopedWorkers)-1]
		reusableId = append(reusableId, idleWorkerID)
	}
	return reusableId
}

// Adds provided number of workers to pool
func (wp *WorkerPool) AddWorkers(number int) {
	newWorkers := make([]*Worker, 0, number)
	wp.mx.Lock()
	reusedId := wp.reuseWorker(number)
	number -= len(reusedId)
	for i := 0; i < number; i++ {
		newWorkers = append(newWorkers, &Worker{
			ID:          uuid.New(),
			tasksStream: wp.tasksChan,
			infoStream:  wp.infoStream,
			idChan:      wp.idChan,
			stop:        wp.stopChan})
	}
	for _, id := range reusedId {
		wp.innerPool[id].Start(wp.wg)
	}
	for _, wk := range newWorkers {
		wp.innerPool[wk.ID] = wk
		wk.Start(wp.wg)
	}
	wp.mx.Unlock()
}

func (wp *WorkerPool) DeleteWorker() {
	wp.wg.Add(1)
	go func() {
		defer wp.wg.Done()
		wp.stopChan <- struct{}{}
	}()
	stopedWorkerId := <-wp.idChan
	if len(wp.stopedWorkers) == int(wp.MaxIdleWorkers) {
		wp.mx.Lock()
		delete(wp.innerPool, stopedWorkerId)
		wp.mx.Unlock()
		return
	}
	wp.stopedWorkers = append(wp.stopedWorkers, stopedWorkerId)
}

func (wp *WorkerPool) Execute(job func() error) (<-chan error, error) {
	errorChan := make(chan error)
	task := &Task{
		taskFunc:  job,
		errorChan: errorChan,
	}
	wp.tasksChan <- *task
	return errorChan, nil
}

func (wp *WorkerPool) GetWorkersInfo() []WorkerInfo {
	info := make([]WorkerInfo, 0, wp.Size())
	wp.mx.RLock()
	for _, wk := range wp.innerPool {
		info = append(info, WorkerInfo{ID: wk.ID, Active: wk.isActive.Load()})
	}
	wp.mx.Unlock()
	return info
}
