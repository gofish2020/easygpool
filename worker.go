package easygpool

import (
	"runtime/debug"
	"time"
)

/*
协程对象：用于处理任务
*/

type TaskFunc func()

type goWorker struct {
	taskChan chan TaskFunc
	pool     *Pool

	// 最后一次使用的时间
	lastUsedTime time.Time
}

func newGoWorker(pool *Pool) *goWorker {
	gw := goWorker{}
	gw.pool = pool
	gw.taskChan = make(chan TaskFunc)
	return &gw
}

// 启动协程（1.等待任务 2.任务处理完成，自动回收到local中）
func (gw *goWorker) start() {
	gw.pool.running.Add(1)
	gw.pool.allRunning.Add(1)
	go func() {
		defer func() {
			gw.pool.running.Add(-1)
			gw.pool.allRunning.Add(-1)
			gw.pool.victim.Put(gw) // 执行失败，回收到victim中

			if p := recover(); p != nil {
				handler := gw.pool.option.PanicHandler
				if handler != nil {
					handler(p)
				} else {
					gw.pool.option.Logger.Printf("*goWorker panic err:%+v\n stack info: %s\n", p, debug.Stack())
				}
			}
			gw.pool.cond.Signal()
		}()

		for task := range gw.taskChan { // 阻塞等待任务
			if task == nil {
				return
			}
			task()
			// 执行成功，自动回收到local中
			if !gw.pool.recycle(gw) {
				return
			}
		}
	}()
}

func (gw *goWorker) stop() {
	gw.taskChan <- nil
}

func (gw *goWorker) exec(task TaskFunc) {
	gw.taskChan <- task
}
