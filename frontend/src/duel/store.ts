/**
 * duelStore 单例与事件总线的接线。
 *
 * domain/duel_state.ts 保持纯净（不 import 总线），这里负责把事件总线上
 * 的每个转发事件喂给 store。组件与冒烟统一 `import { duelStore } from
 * './store.ts'`。
 *
 * 订阅名单直接复用 net/events_forward.ts 的 FORWARDED_EVENTS（Go 侧
 * events_parity_test.go 保证它与 Go emit 点对齐），store 的 reducer 对
 * 不认识的事件名是 no-op，所以多订无害。
 */
import { eventBus } from '../wails_bridge.ts';
import { FORWARDED_EVENTS } from '../net/events_forward.ts';
import { createDuelStore } from '../domain/duel_state.ts';

export const duelStore = createDuelStore();

for (const name of FORWARDED_EVENTS) {
  eventBus.on(name, (data) => duelStore.dispatch(name, data));
}
