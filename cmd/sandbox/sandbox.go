package main

import (
	"fmt"
)

func main() {
	ch := make(chan int)

	go func() {
		close(ch)
	}()

	for range ch {
		fmt.Println(2)
	}

	fmt.Println(1)
}
