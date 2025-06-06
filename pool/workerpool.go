package pool

import (
	"context"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
)

const infoBufferChannelSize = 100
const defaultIdleWorkersNumber = 10
const defaultWaitQueueSize = 10

type WorkerPoolConfig struct {
	MaxIdleWorkers *int
	InitialSize    *int
	WaitQueueSize  *int
}

type controllUnit struct {
	worker   *Worker
	stopChan chan<- struct{}
}

type Executor interface {
	Start()
}

type Worker struct {
	isActive    atomic.Bool
	ID          uuid.UUID
	tasksStream <-chan Task
	infoStream  chan<- string
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

func (w *Worker) Start(ctx context.Context, stop chan struct{}) {
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

		case <-stop:
			w.infoStream <- fmt.Sprintf("Worker %v closed", w.ID)
			return
		case <-ctx.Done():
			w.infoStream <- fmt.Sprintf("Worker %v closed", w.ID)
			return
		}
	}
}

type WorkerPool struct {
	// canAcceptTasks atomic.Bool // заменить на sync.Once
	wg             *sync.WaitGroup
	mx             *sync.Mutex
	tasksChan      chan Task
	innerPool      map[uuid.UUID]*controllUnit
	WaitQueueSize  int // размер буфера канала
	infoStream     chan string
	ctx            context.Context
	stopCtx        context.CancelFunc
	stopedWorkers  []uuid.UUID
	MaxIdleWorkers int
}

func New(initSize int, queueSize int) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	pool := &WorkerPool{
		wg:             &sync.WaitGroup{},
		mx:             &sync.Mutex{},
		tasksChan:      make(chan Task, queueSize),
		innerPool:      make(map[uuid.UUID]*controllUnit, initSize),
		WaitQueueSize:  queueSize,
		infoStream:     make(chan string, infoBufferChannelSize),
		ctx:            ctx,
		stopCtx:        cancel,
		MaxIdleWorkers: defaultIdleWorkersNumber,
		stopedWorkers:  make([]uuid.UUID, 0, defaultIdleWorkersNumber),
	}
	// pool.canAcceptTasks.Store(true)
	return pool
}

func (wp *WorkerPool) startInfoWriter(w io.Writer) {
	for info := range wp.infoStream {
		w.Write([]byte(info))
	}
	wp.wg.Done()
}

func (wp *WorkerPool) Size() int {
	return len(wp.innerPool)
}

// func (wp *WorkerPool) checkAcceptAbility() bool {
// 	return wp.canAcceptTasks.Load()
// }

// func (wp *WorkerPool) setAcceptAbility(newState bool) {
// 	wp.canAcceptTasks.Store(newState)
// }

// Будем открывать пул, нужна переменная, которая будет обновляться через атомики,
// если переменная = 1 то значит, что пул можно открыть,
// если переменная = 0 то возвращаем ошибку, это значит, что по каким-то причинам пул нельзя открыть
// пул нельзя открыть, если он в процессе открытия, в процессе закрытия или уже открыт

func (wp *WorkerPool) Open() {
	// if !wp.checkAcceptAbility() {
	// 	return errors.New("can not open worker pool")
	// }
	// wp.setAcceptAbility(false)
	go wp.startInfoWriter()
	for i := 0; i < len(wp.innerPool); i++ {
		stopChannel := make(chan struct{})
		worker := &Worker{
			ID:          uuid.New(),
			tasksStream: wp.tasksChan,
			infoStream:  wp.infoStream,
		}
		wp.innerPool[worker.ID] =
			&controllUnit{
				worker:   worker,
				stopChan: stopChannel,
			}
		worker.setState(true)
		worker.Start(wp.ctx, stopChannel)
	}
	// wp.setAcceptAbility(true)
	// return nil
}

// будем использовать, чтобы закрыть все каналы и остановить все горутины
func (wp *WorkerPool) Close(ctx context.Context) error {
	wp.wg.Wait()
	return nil
}

func (wp *WorkerPool) AddWorker() {

	stopChan := make(chan struct{})
	newW := &Worker{
		ID:          uuid.New(),
		tasksStream: wp.tasksChan,
		infoStream:  wp.infoStream,
	}
	wp.mx.Lock()
	if len(wp.stopedWorkers) != 0 {
		idleWorkerID := wp.stopedWorkers[len(wp.stopedWorkers)-1]
		wp.stopedWorkers = wp.stopedWorkers[:len(wp.stopedWorkers)-1]
		newStopChannel := make(chan struct{})
		controlUnit := wp.innerPool[idleWorkerID]
		controlUnit.stopChan = newStopChannel
		wp.mx.Unlock()
		controlUnit.worker.Start(wp.ctx, newStopChannel)
		return
	}
	wp.innerPool[newW.ID] =
		&controllUnit{
			worker:   newW,
			stopChan: stopChan,
		}
	wp.mx.Unlock()
	newW.Start(wp.ctx, stopChan)
}

func (wp *WorkerPool) DeleteWorker() {

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
