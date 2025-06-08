package pool

type Task struct {
	errorChan chan<- error
	taskFunc  func() error
}
