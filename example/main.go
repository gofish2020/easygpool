package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/gofish2020/easygpool"
)

func demoFunc() {
	time.Sleep(1 * time.Second)
	fmt.Println("Hello World!")
}

func main() {

	runTimes := 5

	var options []easygpool.Option
	options = append(options, easygpool.SetNonBlocking(false)) // 阻塞模式，并不限定阻塞的数量

	gPool := easygpool.NewPool(1, options...) // 容量1
	var wg sync.WaitGroup
	syncCalculateSum := func() {
		demoFunc()
		wg.Done()
	}

	/*
		i = 0 1 2 3 4 串行执行
	*/
	for i := 0; i < runTimes; i++ {
		wg.Add(1)
		_ = gPool.Submit(syncCalculateSum)
	}

	/*
		i = 0 1 2 3 4 并行执行
	*/
	for i := 0; i < runTimes; i++ {
		wg.Add(1)
		go func() {
			_ = gPool.Submit(syncCalculateSum)
		}()
	}
	wg.Wait()
	fmt.Printf("running goroutines: %d\n", gPool.Running())
	fmt.Printf("finish all tasks.\n")

	err := gPool.CloseWithTimeOut(3 * time.Second)
	if err != nil {
		fmt.Printf("gPool close overtime: %+v\n", err)
		fmt.Printf("running goroutines: %d\n", gPool.Running())
	}
}
