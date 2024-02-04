package easygpool

import "time"

type cacheStack struct {
	items  []*goWorker
	expiry []*goWorker
}

func newCacheStack() *cacheStack {
	return &cacheStack{
		items: make([]*goWorker, 0),
	}
}

func (cs *cacheStack) len() int {
	return len(cs.items)
}

func (cs *cacheStack) isEmpty() bool {
	return len(cs.items) == 0
}

func (cs *cacheStack) insert(w *goWorker) error {
	cs.items = append(cs.items, w)
	return nil
}

func (cs *cacheStack) detach() *goWorker {
	l := cs.len()
	if l == 0 {
		return nil
	}

	w := cs.items[l-1]
	cs.items[l-1] = nil // avoid memory leaks
	cs.items = cs.items[:l-1]

	return w
}

func (cs *cacheStack) clearExpire(duration time.Duration) []*goWorker {
	n := cs.len()
	if n == 0 {
		return nil
	}

	expiryTime := time.Now().Add(-duration)
	index := cs.binarySearch(0, n-1, expiryTime)

	cs.expiry = cs.expiry[:0]
	if index != -1 {
		cs.expiry = append(cs.expiry, cs.items[:index+1]...)
		m := copy(cs.items, cs.items[index+1:]) // 移动 [index+1:]范围数据到头部
		for i := m; i < n; i++ {
			cs.items[i] = nil // 将尾部的数据设定为nil
		}
		cs.items = cs.items[:m] // 修改长度为m
	}
	return cs.expiry
}

func (cs *cacheStack) binarySearch(l, r int, expiryTime time.Time) int {
	for l <= r {
		mid := l + ((r - l) >> 1)                          // avoid overflow when computing mid
		if expiryTime.Before(cs.items[mid].lastUsedTime) { // 这里说明不过期，r值左移
			r = mid - 1
		} else {
			l = mid + 1
		}
	}
	return r
}

func (cs *cacheStack) reset() {
	for i := 0; i < cs.len(); i++ {
		cs.items[i].stop()
		cs.items[i] = nil
	}
	cs.items = cs.items[:0]
}
