package sync

import (
	"runtime"
	"sync"
	"sync/atomic"
)

// spinlock 本质就是一个uint32的正整数
type spinLock uint32

const maxBackoff = 16

func (sl *spinLock) Lock() {
	backoff := 1
	for !atomic.CompareAndSwapUint32((*uint32)(sl), 0, 1) {
		// 这个的意思就是暂时让出时间片 1次 2次 4次 8次 16次，backoff 数值越大，让出的时间片的次数也就越多(最大为16)
		for i := 0; i < backoff; i++ {
			runtime.Gosched()
		}
		if backoff < maxBackoff {
			backoff <<= 1 // backoff *= 2
		}
	}
}

// 修改为0
func (sl *spinLock) Unlock() {
	atomic.StoreUint32((*uint32)(sl), 0)
}

// 自旋锁
func NewSpinLock() sync.Locker {
	return new(spinLock)
}
