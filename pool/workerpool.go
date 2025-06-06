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

const infoBufferChannelSize = 20

var (
	defaultIdleWorkersNumber uint8  = 10
	defaultWaitQueueSize     uint8  = 10
	defaultInitSize          uint32 = 10
)

type Worker struct {
	isActive    atomic.Bool
	ID          uuid.UUID
	tasksStream <-chan Task
	infoStream  chan<- string
	stop        <-chan struct{}
	idChan      chan<- uuid.UUID
	wg          *sync.WaitGroup
}

type Task struct {
	errorChan chan<- error
	taskFunc  func() error
}

func (w *Worker) getState() bool {
	return w.isActive.Load()
}

func (w *Worker) setState(newState bool) {
	w.isActive.Store(newState)
}

func (w *Worker) Start() {
	defer w.wg.Done()
	for {
		select {
		case task := <-w.tasksStream:
			w.infoStream <- fmt.Sprintf("Worker %v: started the job", w.ID)
			if err := task.taskFunc(); err != nil {
				w.infoStream <- fmt.Sprintf("Worker %v: job was executed with error", w.ID)
				task.errorChan <- err
			} else {
				w.infoStream <- fmt.Sprintf("Worker %v: job done", w.ID)
			}
			close(task.errorChan)

		case _, ok := <-w.stop:
			if !ok {
				w.infoStream <- fmt.Sprintf("Worker %v closed by worker pool closing", w.ID)
				return
			}
			w.infoStream <- fmt.Sprintf("Worker %v closed by deleting worker", w.ID)
			w.idChan <- w.ID
			return
		}
	}
}

type WorkerPool struct {
	// canAcceptTasks atomic.Bool // заменить на sync.Once
	wg             *sync.WaitGroup
	mx             *sync.Mutex
	tasksChan      chan Task
	innerPool      map[uuid.UUID]*Worker
	WaitQueueSize  uint8 // размер буфера канала
	infoStream     chan string
	ctx            context.Context
	stopCtx        context.CancelFunc
	stopedWorkers  []uuid.UUID
	MaxIdleWorkers uint8
	infoWriter     io.Writer
	idChan         chan uuid.UUID
	stopChan       chan struct{}
}

type WorkerPoolConfig struct {
	MaxIdleWorkers *uint8
	InitialSize    *uint32
	WaitQueueSize  *uint8
	InfoWriter     io.Writer
}

func New(c WorkerPoolConfig) *WorkerPool {
	if c.MaxIdleWorkers == nil {
		c.MaxIdleWorkers = &defaultIdleWorkersNumber
	}
	if c.InitialSize == nil {
		c.InitialSize = &defaultInitSize
	}
	if c.WaitQueueSize == nil {
		c.WaitQueueSize = &defaultWaitQueueSize
	}

	ctx, cancel := context.WithCancel(context.Background())
	pool := &WorkerPool{
		wg:             &sync.WaitGroup{},
		mx:             &sync.Mutex{},
		tasksChan:      make(chan Task, *c.WaitQueueSize),
		innerPool:      make(map[uuid.UUID]*Worker, *c.InitialSize),
		WaitQueueSize:  *c.WaitQueueSize,
		ctx:            ctx,
		stopCtx:        cancel,
		MaxIdleWorkers: *c.MaxIdleWorkers,
		stopedWorkers:  make([]uuid.UUID, 0, *c.MaxIdleWorkers),
		infoWriter:     c.InfoWriter,
		stopChan:       make(chan struct{}),
	}
	// pool.canAcceptTasks.Store(true)
	return pool
}

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
	return len(wp.innerPool)
}

func (wp *WorkerPool) Open() {
	if wp.infoWriter != nil {
		wp.infoStream = make(chan string, infoBufferChannelSize)
		wp.startInfoWriter()
	}
	for i := 0; i < len(wp.innerPool); i++ {
		worker := &Worker{
			ID:          uuid.New(),
			tasksStream: wp.tasksChan,
			infoStream:  wp.infoStream,
			idChan:      wp.idChan,
			stop:        wp.stopChan,
		}
		wp.innerPool[worker.ID] = worker
		worker.setState(true)
		wp.wg.Add(1)
		worker.Start()
	}

}

// будем использовать, чтобы закрыть все каналы и остановить все горутины
func (wp *WorkerPool) Close(ctx context.Context) error {
	close(wp.stopChan)
	wp.wg.Wait()
	return nil
}

func (wp *WorkerPool) AddWorker() {

	newW := &Worker{
		ID:          uuid.New(),
		tasksStream: wp.tasksChan,
		infoStream:  wp.infoStream,
		idChan:      wp.idChan,
		stop:        wp.stopChan,
	}
	wp.mx.Lock()
	if len(wp.stopedWorkers) != 0 {
		idleWorkerID := wp.stopedWorkers[len(wp.stopedWorkers)-1]
		wp.stopedWorkers = wp.stopedWorkers[:len(wp.stopedWorkers)-1]
		idleWorker := wp.innerPool[idleWorkerID]
		wp.mx.Unlock()
		idleWorker.Start()
		return
	}
	wp.innerPool[newW.ID] = newW
	wp.mx.Unlock()
	newW.Start()
}

func (wp *WorkerPool) DeleteWorker() {
	go func() {
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
	wp.innerPool[stopedWorkerId].setState(false)
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
