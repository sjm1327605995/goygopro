package scenes

import (
	"github.com/sjm1327605995/goygopro/client"
	ui "github.com/sjm1327605995/tenon/pkg/ui"
)

// UseStore 订阅一个 client.Store，值变了就重渲染本组件。
//
// 关键在 ui.Post：Store 的回调跑在改状态的那个 goroutine 上（决斗时是网络线程），
// 而 tenon 的 setState 只能在渲染线程调。Post 把闭包排到下一帧之前由渲染线程执行。
// 直接 setState 会撞坏 fiber 树，且症状是随机的 —— 不要图省事跳过。
func UseStore[T any](s *client.Store[T]) T {
	v, setV := ui.UseState(s.Get())
	ui.UseEffect(func() ui.Cleanup {
		unsub := s.Subscribe(func(nv T) {
			ui.Post(func() { setV(nv) })
		})
		// 订阅建立前的一瞬间可能已经变过值，补一次当前值，否则会停在旧值上。
		ui.Post(func() { setV(s.Get()) })
		return unsub
	})
	return v
}

// UseRevision 订阅一个变更计数器，返回当前版本号。用于 DField/DInfo 这类整块改写的状态：
// 组件拿到新版本号就重渲染，渲染时直接读 MainGame 的最新值。
func UseRevision(r *client.Revision) uint64 {
	v, setV := ui.UseState(r.Get())
	ui.UseEffect(func() ui.Cleanup {
		unsub := r.Subscribe(func(nv uint64) {
			ui.Post(func() { setV(nv) })
		})
		ui.Post(func() { setV(r.Get()) })
		return unsub
	})
	return v
}
