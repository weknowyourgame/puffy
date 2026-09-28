package main

import (
	"fmt"
	"sync"
)

type Request struct {
	value int
	reply chan int
}

func main() {
	ch := make(chan Request)

	var wg sync.WaitGroup

	// Start 10 goroutines
	for i := 0; i < 10; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			reply := make(chan int)

			ch <- Request{
				value: i,
				reply: reply,
			}

			answer := <-reply

			fmt.Println(answer)
		}()
	}

	go func() {
		for req := range ch {
			result := req.value * 2

			req.reply <- result
		}
	}()

	wg.Wait()

	close(ch)
}
