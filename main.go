package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/SosisterRapStar/SWP/pool"
)

// test configs

func testTaskTicker() error {
	taskId := 1
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	timeout := time.After(5 * time.Second)

	for {
		select {
		case <-ticker.C:
			fmt.Printf("Task %v: tick \n", taskId)
		case <-timeout:
			fmt.Printf("Stopped: %v\n", taskId)
			return nil
		}
	}
}

func testTaskClosure(start int) func() error {

	return func() error {
		taskId := 2
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		timeout := time.After(5 * time.Second)

		for {
			select {
			case <-ticker.C:
				start++
				fmt.Printf("Task %v: %v\n", taskId, start)
			case <-timeout:
				fmt.Printf("Stopped: %v\n", taskId)
				return nil
			}
		}
	}
}

func main() {
	// writer := bufio.NewWriter(bufio.NewWriter())
	// defer writer.Flush()
	pool := pool.New(pool.WorkerPoolConfig{
		MaxIdleWorkers: 10,
		InitialSize:    10,
		WaitQueueSize:  10,
		InfoWriter:     os.Stderr})
	pool.Open()
	ctx, closeTimeOut := context.WithTimeout(context.Background(), 10*time.Second)
	defer closeTimeOut()
	if _, err := pool.Execute(testTaskTicker, ctx); err != nil {
		fmt.Println(err)
	}
	if _, err := pool.Execute(testTaskClosure(4), ctx); err != nil {
		fmt.Println(err)
	}

	// time.Sleep(10 * time.Second)
	pool.Close(ctx)
	fmt.Print("ds")
}
