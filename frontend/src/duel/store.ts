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
 *
 * MSG_RETRY 支持（原版 duelclient.cpp:990-1043 的 last_successful_msg 语义）
 * 也挂在这里：记录最近一条待应答询问及其 select_hint，retry 到达时恢复
 * hint 并经总线重放该询问——服务端 analyzeRetry 只重发 RETRY 字节，不重发
 * 上一询问。
 */
import { eventBus } from '../wails_bridge.ts';
import { FORWARDED_EVENTS } from '../net/events_forward.ts';
import { createDuelStore } from '../domain/duel_state.ts';
import { sysString } from '../domain/sys_strings.ts';

export const duelStore = createDuelStore();

/** 会等待玩家应答的询问事件：到达即视为「最近询问」候选（原版 curMsg 集合） */
const PROMPT_EVENTS = new Set([
  'duel:select_idlecmd', 'duel:select_battlecmd', 'duel:select_effectyn',
  'duel:select_yesno', 'duel:select_option', 'duel:select_card',
  'duel:select_unselect', 'duel:select_position', 'duel:select_place',
  'duel:select_chain', 'duel:select_counter', 'duel:select_sum',
  'duel:sort_card', 'duel:announce_race', 'duel:announce_attrib',
  'duel:announce_card', 'duel:announce_number', 'duel:rps',
]);

/** retry 错误文案按上一条询问类型选取（duelclient.cpp:994-1035 的 switch） */
const RETRY_ERR_ID: Record<string, number> = {
  'duel:announce_card': 1422,
  'duel:announce_attrib': 1423,
  'duel:announce_race': 1424,
  'duel:announce_number': 1425,
  'duel:select_effectyn': 1426,
  'duel:select_yesno': 1426,
  'duel:select_option': 1426,
  'duel:select_card': 1427,
  'duel:select_unselect': 1427,
  'duel:select_sum': 1427,
  'duel:sort_card': 1427,
  'duel:select_chain': 1428,
  'duel:select_place': 1429,
  'duel:select_position': 1430,
  'duel:select_counter': 1431,
};

let lastPrompt: { name: string; data: unknown } | null = null;
let lastSelectHint: number | null = null;

for (const name of FORWARDED_EVENTS) {
  eventBus.on(name, (data) => {
    if (name === 'duel:start') {
      // 新局清空 retry 记录
      lastPrompt = null;
      lastSelectHint = null;
    }
    duelStore.dispatch(name, data);
    if (PROMPT_EVENTS.has(name)) {
      lastPrompt = { name, data };
      lastSelectHint = duelStore.getState().selectHint;
    }
  });
}

eventBus.on('duel:retry', () => {
  // 原版：弹错误提示（SysString 1421-1431）→ 恢复 select_hint →
  // 重跑 last_successful_msg 重新打开同一询问。
  duelStore.setHint(sysString(RETRY_ERR_ID[lastPrompt?.name || ''] ?? 1421));
  if (lastSelectHint != null) duelStore.restoreSelectHint(lastSelectHint);
  if (!lastPrompt) return;
  const prompt = lastPrompt;
  // 经总线异步重放：让当前 retry 事件先落定，再重开弹窗（原版模态 wMessage
  // 点击后的行为等价）
  queueMicrotask(() => eventBus.emit(prompt.name, prompt.data));
});
