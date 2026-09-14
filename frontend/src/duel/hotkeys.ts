/**
 * 决斗界面全局快捷键（gframe event_handler.cpp:1734-1820 对局处理 +
 * :2071-2110 F1-F8 的翻译）。
 *
 * 语义核对（vendored gframe）：
 *  - A/S/D：仅 control_mode==0（键鼠模式）生效，**按住**生效——
 *    KeyPress 置位、KeyRelease 清零（React 的 keyup/keydown 对应）；
 *    三者互斥；只改本地连锁判定（chain_prefs），不发送任何包。
 *    A=ignore_chain / S=always_chain / D=chain_when_avail。
 *  - F1-F8：**松开时**触发（原版 PressedDown 即处理即 break，等效于
 *    一次触发）。F1-F4=自己 墓地/除外/额外/超量素材，
 *    F5-F8=对方同序。仅在决斗进行中且非回放时可用。
 *  - R/F9（字体透明）与 ESC 最小化窗口是桌面窗口行为，浏览器中
 *    不翻译；ESC 这里改为关闭卡片列表浮层。
 *
 * 输入框焦点时全部跳过（聊天/改名输入不能被劫持）。
 */
import { eventBus } from '../wails_bridge.ts';
import { settingsStore } from '../domain/settings.ts';
import { duelStore } from './store.ts';
import chainPrefs, { type ChainPrefMode } from './chain_prefs.ts';

/** F1-F8 → { player, pile }；顺序照原版：墓地/除外/额外/超量素材 */
export const PILE_HOTKEYS: Record<string, { player: number; pile: string }> = {
  F1: { player: 0, pile: 'grave' },
  F2: { player: 0, pile: 'banish' },
  F3: { player: 0, pile: 'extra' },
  F4: { player: 0, pile: 'overlay' },
  F5: { player: 1, pile: 'grave' },
  F6: { player: 1, pile: 'banish' },
  F7: { player: 1, pile: 'extra' },
  F8: { player: 1, pile: 'overlay' },
};

const CHAIN_KEYS: Record<string, ChainPrefMode> = {
  a: 'ignore',
  s: 'always',
  d: 'whenAvail',
};

function inTextField(): boolean {
  const el = document.activeElement as HTMLElement | null;
  if (!el) return false;
  const tag = el.tagName;
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || el.isContentEditable;
}

export function installHotkeys(): () => void {
  // 原版 isStarted 条件：决斗开始过才可看列表（非回放——回放 theater
  // 页面不挂载本模块，无需另判）。
  const inDuel = (): boolean => duelStore.getState().started;

  const heldMode: { key: string | null } = { key: null };

  const onKeyDown = (e: KeyboardEvent): void => {
    if (e.repeat || e.ctrlKey || e.altKey || e.metaKey || inTextField()) return;
    const mode = CHAIN_KEYS[e.key.toLowerCase()];
    if (mode) {
      // control_mode==0 才启用三键（原版 checkKeyManagement 前提）
      if (Number(settingsStore.get('control_mode')) !== 0) return;
      heldMode.key = e.key.toLowerCase();
      chainPrefs.setMode(mode);
    }
  };

  const onKeyUp = (e: KeyboardEvent): void => {
    const lower = e.key.toLowerCase();
    if (heldMode.key === lower) {
      heldMode.key = null;
      chainPrefs.setMode(null);
    }
    const pile = PILE_HOTKEYS[e.key];
    if (pile && inDuel()) {
      eventBus.emit('ui:show_pile', pile);
    }
  };

  window.addEventListener('keydown', onKeyDown);
  window.addEventListener('keyup', onKeyUp);
  return () => {
    window.removeEventListener('keydown', onKeyDown);
    window.removeEventListener('keyup', onKeyUp);
  };
}
