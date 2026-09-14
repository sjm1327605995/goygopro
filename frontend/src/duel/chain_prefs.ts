/**
 * 连锁模式偏好（原版 gframe 的 ignore_chain / always_chain /
 * chain_when_avail 三键，game.cpp:916-923）。
 *
 * 包语义（已核对 vendored 源码）：这三个键不发送任何包——它们只影响客户端
 * 收到 MSG_SELECT_CHAIN 时是否自动代答 -1（duelclient.cpp:1776）：
 *   - ignore_chain：有可发动也一律自动放弃连锁；
 *   - always_chain：永不自动放弃（即使本判定无可发动也保持询问）；
 *   - chain_when_avail：有任何可发动时就询问。
 * 三者互斥（原版 isPushButton 单选）。
 */

export type ChainPrefMode = 'ignore' | 'always' | 'whenAvail';

// system.conf 初始映射（原版 tabHelper 复选框 game.cpp:380-501 只写
// always_chain/chain_when_avail 两个键，autochain→always、waitchain→whenAvail；
// 两者全关 = 不预设，全开时原版后写的生效，这里同样取 waitchain 优先）。
export function chainPrefFromSettings(autochain: number, waitchain: number): ChainPrefMode | null {
  if (waitchain) return 'whenAvail';
  if (autochain) return 'always';
  return null;
}

type ChainPrefListener = (mode: ChainPrefMode | null) => void;

let listeners: ChainPrefListener[] = [];

const chainPrefs = {
  mode: null as ChainPrefMode | null,

  set(mode: ChainPrefMode) {
    // 同一键再点一次 = 取消（原版 push button 行为）
    this.mode = this.mode === mode ? null : mode;
    listeners.forEach((fn) => fn(this.mode));
  },

  // 直接赋值（含 null）：A/S/D 按住热键用——按住设模式、松开清空，
  // 不带 toggle 语义。波 F 快捷键。
  setMode(mode: ChainPrefMode | null) {
    if (this.mode === mode) return;
    this.mode = mode;
    listeners.forEach((fn) => fn(this.mode));
  },

  get(): ChainPrefMode | null {
    return this.mode;
  },

  subscribe(fn: ChainPrefListener) {
    listeners.push(fn);
    return () => {
      listeners = listeners.filter((f) => f !== fn);
    };
  },
};

export default chainPrefs;
