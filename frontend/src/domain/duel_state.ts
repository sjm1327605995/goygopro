/**
 * DuelState store — 单一状态源。
 *
 * 订阅/快照模式（React 用 useSyncExternalStore 消费）：
 *   const state = useSyncExternalStore(duelStore.subscribe, duelStore.getState);
 *
 * 纯领域模块：不 import DOM/Three/事件总线；事件接线在 duel/store.js。
 * 所有状态变更都经 reducer.ts 的 applyEvent（引擎事件）或下面的 UI 动作。
 */
import {
  applyEvent, applyInspect, applyInspectInfo, applyViewSwap, initialDuelState,
  type DuelState, type CardInspection, type PopupAction,
} from './reducer.ts';

export type { DuelState, CardInspection, PileCounts, HandCard, LogEntry, BoardSide, BoardCard, PopupAction } from './reducer.ts';
export type { CardSelectState, CardSelectEntry } from './reducer.ts';

export interface DuelStore {
  getState(): DuelState;
  subscribe(fn: () => void): () => void;
  /** 事件总线→store 的唯一入口（duel/store.js 对每个转发事件调一次） */
  dispatch(name: string, data: unknown): void;
  /** UI 动作：左面板查看某张卡（随后用 resolveInspectInfo 回填卡信息） */
  inspect(code: number): void;
  resolveInspectInfo(code: number, info: Record<string, unknown> | null): void;
  /** UI 动作：清空全部对局状态（回放 seek/重开时先归零再重放） */
  reset(): void;
  /** UI 动作：手牌/场地点击的 2D 动作菜单 */
  openActionPopup(x: number, y: number, actions: PopupAction[]): void;
  closeActionPopup(): void;
  /** UI 动作：设置 stHintMsg 提示条文本（duel:hint 的 HINT_EVENT/HINT_MESSAGE） */
  setHint(text: string): void;
  /** UI 动作：select 类弹窗/落点选择取走当前 selectHint（原版 select_hint=0） */
  consumeSelectHint(): void;
  /** UI 动作：选择进行中挂上提示文本（CardTooltip 悬停附带显示） */
  armSelectHint(text: string): void;
  disarmSelectHint(): void;
  /** UI 动作：交换双方视角（原版 btnSpectatorSwap/btnReplaySwap） */
  toggleViewSwap(): void;
  /** UI 动作：select_card/unselect 的点选/取消（场上点选与弹窗共用） */
  toggleCardSelect(idx: number): void;
  /** UI 动作：应答/取消后清空 select_card/unselect 选择态 */
  endCardSelect(): void;
}

export function createDuelStore(): DuelStore {
  let state: DuelState = initialDuelState();
  const listeners = new Set<() => void>();

  const notify = () => {
    for (const fn of listeners) fn();
  };
  const commit = (next: DuelState) => {
    if (next === state) return;
    state = next;
    notify();
  };

  return {
    getState: () => state,
    subscribe(fn: () => void) {
      listeners.add(fn);
      return () => listeners.delete(fn);
    },
    dispatch(name: string, data: unknown) {
      commit(applyEvent(state, name, data));
    },
    inspect(code: number) {
      if (state.inspection && state.inspection.code === code) return;
      commit(applyInspect(state, code));
    },
    resolveInspectInfo(code: number, info: Record<string, unknown> | null) {
      commit(applyInspectInfo(state, code, info));
    },
    reset() {
      commit(initialDuelState());
    },
    openActionPopup(x: number, y: number, actions: PopupAction[]) {
      commit({ ...state, actionPopup: { x, y, actions } });
    },
    closeActionPopup() {
      if (!state.actionPopup) return;
      commit({ ...state, actionPopup: null });
    },
    setHint(text: string) {
      if (state.hint === text) return;
      commit({ ...state, hint: text });
    },
    consumeSelectHint() {
      if (state.selectHint === null) return;
      commit({ ...state, selectHint: null });
    },
    armSelectHint(text: string) {
      if (state.selectHintText === text) return;
      commit({ ...state, selectHintText: text });
    },
    disarmSelectHint() {
      if (state.selectHintText === null) return;
      commit({ ...state, selectHintText: null });
    },
    toggleViewSwap() {
      commit(applyViewSwap(state));
    },
    toggleCardSelect(idx: number) {
      const cs = state.cardSelect;
      if (!cs) return;
      // 点选/再点取消，selected 保持点击先后顺序，达到 max 上限不再新增
      // （与 useCardSelection 语义一致）；选满 max 的自动应答由消费方
      // （duel_manager 的 store 订阅）统一触发
      const selected = cs.selected.includes(idx)
        ? cs.selected.filter((i) => i !== idx)
        : cs.selected.length >= cs.max
          ? cs.selected
          : [...cs.selected, idx];
      if (selected === cs.selected) return;
      commit({ ...state, cardSelect: { ...cs, selected } });
    },
    endCardSelect() {
      if (!state.cardSelect) return;
      commit({ ...state, cardSelect: null });
    },
  };
}
