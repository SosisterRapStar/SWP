package pool

import (
	"context"
	"sync/atomic"

	"github.com/google/uuid"
)

type Executor interface {
	Start()
}

type Worker struct {
	workerId    uuid.UUID
	tasksStream chan<- WorkerTask
	stopChan    chan<- struct{}
	errorChan   <-chan error
}

type WorkerTask struct {
	errorChan chan<- error
	taskFunc  func() error
}

// func (w *Worker) Start() {
// 	for {
// 		select {
// 		case task :=<-w.tasksStream:
// 			if err := task.
// 		}
// 	}
// }

type WorkerPool struct {
	canAcceptTasks atomic.Bool
	bareFuncStream chan func() error
	size           int // размер пула (кол-во воркеров)
	waitQueueSize  int // размер буфера канала
}

func (wp *WorkerPool) checkAcceptAbility() bool {
	return wp.canAcceptTasks.Load()
}

func (wp *WorkerPool) setAcceptAbility(newState bool) {
	wp.canAcceptTasks.Store(newState)
}

// Будем открывать пул, нужна переменная, которая будет обновляться через атомики,
// если переменная = 1 то значит, что пул можно открыть,
// если переменная = 0 то возвращаем ошибку, это значит, что по каким-то причинам пул нельзя открыть
// пул нельзя открыть, если он в процессе открытия, в процессе закрытия или уже открыт

func (wp *WorkerPool) Open() error {
	wp.setAcceptAbility(false)
	workers := make([]Worker, wp.Size)
	return nil
}

// будем использовать, чтобы закрыть все каналы и остановить все горутины
func (wp *WorkerPool) Close(ctx context.Context) error {
	return nil
}

func (wp *WorkerPool) startTask() {

}

func (wp *WorkerPool) Execute(ctx context.Context, task func() error) error {

}

func New() {

}
