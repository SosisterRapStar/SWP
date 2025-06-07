package main

import "fmt"

func reuseWorker() {
	arr := make([]int, 10)
	for len(arr) > 0 {
		arr = arr[:len(arr)-1]
		fmt.Println(len(arr))
	}
}

func main() {
	reuseWorker()
}
