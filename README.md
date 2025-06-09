# SWP
Simple Worker Pool package
Allows you to dynamically add and remove workers, helps to control the number of goroutines.

# Import
`go get github.com/SosisterRapStar/SWP/pool `

# Example of usage
```Go
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/SosisterRapStar/SWP/pool"
)

func TickerWithVarStopTime(stopDur int, id int) func() error {

	return func() error {
		taskId := id
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		timeout := time.After(time.Duration(stopDur) * time.Second)

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

}

func Closure(start int, id int) func() error {

	return func() error {
		taskId := id
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
	wp, err := pool.New(pool.DefaultConfig())
	wp.Open()

	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	errchan, err := wp.Execute(TickerWithVarStopTime(10, 1), ctx)

	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	// waiting for error
	go func() {
		fmt.Println(<-errchan)
	}()

	_, err = wp.Execute(Closure(0, 2), ctx)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	wp.Close(context.Background())

}
```

