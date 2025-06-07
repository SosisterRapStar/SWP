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

var once sync.Once

type WorkerInfo struct {
	ID     uuid.UUID `json:"ID"`
	Active bool      `json:"Active"`
}

type Worker struct {
	isActive atomic.Bool

	ID uuid.UUID

	infoStream chan<- string
	stop       <-chan struct{}
	idChan     chan<- uuid.UUID
	tasks      <-chan Task
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
		w.sendMessage(fmt.Sprintf("Worker %v: started to listen for tasks\n", w.ID))
		for {
			select {
			case task := <-w.tasks:
				if w.infoStream != nil {
					w.sendMessage(fmt.Sprintf("Worker %v: started the job\n", w.ID))
				}
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
					w.sendMessage(fmt.Sprintf("Worker %v closed by worker pool closingn\n", w.ID))
					return
				}
				w.sendMessage(fmt.Sprintf("Worker %v closed by deleting workern\n", w.ID))
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
	if wp.infoWriter != nil {
		wp.infoStream = make(chan string, infoBufferChannelSize)
		wp.startInfoWriter()
	}
	for i := 0; i < wp.size; i++ {
		worker := &Worker{
			ID:         uuid.New(),
			tasks:      wp.tasksChan,
			infoStream: wp.infoStream,
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

	}
	if wp.infoStream != nil {
		wp.infoStream <- "Pool was closed"
		close(wp.infoStream)
	}
	close(wp.tasksChan)
	return nil
}

// Get idle worker from stoped queue
// func (wp *WorkerPool) reuseWorker(number int) []uuid.UUID {
// 	reusableId := make([]uuid.UUID, 0, number)
// 	for len(wp.stopedWorkers) > 0 && number > 0 {
// 		number--
// 		idleWorkerID := wp.stopedWorkers[len(wp.stopedWorkers)-1]
// 		wp.stopedWorkers = wp.stopedWorkers[:len(wp.stopedWorkers)-1]
// 		reusableId = append(reusableId, idleWorkerID)
// 	}
// 	return reusableId
// }

// Adds provided number of workers to pool
// TODO: refactor
func (wp *WorkerPool) AddWorkers(number int) {
	wp.mx.Lock()
	defer wp.mx.Unlock()
	reused := wp.stopedWorkers[:min(number, len(wp.stopedWorkers))]
	wp.stopedWorkers = wp.stopedWorkers[:len(reused)]
	number -= len(reused)
	for _, wrk := range reused {
		wp.innerPool[wrk].Start(wp.wg)
	}
	var worker *Worker
	for i := 0; i < number; i++ {
		worker = &Worker{
			ID:         uuid.New(),
			tasks:      wp.tasksChan,
			infoStream: wp.infoStream,
			idChan:     wp.idChan,
			stop:       wp.stopChan}
		wp.innerPool[worker.ID] = worker
		worker.Start(wp.wg)
	}
	wp.size += number
}

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
			wp.size--
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
			log.Println("task was sent")
			return errorChan, nil
		case <-ctx.Done():
			close(errorChan)
			return nil, errors.New("can not execute task because all workers are busy")
		default:
			log.Println("Wait")
			time.Sleep(5 * time.Second)
		}
	}

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
