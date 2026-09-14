/**
 * 设置中心：system.conf 配置的前端镜像与持久化。
 *
 * 语义对照 vendored gframe game.h:47-104 的 Config 结构体（键名与
 * system.conf 一一对应，Go 侧 cmd/wails/config.go 做读写）。前端启动时
 * load() 拉一次全量快照，之后每次 set() 乐观更新本地并 debounce 写回
 * ——原版是退出时整体落盘，Web 客户端随时可能被关，改为每次变更即存。
 *
 * 消费方（波 A 接线）：
 *   soundManager（音量/开关）、ChatOverlay（屏蔽聊天）、chain_prefs 初始态、
 *   Lobby（上次昵称/地址/卡组回填）、PlayerPanel（隐藏玩家名）、
 *   field3d（场地魔法背景开关、快速动画）、CardPreviewPanel（隐藏系列名）。
 */
import { WailsBridge } from '../wails_bridge.ts';

/** 全部配置键与默认值（Go defaultConfig() 的镜像） */
export const SETTINGS_DEFAULTS: Record<string, any> = {
  // 决斗辅助
  automonsterpos: 0,
  autospellpos: 1,
  randompos: 0,
  autochain: 0,
  waitchain: 0,
  showchain: 0,
  quick_animation: 0,
  auto_save_replay: 0,
  draw_single_chain: 0,
  // 系统设置
  mute_opponent: 0,
  mute_spectators: 0,
  hide_player_name: 0,
  ignore_deck_changes: 0,
  auto_search_limit: -1,
  search_multiple_keywords: 1,
  draw_field_spell: 1,
  separate_clear_button: 1,
  hide_setname: 0,
  hide_hint_button: 0,
  swap_yes_no_button: 0,
  resize_select_window: 1,
  resize_popup_menu: 0,
  control_mode: 0,
  prefer_expansion_script: 0,
  // 音频
  enable_sound: true,
  enable_music: true,
  sound_volume: 50,
  music_volume: 50,
  music_mode: 1,
  // 卡组/禁限/规则
  use_lflist: 1,
  default_lflist: 0,
  default_rule: 0,
  defaultOT: 1,
  // 大厅记忆
  nickname: '',
  gamename: '',
  lasthost: '',
  lastport: '',
  lastcategory: '',
  lastdeck: '',
  serverport: 7911,
};

type Listener = () => void;

class SettingsStore {
  private values: Record<string, any> = { ...SETTINGS_DEFAULTS };
  private listeners: Listener[] = [];
  private loaded = false;
  private saveTimer: ReturnType<typeof setTimeout> | null = null;
  private pendingPatch: Record<string, any> = {};

  // 方法全部用箭头字段绑定：getSnapshot/subscribe 会被当回调传给
  // useSyncExternalStore(settingsStore.subscribe, settingsStore.getSnapshot)，
  // 不绑定的话 this 是 undefined（settings_smoke 抓到的崩溃）。
  get = <K extends keyof typeof SETTINGS_DEFAULTS>(key: K): (typeof SETTINGS_DEFAULTS)[K] =>
    this.values[key as string];

  /** useSyncExternalStore 快照（引用稳定：只在变更时换对象） */
  getSnapshot = (): Record<string, any> => this.values;

  subscribe = (fn: Listener): (() => void) => {
    this.listeners.push(fn);
    return () => {
      this.listeners = this.listeners.filter((f) => f !== fn);
    };
  };

  /** 启动时拉取一次全量快照；重复调用幂等。 */
  load = async (): Promise<Record<string, any>> => {
    if (this.loaded) return this.values;
    this.loaded = true;
    try {
      const conf = await WailsBridge.getConfig();
      this.values = { ...SETTINGS_DEFAULTS, ...conf };
    } catch {
      this.values = { ...SETTINGS_DEFAULTS };
    }
    this.listeners.forEach((fn) => fn());
    return this.values;
  };

  /** 乐观更新 + debounce 落盘（键必须是合法 conf 键）。 */
  set = (key: string, value: any): void => {
    if (!(key in SETTINGS_DEFAULTS)) return;
    this.values = { ...this.values, [key]: value };
    this.listeners.forEach((fn) => fn());
    this.pendingPatch[key] = value;
    if (this.saveTimer) clearTimeout(this.saveTimer);
    this.saveTimer = setTimeout(() => {
      const patch = this.pendingPatch;
      this.pendingPatch = {};
      this.saveTimer = null;
      WailsBridge.saveConfig(patch).catch(() => {});
    }, 300);
  };
}

export const settingsStore = new SettingsStore();
