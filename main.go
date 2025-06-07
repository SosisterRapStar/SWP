package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/SosisterRapStar/SWP/pool"
	"github.com/google/uuid"
)

func testTask() error {
	taskId := uuid.New()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop() // Важно: освобождаем ресурсы тикера

	timeout := time.After(5 * time.Second)

	for {
		select {
		case <-ticker.C:
			fmt.Printf("Tick: %v\n", taskId)
		case <-timeout:
			fmt.Printf("Stopped: %v\n", taskId)
			return nil
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
	if _, err := pool.Execute(testTask, ctx); err != nil {
		fmt.Println(err)
	}

	time.Sleep(10 * time.Second)
}
