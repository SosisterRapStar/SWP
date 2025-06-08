package pool

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

const (
	infoBufferChannelSize              = 20
	defaultIdleWorkersNumber           = 10
	defaultWaitQueueSize               = 10
	defaultInitSize                    = 10
	defaultAdditionalSpaceForInnerPool = 15
)

// var once sync.Once

type WorkerInfo struct {
	ID     uuid.UUID `json:"ID"`
	Active bool      `json:"Active"`
}

type Worker struct {
	isActive atomic.Bool

	ID uuid.UUID

	infoWriter io.Writer
	stop       <-chan struct{}
	idChan     chan<- uuid.UUID
	tasks      <-chan Task
	// wg          *sync.WaitGroup
}

type Task struct {
	errorChan chan<- error
	taskFunc  func() error
}

// Worker
func (w *Worker) Start(wg *sync.WaitGroup) {
	go func() {
		defer wg.Done()
		w.isActive.Store(true)
		w.sendMessage(fmt.Sprintf("Worker %v: started to listen for tasks\n", w.ID))
		for {
			select {
			case task := <-w.tasks:
				w.sendMessage(fmt.Sprintf("Worker %v: started the job\n", w.ID))
				if err := task.taskFunc(); err != nil {
					w.sendMessage(fmt.Sprintf("Worker %v: job was executed with error\n", w.ID))
					task.errorChan <- err
				} else {
					w.sendMessage(fmt.Sprintf("Worker %v: job done\n", w.ID))
				}
				close(task.errorChan)

			case _, ok := <-w.stop:
				w.isActive.Store(false)
				if !ok {
					w.sendMessage(fmt.Sprintf("Worker %v closed by worker pool\n", w.ID))
					return
				}
				w.sendMessage(fmt.Sprintf("Worker %v closed by deleting worker\n", w.ID))
				w.idChan <- w.ID
				return
			}
		}
	}()
}

func (w *Worker) sendMessage(message string) {
	if w.infoWriter != nil {
		w.infoWriter.Write([]byte(message))
	}
}

type WorkerPool struct {
	wg *sync.WaitGroup
	mx *sync.RWMutex

	innerPool     map[uuid.UUID]*Worker
	stopedWorkers []uuid.UUID

	WaitQueueSize  int // размер буфера канала
	Size           int
	MaxIdleWorkers int

	ctx       context.Context
	tasksChan chan Task
	idChan    chan uuid.UUID // this chan is used to send concrete worker id which was stoped by poo
	stopChan  chan struct{}

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

	// TODO: add checks
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
		Size:           c.InitialSize,

		ctx:     ctx,
		stopCtx: cancel,
	}
	return pool
}

func (wp *WorkerPool) Open() {
	for i := 0; i < wp.Size; i++ {
		worker := &Worker{
			ID:         uuid.New(),
			tasks:      wp.tasksChan,
			infoWriter: wp.infoWriter,
			idChan:     wp.idChan,
			stop:       wp.stopChan,
		}
		wp.innerPool[worker.ID] = worker
		worker.isActive.Store(true)
		wp.wg.Add(1)
		worker.Start(wp.wg)
	}
	log.Println("Workerpool started")

}

// Close all goroutines which was added
func (wp *WorkerPool) Close(ctx context.Context) error {
	close(wp.stopChan)
	done := make(chan struct{})
	go func() {
		wp.wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf("error occured during pool closing: %w", ctx.Err())
	case <-done:
		// fmt.Println("pool closed ?")W
	}
	close(wp.tasksChan)
	if wp.infoWriter != nil {
		wp.infoWriter.Write([]byte("Pool was closed"))
	}
	return nil
}

// Adds provided number of workers to pool
// TODO: refactor
func (wp *WorkerPool) AddWorkers(number int) {
	wp.mx.Lock()
	defer wp.mx.Unlock()
	log.Printf("stoped workers %v", wp.stopedWorkers)
	reused := wp.stopedWorkers[:min(number, len(wp.stopedWorkers))]
	log.Printf("workers added to reuse %v", reused)
	wp.stopedWorkers = wp.stopedWorkers[len(reused):]
	log.Printf("stop workers after reuse %v", wp.stopedWorkers)
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
			ID:         uuid.New(),
			tasks:      wp.tasksChan,
			infoWriter: wp.infoWriter,
			idChan:     wp.idChan,
			stop:       wp.stopChan}
		wp.innerPool[worker.ID] = worker
		log.Println("New worker added")

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
			return fmt.Errorf("error occured during worker stop: %w", ctx.Err())
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
			log.Println("Task was planned")
			return errorChan, nil
		case <-ctx.Done():
			close(errorChan)
			return nil, errors.New("can not execute task because all workers are busy")
		default:
			log.Println("Wait queue is full, waiting for free workers")
			time.Sleep(1 * time.Second)
		}
	}

}

// сделать другой метод, так как все равно выведет все воркеры, даже если они считаются остановленными
func (wp *WorkerPool) GetWorkersInfo() []WorkerInfo {
	info := make([]WorkerInfo, 0, wp.Size)
	wp.mx.RLock()
	for _, wk := range wp.innerPool {
		info = append(info, WorkerInfo{ID: wk.ID, Active: wk.isActive.Load()})
	}
	wp.mx.RUnlock()
	return info
}
