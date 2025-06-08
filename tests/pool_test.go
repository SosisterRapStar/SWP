package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/SosisterRapStar/SWP/pool"
	"github.com/SosisterRapStar/SWP/tests/utils"
	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
)

func TestDefault(t *testing.T) {
	defer goleak.VerifyNone(t)

	pool := pool.New(utils.DefaultConfig)
	pool.Open()
	ctx, closeTimeOut := context.WithTimeout(context.Background(), 10*time.Second)
	defer closeTimeOut()
	if _, err := pool.Execute(utils.Ticker, ctx); err != nil {
		fmt.Println(err)
	}
	if _, err := pool.Execute(utils.Closure(4, 1), ctx); err != nil {
		fmt.Println(err)
	}

	// time.Sleep(10 * time.Second)
	pool.Close(ctx)
	fmt.Print("ds")
}

func TestAddingToPoolAndEmptyQueue(t *testing.T) {
	defer goleak.VerifyNone(t)

	pool := pool.New(utils.OneWorker)
	pool.Open()
	assert.Equal(t, 1, pool.Size)
	ctx, closeTimeOut := context.WithTimeout(context.Background(), 10*time.Second)
	defer closeTimeOut()

	if _, err := pool.Execute(utils.Closure(-1, 1), ctx); err != nil {
		fmt.Println(err)
	}

	pool.AddWorkers(1)
	assert.Equal(t, 2, pool.Size)
	if _, err := pool.Execute(utils.Closure(4, 2), ctx); err != nil {
		fmt.Println(err)
	}
	workersInfo := pool.GetWorkersInfo()
	assert.Equal(t, 2, len(workersInfo))
	for _, wki := range workersInfo {
		assert.Equal(t, wki.Active, true)
	}

	// time.Sleep(10 * time.Second)
	pool.Close(ctx)

	workersInfo = pool.GetWorkersInfo()
	assert.Equal(t, 2, len(workersInfo))
	for _, wki := range workersInfo {
		assert.Equal(t, wki.Active, false)
	}
}

func TestQueue(t *testing.T) {
	defer goleak.VerifyNone(t)

	pool := pool.New(utils.ZeroWorkersOnlyQueue)
	assert.Equal(t, 0, pool.Size)
	pool.Open()

	tasks := make([]func() error, 5)

	tasks[0] = utils.Closure(0, 1)
	tasks[1] = utils.Closure(0, 2)
	tasks[2] = utils.TickerWithVarStopTime(10, 3)
	tasks[3] = utils.Closure(0, 4)
	tasks[4] = utils.TickerWithVarStopTime(5, 5)

	ctx, closeTimeOut := context.WithTimeout(context.Background(), 10*time.Second)
	defer closeTimeOut()

	for _, t := range tasks {
		if _, err := pool.Execute(t, ctx); err != nil {
			fmt.Println(err)
		}
	}

	pool.Close(ctx)

}

func TestWithoutWaitQueue(t *testing.T) {
	defer goleak.VerifyNone(t)

	pool := pool.New(utils.NoWaitQueueNoIdleWorkers)
	assert.Equal(t, 3, pool.Size)
	pool.Open()

	tasks := make([]func() error, 5)

	tasks[0] = utils.Closure(0, 1)
	tasks[1] = utils.Closure(0, 2)
	tasks[2] = utils.TickerWithVarStopTime(10, 3)
	tasks[3] = utils.Closure(0, 4)
	tasks[4] = utils.TickerWithVarStopTime(5, 5)

	ctx, closeTimeOut := context.WithTimeout(context.Background(), 10*time.Second)
	defer closeTimeOut()

	for _, t := range tasks {
		if _, err := pool.Execute(t, ctx); err != nil {
			fmt.Println(err)
		}
	}

	closeCtx, closeCloseCtx := context.WithTimeout(context.Background(), 10*time.Second)
	defer closeCloseCtx()
	pool.Close(closeCtx)
}

// TODO: change test architecture for configs as test can not know about how many workers in config
func TestDeleteWorker(t *testing.T) {
	pool := pool.New(utils.NoWaitQueueNoIdleWorkers)
	defer goleak.VerifyNone(t)
	defer func() {
		closeCtx, closeCloseCtx := context.WithTimeout(context.Background(), 10*time.Second)
		defer closeCloseCtx()
		pool.Close(closeCtx)
	}()
	assert.Equal(t, 3, pool.Size)
	pool.Open()
	tasks := make([]func() error, 3, 5)
	tasks[0] = utils.Closure(0, 1)
	tasks[1] = utils.Closure(0, 2)
	tasks[2] = utils.TickerWithVarStopTime(10, 3)
	// tasks[3] = utils.Closure(0, 4)
	// tasks[4] = utils.TickerWithVarStopTime(5, 5)

	ctx, closeTimeOut := context.WithTimeout(context.Background(), 10*time.Second)
	defer closeTimeOut()
	for _, t := range tasks {
		if _, err := pool.Execute(t, ctx); err != nil {
			fmt.Println(err)
		}
	}
	pool.DeleteWorker(context.Background())

	assert.Equal(t, 2, pool.Size)
	assert.Equal(t, 2, len(pool.GetWorkersInfo()))
	for _, wki := range pool.GetWorkersInfo() {
		assert.Equal(t, wki.Active, true)
	}
}

func TestIdleWorkers(t *testing.T) {
	pool := pool.New(utils.OneIdleWorker)
	defer goleak.VerifyNone(t)
	defer func() {
		closeCtx, closeCloseCtx := context.WithTimeout(context.Background(), 10*time.Second)
		defer closeCloseCtx()
		pool.Close(closeCtx)
	}()

	assert.Equal(t, 1, pool.MaxIdleWorkers)
	pool.Open()
	tasks := make([]func() error, 3, 5)
	tasks[0] = utils.Closure(0, 1)
	tasks[1] = utils.Closure(0, 2)
	tasks[2] = utils.TickerWithVarStopTime(10, 3)

	ctx, closeTimeOut := context.WithTimeout(context.Background(), 10*time.Second)
	defer closeTimeOut()
	for _, t := range tasks {
		if _, err := pool.Execute(t, ctx); err != nil {
			fmt.Println(err)
		}
	}
	pool.DeleteWorker(context.Background())

	assert.Equal(t, 2, pool.Size)

	assert.Equal(t, 3, len(pool.GetWorkersInfo()))

	counter := 0
	for _, wki := range pool.GetWorkersInfo() {
		if wki.Active == true {
			counter++
		}
	}

	assert.Equal(t, 2, counter)
}

func TestReuseIdleWorkers(t *testing.T) {
	pool := pool.New(utils.OneIdleWorker)
	defer goleak.VerifyNone(t)
	defer func() {
		closeCtx, closeCloseCtx := context.WithTimeout(context.Background(), 10*time.Second)
		defer closeCloseCtx()
		pool.Close(closeCtx)
	}()

	assert.Equal(t, 1, pool.MaxIdleWorkers)
	pool.Open()
	tasks := make([]func() error, 3, 5)
	tasks[0] = utils.Closure(0, 1)
	tasks[1] = utils.TickerWithVarStopTime(10, 2)
	tasks[2] = utils.TickerWithVarStopTime(10, 3)

	ctx, closeTimeOut := context.WithTimeout(context.Background(), 10*time.Second)
	defer closeTimeOut()
	for _, t := range tasks {
		if _, err := pool.Execute(t, ctx); err != nil {
			fmt.Println(err)
		}
	}
	pool.DeleteWorker(context.Background())

	if _, err := pool.Execute(utils.TickerWithVarStopTime(10, 2), ctx); err != nil {
		fmt.Println(err)
	}

	assert.Equal(t, 2, pool.Size)

	assert.Equal(t, 3, len(pool.GetWorkersInfo()))

	counter := 0
	for _, wki := range pool.GetWorkersInfo() {
		if wki.Active == true {
			counter++
		}
	}
	assert.Equal(t, 2, counter)

	pool.AddWorkers(1)

	time.Sleep(1 * time.Second) // нужно чтобы горутина успела вызват
	assert.Equal(t, 3, pool.Size)
	assert.Equal(t, 3, len(pool.GetWorkersInfo()))

	counter = 0
	for _, wki := range pool.GetWorkersInfo() {
		b, _ := json.Marshal(wki)
		fmt.Println(string(b))
		if wki.Active == true {
			counter++

		}
	}
	assert.Equal(t, 3, counter)

}

func TestSeveralReuseIdleWorkers(t *testing.T) {
	pool := pool.New(utils.TwoIdleWorker)
	defer goleak.VerifyNone(t)
	defer func() {
		closeCtx, closeCloseCtx := context.WithTimeout(context.Background(), 10*time.Second)
		defer closeCloseCtx()
		pool.Close(closeCtx)
	}()

	assert.Equal(t, 2, pool.MaxIdleWorkers)
	pool.Open()
	tasks := make([]func() error, 1, 5)
	tasks[0] = utils.Closure(0, 1)

	ctx, closeTimeOut := context.WithTimeout(context.Background(), 10*time.Second)
	defer closeTimeOut()
	for _, t := range tasks {
		if _, err := pool.Execute(t, ctx); err != nil {
			fmt.Println(err)
		}
	}
	pool.DeleteWorker(context.Background())
	pool.DeleteWorker(context.Background())

	if _, err := pool.Execute(utils.TickerWithVarStopTime(10, 2), ctx); err != nil {
		fmt.Println(err)
	}

	assert.Equal(t, 1, pool.Size)

	assert.Equal(t, 3, len(pool.GetWorkersInfo()))

	counter := 0
	for _, wki := range pool.GetWorkersInfo() {
		b, _ := json.Marshal(wki)
		fmt.Println(string(b))
		if wki.Active == true {
			counter++
		}
	}
	assert.Equal(t, 1, counter)

	pool.AddWorkers(2)

	time.Sleep(1 * time.Second) // нужно чтобы горутина успела вызваться
	assert.Equal(t, 3, pool.Size)
	assert.Equal(t, 3, len(pool.GetWorkersInfo()))

	counter = 0
	for _, wki := range pool.GetWorkersInfo() {
		b, _ := json.Marshal(wki)
		fmt.Println(string(b))
		if wki.Active == true {
			counter++

		}
	}
	assert.Equal(t, 3, counter)

}
