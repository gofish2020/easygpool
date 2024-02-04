# Go协程池

解决的问题：

- 当需要创建大量的 Goroutine的时候，如果不限定Goroutine的数量，将是对应用的巨大灾难
- 使用完的Goroutinue可以复用继续执行下一个任务（而不是立即销毁），如果每次都创建新的Goroutinue执行任务，频繁的创建销毁Goroutinue导致利用率低下

## 逻辑图

![](pool.png)