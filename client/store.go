package client

import "sync"

// Store 是一个可订阅的值。它刻意不认识任何 GUI 库：网络 goroutine 改状态、UI 层订阅重绘，
// 两边只通过这里交换，client 包才不会被绑死在某个渲染框架上。
//
// 订阅回调在调用 Set 的那个 goroutine 上执行（通常是网络线程），**不是**渲染线程。
// UI 层必须自己把 setState 送回渲染线程（tenon 的 ui.Post），见 scenes.UseStore。
type Store[T any] struct {
	mu     sync.RWMutex
	value  T
	subs   map[int]func(T)
	nextID int
}

func NewStore[T any](initial T) *Store[T] {
	return &Store[T]{value: initial, subs: map[int]func(T){}}
}

func (s *Store[T]) Get() T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.value
}

// Set 写入新值并通知订阅者。通知时不持锁 —— 回调里可能反过来读 Store（UI 重绘就会），
// 持锁通知会自锁死。
func (s *Store[T]) Set(v T) {
	s.mu.Lock()
	s.value = v
	subs := make([]func(T), 0, len(s.subs))
	for _, fn := range s.subs {
		subs = append(subs, fn)
	}
	s.mu.Unlock()
	for _, fn := range subs {
		fn(v)
	}
}

// Subscribe 注册回调，返回取消订阅的函数。取消必须做到：UI 卸载后回调还在跑，
// 就会往已经没人看的组件里塞状态。
func (s *Store[T]) Subscribe(fn func(T)) func() {
	s.mu.Lock()
	id := s.nextID
	s.nextID++
	s.subs[id] = fn
	s.mu.Unlock()
	return func() {
		s.mu.Lock()
		delete(s.subs, id)
		s.mu.Unlock()
	}
}

// Revision 是一个只用来说「有东西变了」的计数器，给那些状态藏在大结构体里（ClientField、
// DuelInfo）、不值得逐字段拆成 Store 的地方用：改完调 Bump，UI 收到后整体重读。
type Revision struct {
	store *Store[uint64]
}

func NewRevision() *Revision { return &Revision{store: NewStore[uint64](0)} }

func (r *Revision) Bump() { r.store.Set(r.store.Get() + 1) }

func (r *Revision) Get() uint64 { return r.store.Get() }

func (r *Revision) Subscribe(fn func(uint64)) func() { return r.store.Subscribe(fn) }
