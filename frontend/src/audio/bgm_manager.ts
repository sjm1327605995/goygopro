/**
 * BGM 管理器（原版 sound_manager.cpp PlayBGM 的 Web Audio 合成版）。
 *
 * 原版按 ./sound/BGM/<scene>/ 目录播 mp3/ogg：menu/duel/deck/advantage/
 * disadvantage/win/lose 七类场景 + BGM_ALL（music_mode 关时全池随机）。
 * 本仓库没有音频资源，改为每场景一套和弦进行的循环氛围合成：
 *   - 每小节一组 pad 和弦（attack/release 包络，低通滤波）
 *   - duel 系场景叠 bass 脉冲与分解和弦琶音，不同场景不同音型/速度
 * 设置语义与原版一致：
 *   enable_music 关 → 完全不播；music_volume → 总音量；
 *   music_mode 0 → 不按场景细分（用通用 duel 音型）。
 *
 * 场景切换来源（App.tsx 接线）：
 *   - 路由：menu/lobby/single/replay → menu，deck → deck，duel → duel
 *   - 决斗内：LP 优劣（advantage/disadvantage/平势 duel）、胜负（win/lose）
 * 与原版 PlayBGM 同语义：同场景且正在播 → 不打断重排（scene 相同即 no-op）。
 */

export type BgmScene =
  | 'menu' | 'duel' | 'deck'
  | 'advantage' | 'disadvantage'
  | 'win' | 'lose';

/** MIDI 音高 → 频率 */
const nf = (n: number): number => 440 * Math.pow(2, (n - 69) / 12);

interface BgmSceneDef {
  /** pad 和弦进行（MIDI 音高数组，每小节一个和弦） */
  chords: number[][];
  /** 每小节 bass 根音（MIDI） */
  bass: number[];
  /** 小节时长 ms */
  barMs: number;
  wave: OscillatorType;
  /** 低通截止 Hz（明亮度） */
  cutoff: number;
  /** bass 脉冲间隔 ms（0 = 不弹 bass） */
  bassEveryMs: number;
  /** 琶音：相对和弦根音的半音步进（空 = 不弹琶音） */
  arpSteps: number[];
  arpEveryMs: number;
  gain: number;
}

const SCENES: Record<BgmScene, BgmSceneDef> = {
  // 主菜单：Cmaj7 - Am7 - Fmaj7 - G，舒缓垫音
  menu: {
    chords: [[60, 64, 67, 71], [57, 60, 64, 67], [53, 57, 60, 65], [55, 59, 62, 67]],
    bass: [36, 33, 29, 31],
    barMs: 2600, wave: 'triangle', cutoff: 1400, bassEveryMs: 1300,
    arpSteps: [], arpEveryMs: 0, gain: 0.5,
  },
  // 卡组编辑：Cadd9 - G - Am7 - F，轻快
  deck: {
    chords: [[60, 64, 67, 74], [55, 59, 62, 67], [57, 60, 64, 67], [53, 57, 60, 65]],
    bass: [36, 31, 33, 29],
    barMs: 2400, wave: 'triangle', cutoff: 1600, bassEveryMs: 1200,
    arpSteps: [0, 7, 12], arpEveryMs: 600, gain: 0.45,
  },
  // 决斗：Em - D - C - B 进行，bass 八分脉冲 + 琶音（原版 duel 战斗氛围）
  duel: {
    chords: [[52, 55, 59, 64], [50, 57, 62, 66], [48, 55, 60, 64], [47, 54, 59, 63]],
    bass: [40, 38, 36, 35],
    barMs: 2000, wave: 'sawtooth', cutoff: 900, bassEveryMs: 250,
    arpSteps: [0, 7, 12, 7], arpEveryMs: 250, gain: 0.4,
  },
  // 优势：C - G - Am - F 大调明亮琶音
  advantage: {
    chords: [[60, 64, 67, 72], [55, 59, 62, 67], [57, 60, 64, 69], [53, 57, 60, 65]],
    bass: [36, 31, 33, 29],
    barMs: 1800, wave: 'triangle', cutoff: 2200, bassEveryMs: 450,
    arpSteps: [0, 4, 7, 12, 7, 4], arpEveryMs: 225, gain: 0.5,
  },
  // 劣势：Am - Em - F - E 小调暗沉慢板
  disadvantage: {
    chords: [[57, 60, 64, 69], [52, 55, 59, 64], [53, 57, 60, 65], [52, 56, 59, 64]],
    bass: [33, 28, 29, 28],
    barMs: 2800, wave: 'sawtooth', cutoff: 600, bassEveryMs: 1400,
    arpSteps: [0, 3, 7], arpEveryMs: 700, gain: 0.42,
  },
  // 胜利：C - F - C - G 昂扬上行琶音
  win: {
    chords: [[60, 64, 67, 72], [53, 57, 60, 65], [60, 64, 67, 72], [55, 59, 62, 67]],
    bass: [36, 29, 36, 31],
    barMs: 1600, wave: 'triangle', cutoff: 2400, bassEveryMs: 800,
    arpSteps: [0, 4, 7, 12], arpEveryMs: 200, gain: 0.55,
  },
  // 败北：Am - F - Dm - E 哀缓
  lose: {
    chords: [[57, 60, 64, 69], [53, 57, 60, 65], [50, 53, 57, 62], [52, 56, 59, 64]],
    bass: [33, 29, 26, 28],
    barMs: 3000, wave: 'sine', cutoff: 700, bassEveryMs: 1500,
    arpSteps: [0, 7], arpEveryMs: 1500, gain: 0.5,
  },
};

type RouteName = string;

class BgmManager {
  ctx: AudioContext | null = null;
  master: GainNode | null = null;
  enabled = true;
  volume = 0.5;
  /** 1 = 按场景分曲（原版 chkMusicMode 勾选）；0 = 通用音型 */
  mode = 1;

  /** 路由基础场景（App 路由驱动） */
  private routeScene: BgmScene = 'menu';
  /** 当前是否在决斗/回放画面（决定决斗内场景是否生效） */
  private inDuelScreen = false;
  /** 决斗内场景覆盖（LP 优劣/胜负），null = 无覆盖 */
  private duelOverride: BgmScene | null = null;

  currentScene: BgmScene | null = null;
  private barTimer: ReturnType<typeof setInterval> | null = null;
  private barIdx = 0;
  private liveOsc: { osc: OscillatorNode; until: number }[] = [];

  constructor() {
    // 与 sound_manager 同款手势解锁：浏览器自动播放策略禁止无声期建ctx
    const init = () => {
      this.ensureCtx();
      document.removeEventListener('click', init);
      document.removeEventListener('keydown', init);
    };
    if (typeof document !== 'undefined') {
      document.addEventListener('click', init);
      document.addEventListener('keydown', init);
    }
  }

  ensureCtx(): AudioContext | null {
    if (!this.ctx) {
      const AudioCtx = window.AudioContext || (window as any).webkitAudioContext;
      if (!AudioCtx) return null;
      this.ctx = new AudioCtx();
      this.master = this.ctx.createGain();
      this.master.gain.value = this.volume;
      this.master.connect(this.ctx.destination);
    }
    if (this.ctx.state === 'suspended') this.ctx.resume().catch(() => {});
    return this.ctx;
  }

  setEnabled(on: boolean): void {
    this.enabled = on;
    if (!on) this.stop();
    else this.recompute();
  }

  setVolume(v: number): void {
    this.volume = Math.max(0, Math.min(1, v));
    if (this.master) this.master.gain.value = this.volume;
  }

  setMode(mode: number): void {
    this.mode = mode;
    this.recompute(true);
  }

  /** App 路由 → 基础场景；离开决斗画面时决斗内覆盖随之失效 */
  setRoute(screen: RouteName): void {
    this.routeScene = screen === 'deck' ? 'deck'
      : screen === 'duel' ? 'duel'
      : 'menu';
    this.inDuelScreen = screen === 'duel' || screen === 'replay';
    this.recompute();
  }

  /** 决斗状态（duelStore 快照的子集）→ LP 优劣 / 胜负场景 */
  syncDuel(state: { started: boolean; win: { winner: number } | null; lp: [number, number] }): void {
    if (!state.started) {
      this.duelOverride = null;
    } else if (state.win) {
      this.duelOverride = state.win.winner === 0 ? 'win' : 'lose';
    } else if (state.lp[0] > state.lp[1]) {
      this.duelOverride = 'advantage';
    } else if (state.lp[0] < state.lp[1]) {
      this.duelOverride = 'disadvantage';
    } else {
      this.duelOverride = 'duel';
    }
    this.recompute();
  }

  isPlaying(): boolean {
    return this.barTimer !== null;
  }

  /** 计算当前应有场景并播放；force=true 时同场景也重排（模式切换用） */
  private recompute(force = false): void {
    let scene: BgmScene = this.routeScene;
    if (this.mode === 0) scene = 'duel'; // BGM_ALL：通用音型
    else if (this.inDuelScreen && this.duelOverride) scene = this.duelOverride;
    if (!this.enabled) return;
    this.play(scene, force);
  }

  play(scene: BgmScene, force = false): void {
    if (!this.enabled) return;
    if (this.currentScene === scene && this.isPlaying() && !force) return;
    this.stop();
    const ctx = this.ensureCtx();
    if (!ctx || !this.master) return;
    this.currentScene = scene;
    this.barIdx = 0;
    this.scheduleBar();
    this.barTimer = setInterval(() => this.scheduleBar(), SCENES[scene].barMs);
  }

  stop(): void {
    if (this.barTimer) {
      clearInterval(this.barTimer);
      this.barTimer = null;
    }
    // 快速淡出防爆音
    const now = this.ctx ? this.ctx.currentTime : 0;
    for (const { osc } of this.liveOsc) {
      try {
        osc.stop(now + 0.08);
      } catch { /* 已停止的 osc 再 stop 会抛 InvalidStateError */ }
    }
    this.liveOsc = [];
    this.currentScene = null;
  }

  /** 排一个小节：pad 和弦 + bass 脉冲 + 琶音 */
  private scheduleBar(): void {
    const scene = this.currentScene;
    const ctx = this.ctx;
    if (!scene || !ctx || !this.master) return;
    // 自动播放策略下 ctx 可能仍处 suspended（等用户手势）：跳过本小节，
    // 恢复 running 后由后续小节接续（此时 currentTime 尚未推进，不能排程）
    if (ctx.state !== 'running') return;
    // 清理前面小节已结束的振荡器引用（每小节排程时顺手收割）
    const nowSec = ctx.currentTime;
    this.liveOsc = this.liveOsc.filter((e) => e.until > nowSec);
    const def = SCENES[scene];
    const chord = def.chords[this.barIdx % def.chords.length];
    const root = def.bass[this.barIdx % def.bass.length];
    const t0 = ctx.currentTime + 0.05;
    const filter = ctx.createBiquadFilter();
    filter.type = 'lowpass';
    filter.frequency.value = def.cutoff;
    filter.connect(this.master);

    // pad：每音双振荡微失谐，包络 attack 30% / 持续整小节
    for (const n of chord) {
      for (const detune of [-4, 4]) {
        const osc = ctx.createOscillator();
        osc.type = def.wave;
        osc.frequency.value = nf(n);
        osc.detune.value = detune;
        const g = ctx.createGain();
        g.gain.setValueAtTime(0, t0);
        g.gain.linearRampToValueAtTime(0.05 * def.gain, t0 + def.barMs * 0.3 / 1000);
        g.gain.setValueAtTime(0.05 * def.gain, t0 + (def.barMs - 200) / 1000);
        g.gain.linearRampToValueAtTime(0, t0 + def.barMs / 1000 + 0.05);
        osc.connect(g);
        g.connect(filter);
        osc.start(t0);
        osc.stop(t0 + def.barMs / 1000 + 0.1);
        this.liveOsc.push({ osc, until: t0 + def.barMs / 1000 + 0.2 });
      }
    }

    // bass 脉冲
    if (def.bassEveryMs > 0) {
      const pulses = Math.floor(def.barMs / def.bassEveryMs);
      for (let i = 0; i < pulses; i++) {
        const tt = t0 + (i * def.bassEveryMs) / 1000;
        const osc = ctx.createOscillator();
        osc.type = 'sine';
        osc.frequency.value = nf(root);
        const g = ctx.createGain();
        const peak = 0.12 * def.gain;
        g.gain.setValueAtTime(0, tt);
        g.gain.linearRampToValueAtTime(peak, tt + 0.02);
        g.gain.exponentialRampToValueAtTime(0.001, tt + Math.min(0.22, def.bassEveryMs / 1000));
        osc.connect(g);
        g.connect(this.master);
        osc.start(tt);
        osc.stop(tt + 0.3);
        this.liveOsc.push({ osc, until: t0 + def.barMs / 1000 + 0.2 });
      }
    }

    // 琶音（相对 pad 和弦根音的半音步进）
    if (def.arpSteps.length > 0 && def.arpEveryMs > 0) {
      const notes = Math.floor(def.barMs / def.arpEveryMs);
      for (let i = 0; i < notes; i++) {
        const tt = t0 + (i * def.arpEveryMs) / 1000;
        const step = def.arpSteps[i % def.arpSteps.length];
        const osc = ctx.createOscillator();
        osc.type = 'triangle';
        osc.frequency.value = nf(chord[0] + step + 12);
        const g = ctx.createGain();
        const peak = 0.06 * def.gain;
        g.gain.setValueAtTime(0, tt);
        g.gain.linearRampToValueAtTime(peak, tt + 0.015);
        g.gain.exponentialRampToValueAtTime(0.001, tt + 0.18);
        osc.connect(g);
        g.connect(this.master);
        osc.start(tt);
        osc.stop(tt + 0.22);
        this.liveOsc.push({ osc, until: t0 + def.barMs / 1000 + 0.2 });
      }
    }

    this.barIdx += 1;
  }
}

export const bgmManager = new BgmManager();
