package easygpool

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	syncx "github.com/gofish2020/easygpool/tool/sync"
	"github.com/gofish2020/easygpool/tool/wait"
)

const (
	maxIdleTime = 1 * time.Second
)

/*
Pool：用来管理协程对象 *goWorker
*/
type Pool struct {
	// 协程对象最大个数
	capactiy int32
	// 活跃的协程对象个数
	running atomic.Int32

	// 阻塞模式
	waiting atomic.Int32 // 阻塞等待的数量
	cond    *sync.Cond

	// Pool是否关闭
	closed chan struct{}
	// 配置信息
	option *Options

	// 回收对象
	victim sync.Pool
	// 操作local对象的锁
	lock sync.Locker
	// 协程对象缓存
	local goWorkerCache

	// 判断goPurge是否已经结束
	purgeDone int32
	// 等待协程对象停止（包括协程池 和用户协程）
	allRunning *wait.Wait
}

// capacity: -1表示不限定 >0表示最大的协程个数
func NewPool(capacity int32, opts ...Option) *Pool {

	option := new(Options)
	for _, opt := range opts {
		opt(option)
	}

	//如果启用清理，清理间隔的值需要 >0
	if !option.DisablePurge {
		if option.MaxIdleTime <= 0 {
			option.MaxIdleTime = maxIdleTime
		}
	}

	// 日志打印器
	if option.Logger == nil {
		option.Logger = defaultLogger
	}

	p := Pool{
		capactiy:   capacity,
		closed:     make(chan struct{}),
		lock:       syncx.NewSpinLock(),
		option:     option,
		allRunning: &wait.Wait{},
		local:      newCacheStack(),
	}

	p.victim.New = func() any {
		return newGoWorker(&p)
	}
	p.cond = sync.NewCond(p.lock)

	go p.goPurge()
	return &p
}

func (p *Pool) Waiting() int32 {
	return p.waiting.Load()
}

func (p *Pool) Running() int32 {
	return p.running.Load()
}

func (p *Pool) Cap() int32 {
	return p.capactiy
}

func (p *Pool) Free() int32 {
	if p.Cap() < 0 {
		return -1
	}
	return p.Cap() - p.Running()
}

// 调整容量
func (p *Pool) Tune(capacity int32) {
	oldCap := p.Cap()
	if capacity < 0 {
		capacity = -1
	}
	if oldCap == capacity {
		return
	}

	atomic.StoreInt32(&p.capactiy, capacity)

	if p.Cap() < 0 { // 调整为无限容量
		p.cond.Broadcast()
		return
	}

	// 调整的容量大于之前的容量（说明有更多的空闲）
	if oldCap > 0 && p.Cap() > oldCap {
		p.cond.Broadcast()
	}
}

// 外部提交任务
func (p *Pool) Submit(task TaskFunc) error {

	if p.IsClosed() {
		return ErrPoolClosed
	}

	w, err := p.getGoWorker()
	if err != nil {
		return err
	}
	w.exec(task)
	return nil
}

func (p *Pool) getGoWorker() (*goWorker, error) {
	// 上锁（避免并发问题）
	p.allRunning.Add(1)
	p.lock.Lock()
	defer func() {
		p.lock.Unlock()
		p.allRunning.Add(-1)
	}()

	for {

		if p.IsClosed() {
			return nil, ErrPoolClosed
		}

		// 1.从local缓冲中获取
		if w := p.local.detach(); w != nil {
			return w, nil
		}

		// 2. 从victim中获取（新建一个）
		if p.Cap() == -1 || p.Cap() > p.running.Load() { // 说明还有容量
			raw := p.victim.Get()
			w, ok := raw.(*goWorker)
			if !ok {
				return nil, errors.New("victim cache data is wrong type")
			}
			w.start()
			return w, nil
		}

		//3. 执行到这里，说明没有空闲的协程对象
		//  非阻塞模式 or 阻塞模式（但是阻塞的太多了）
		if p.option.NonBlocking || (p.option.MaxBlockingTasks != 0 && p.Waiting() >= p.option.MaxBlockingTasks) {
			return nil, ErrPoolOverload
		}

		//4. 阻塞等待
		p.waiting.Add(1)
		p.cond.Wait() // 这里会对p.lock解锁，然后阻塞等待；被唤醒后，又对p.lock上锁
		p.waiting.Add(-1)
	}

}

func (p *Pool) Close() {
	close(p.closed)

	p.lock.Lock()
	p.local.reset()
	p.lock.Unlock()
	p.cond.Broadcast()

	p.allRunning.Wait()
}

func (p *Pool) CloseWithTimeOut(timeout time.Duration) error {
	close(p.closed)

	p.lock.Lock()
	p.local.reset()
	p.lock.Unlock()
	p.cond.Broadcast()

	p.allRunning.WaitWithTimeOut(timeout)

	if p.Running() != 0 || p.Waiting() != 0 || (!p.option.DisablePurge && atomic.LoadInt32(&p.purgeDone) == 0) {
		return ErrTimeout
	}
	return nil
}
func (p *Pool) IsClosed() bool {
	select {
	case <-p.closed:
		return true
	default:
	}
	return false
}

// 定期清理：长时间未使用的*goWorker
func (p *Pool) goPurge() {

	if p.option.DisablePurge {
		return
	}

	ticker := time.NewTicker(p.option.MaxIdleTime)
	defer func() {
		ticker.Stop()
		atomic.StoreInt32(&p.purgeDone, 1)
	}()

	for {
		select {
		case <-p.closed:
			return
		case <-ticker.C: // 1s检测一次

			isDormant := false
			p.lock.Lock()
			// 获取空闲的协程对象
			n := p.Running()
			expireWorkers := p.local.clearExpire(p.option.MaxIdleTime)
			if n == 0 || n == int32(len(expireWorkers)) { // 如果全部都被清理掉
				isDormant = true
			}
			p.lock.Unlock()

			for i := range expireWorkers {
				expireWorkers[i].stop()
				expireWorkers[i] = nil
			}
			// 通知所有阻塞等待的任（激活）
			if isDormant && p.Waiting() > 0 {
				p.cond.Broadcast()
			}
		}
	}

}

// 循环利用协程对象
func (p *Pool) recycle(gw *goWorker) bool {

	// 容量已经满了(不回收)
	if (p.Cap() > 0 && p.Cap() < p.Running()) || p.IsClosed() {
		p.cond.Broadcast()
		return false
	}
	// 修改最后一次使用时间（回收）
	gw.lastUsedTime = time.Now()
	p.lock.Lock()
	if err := p.local.insert(gw); err != nil {
		p.lock.Unlock()
		return false
	}
	p.lock.Unlock()

	// 通知 `getGoWorker()`函数有 *goWorker可以使用
	p.cond.Signal()
	return true
}
