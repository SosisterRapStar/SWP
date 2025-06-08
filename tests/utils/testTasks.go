package utils

import (
	"fmt"
	"time"
)

func Ticker() error {
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

func TickerWithVarStopTime(stopDur int) func() error {

	return func() error {
		taskId := 1
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

func Closure(start int) func() error {

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
