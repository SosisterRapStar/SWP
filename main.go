package main

import "fmt"

func some(done chan struct{}) {
	for {
		select {
		case done <- struct{}{}:
			fmt.Println("chan")
			return
		default:
			fmt.Println("default")
		}
	}
}

func main() {
	done := make(chan struct{})
	go func() {
		fmt.Print("started to listen")
		<-done
	}()

	some(done)
}
