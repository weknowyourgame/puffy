package main

// package scratchpad_test

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
)

// import (
// 	"fmt"
// 	"sync"
// )

// type Request2 struct {
// 	value int
// 	reply chan int
// }

// func main() {
// 	ch := make(chan Request)

// 	var wg sync.WaitGroup

// 	// Start 10 goroutines
// 	for i := 0; i < 10; i++ {
// 		wg.Add(1)

// 		go func() {
// 			defer wg.Done()

// 			reply := make(chan int)

// 			ch <- Request{
// 				value: i,
// 				reply: reply,
// 			}

// 			answer := <-reply

// 			fmt.Println(answer)
// 		}()
// 	}

// 	go func() {
// 		for req := range ch {
// 			result := req.value * 2

// 			req.reply <- result
// 		}
// 	}()

// 	wg.Wait()

// 	close(ch)
// }

func main() {
	if err := scratchpad("../testdata/siftsmall/siftsmall_base.fvecs"); err != nil {
		fmt.Println("error:", err)
	}
}

func scratchpad(filepath string) error {
	file, _ := os.ReadFile(filepath)

	// Read first 4 bytes -> gives us the dim
	dim := int(binary.LittleEndian.Uint32(file[:4]))
	recordSize := 4 + dim*4

	if len(file)%recordSize != 0 {
		return fmt.Errorf("corrupt file: size is not divisible by record size")
	}

	vectorCount := len(file) / recordSize

	fmt.Println("vector count:", vectorCount)

	data := make([]float32, vectorCount*dim)
	for i := 0; i < vectorCount; i++ {
		offset := i * recordSize
		for j := 0; j < dim; j++ {
			b := offset + 4 + j*4
			bits := binary.LittleEndian.Uint32(file[b : b+4])
			data[i*dim+j] = math.Float32frombits(bits)
			// fmt.Println(data[0:20])
			// fmt.Println(data[128:148])
		}
	}
	return nil
}
