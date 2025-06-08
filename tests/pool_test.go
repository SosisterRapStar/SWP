package tests

import (
	"context"
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

func TestWith(t *testing.T) {
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
