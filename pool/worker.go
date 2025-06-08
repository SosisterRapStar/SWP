package pool

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
)

type WorkerInfo struct {
	ID     uuid.UUID `json:"ID"`
	Active bool      `json:"Active"`
}

type Worker struct {
	isActive atomic.Bool

	id uuid.UUID

	infoWriter io.Writer
	stop       <-chan struct{}
	idChan     chan<- uuid.UUID
	tasks      <-chan Task
}

// Worker
func (w *Worker) Start(wg *sync.WaitGroup) {
	go func() {
		defer wg.Done()
		w.isActive.Store(true)
		w.sendMessage(fmt.Sprintf("Worker %v: started to listen for tasks\n", w.id))
		for {
			select {
			case task := <-w.tasks:
				w.sendMessage(fmt.Sprintf("Worker %v: started the job\n", w.id))
				if err := task.taskFunc(); err != nil {
					w.sendMessage(fmt.Sprintf("Worker %v: job was executed with error\n", w.id))
					task.errorChan <- err
				} else {
					w.sendMessage(fmt.Sprintf("Worker %v: job done\n", w.id))
				}
				close(task.errorChan)

			case _, ok := <-w.stop:
				w.isActive.Store(false)
				if !ok {
					w.sendMessage(fmt.Sprintf("Worker %v closed by worker pool\n", w.id))
					return
				}
				w.sendMessage(fmt.Sprintf("Worker %v closed by deleting worker\n", w.id))
				w.idChan <- w.id
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
