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

type controllUnit struct {
	worker   *Worker
	stopChan chan<- struct{}
}

type Executor interface {
	Start()
}

type Worker struct {
	isFree      atomic.Bool
	workerId    uuid.UUID
	tasksStream <-chan Task
	infoStream  chan<- string
}

type Task struct {
	errorChan chan<- error
	taskFunc  func() error
}

func (w *Worker) getState() bool {
	return w.isFree.Load()
}

func (w *Worker) setState(newState bool) {
	w.isFree.Store(newState)
}

func (w *Worker) Start(ctx context.Context, stop chan struct{}) {
	for {
		select {
		case task := <-w.tasksStream:
			w.infoStream <- fmt.Sprintf("Worker %v: started the job", w.workerId)
			if err := task.taskFunc(); err != nil {
				w.infoStream <- fmt.Sprintf("Worker %v: job was executed with error", w.workerId)
				task.errorChan <- err
			} else {
				w.infoStream <- fmt.Sprintf("Worker %v: job done", w.workerId)
			}
			close(task.errorChan)

		case <-stop:
			w.infoStream <- fmt.Sprintf("Worker %v closed", w.workerId)
			return
		case <-ctx.Done():
			w.infoStream <- fmt.Sprintf("Worker %v closed", w.workerId)
			return
		}
	}
}

type WorkerPool struct {
	// canAcceptTasks atomic.Bool // заменить на sync.Once
	sync.Mutex
	tasksChan     chan Task
	innerPool     []*controllUnit
	size          int // размер пула (кол-во воркеров)
	waitQueueSize int // размер буфера канала
	infoStream    chan string
	ctx           context.Context
	stopCtx       context.CancelFunc
}

func New(size int, queueSize int) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	pool := &WorkerPool{
		tasksChan:     make(chan Task, queueSize),
		innerPool:     make([]*controllUnit, size, size+10),
		size:          size,
		waitQueueSize: size,
		infoStream:    make(chan string, infoBufferChannelSize),
		ctx:           ctx,
		stopCtx:       cancel,
	}
	// pool.canAcceptTasks.Store(true)
	return pool
}

func (wp *WorkerPool) startInfoWriter(w io.Writer) {
	for info := range wp.infoStream {
		w.Write([]byte(info))
	}
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

	for i := 0; i < wp.size; i++ {
		stopChannel := make(chan struct{})
		worker := &Worker{
			workerId:    uuid.New(),
			tasksStream: wp.tasksChan,
			infoStream:  wp.infoStream,
		}
		wp.innerPool[i] = &controllUnit{
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
	return nil
}

func (wp *WorkerPool) startTask() {

}

func (wp *WorkerPool) AddWorker() {

	stopChan := make(chan struct{})
	newW := &Worker{
		workerId:    uuid.New(),
		tasksStream: wp.tasksChan,
		infoStream:  wp.infoStream,
	}
	wp.Lock()
	wp.innerPool = append(wp.innerPool, &controllUnit{
		worker:   newW,
		stopChan: stopChan,
	})
	wp.size++
	wp.Unlock()
	newW.Start(wp.ctx, stopChan)
}

func (wp *WorkerPool) DeleteWorker() {

}

func (wp *WorkerPool) Execute(ctx context.Context, task func() error) (<-chan error, error) {

}
