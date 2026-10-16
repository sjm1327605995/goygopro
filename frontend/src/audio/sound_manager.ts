/**
 * Web Audio API Sound Effects Synthesizer & Audio Manager
 * Generates rich, immersive audio effects natively without external audio asset dependencies.
 */

class SoundManager {
  ctx: AudioContext | null = null;
  masterVolume = 0.6;
  sfxVolume = 0.8;
  bgmVolume = 0.4;
  muted = false;

  constructor() {

    this.initAudioContext();
  }

  initAudioContext(): void {
    const init = () => {
      if (!this.ctx) {
        const AudioCtx = window.AudioContext || window.webkitAudioContext;
        if (AudioCtx) {
          this.ctx = new AudioCtx();
        }
      }
      if (this.ctx && this.ctx.state === 'suspended') {
        this.ctx.resume();
      }
      document.removeEventListener('click', init);
      document.removeEventListener('keydown', init);
    };

    if (typeof document !== 'undefined') {
      document.addEventListener('click', init);
      document.addEventListener('keydown', init);
    }
  }

  ensureContext(): void {
    if (!this.ctx) {
      const AudioCtx = window.AudioContext || window.webkitAudioContext;
      if (AudioCtx) {
        this.ctx = new AudioCtx();
      }
    }
    if (this.ctx && this.ctx.state === 'suspended') {
      this.ctx.resume();
    }
  }

  /**
   * Card Draw / Swish Sound
   */
  playDraw() {
    if (this.muted) return;
    this.ensureContext();
    if (!this.ctx) return;

    const t = this.ctx.currentTime;
    const osc = this.ctx.createOscillator();
    const gain = this.ctx.createGain();
    const filter = this.ctx.createBiquadFilter();

    osc.type = 'triangle';
    osc.frequency.setValueAtTime(300, t);
    osc.frequency.exponentialRampToValueAtTime(1200, t + 0.12);

    filter.type = 'lowpass';
    filter.frequency.setValueAtTime(800, t);
    filter.frequency.exponentialRampToValueAtTime(3000, t + 0.12);

    gain.gain.setValueAtTime(0.01, t);
    gain.gain.linearRampToValueAtTime(0.2 * this.sfxVolume, t + 0.04);
    gain.gain.exponentialRampToValueAtTime(0.001, t + 0.15);

    osc.connect(filter);
    filter.connect(gain);
    gain.connect(this.ctx.destination);

    osc.start(t);
    osc.stop(t + 0.16);
  }

  /**
   * Summon Impact Sound (Deep bass punch + cyber sparkle)
   */
  playSummon() {
    if (this.muted) return;
    this.ensureContext();
    if (!this.ctx) return;

    const t = this.ctx.currentTime;

    // Bass impact
    const osc = this.ctx.createOscillator();
    const gain = this.ctx.createGain();
    osc.type = 'sine';
    osc.frequency.setValueAtTime(160, t);
    osc.frequency.exponentialRampToValueAtTime(35, t + 0.35);

    gain.gain.setValueAtTime(0.4 * this.sfxVolume, t);
    gain.gain.exponentialRampToValueAtTime(0.001, t + 0.4);

    osc.connect(gain);
    gain.connect(this.ctx.destination);
    osc.start(t);
    osc.stop(t + 0.4);

    // High shimmer
    const chime = this.ctx.createOscillator();
    const chimeGain = this.ctx.createGain();
    chime.type = 'triangle';
    chime.frequency.setValueAtTime(650, t + 0.05);
    chime.frequency.exponentialRampToValueAtTime(1400, t + 0.25);

    chimeGain.gain.setValueAtTime(0.15 * this.sfxVolume, t + 0.05);
    chimeGain.gain.exponentialRampToValueAtTime(0.001, t + 0.3);

    chime.connect(chimeGain);
    chimeGain.connect(this.ctx.destination);
    chime.start(t + 0.05);
    chime.stop(t + 0.3);
  }

  /**
   * Special Summon / Fusion / Synchro / Xyz Fanfare
   */
  playSpecialSummon() {
    if (this.muted) return;
    this.ensureContext();
    if (!this.ctx) return;

    const notes = [523.25, 659.25, 783.99, 1046.50]; // C5, E5, G5, C6
    const ctx = this.ctx;
    notes.forEach((freq, idx) => {
      const t = ctx!.currentTime + idx * 0.06;
      const osc = ctx!.createOscillator();
      const gain = ctx!.createGain();

      osc.type = 'sine';
      osc.frequency.setValueAtTime(freq, t);

      gain.gain.setValueAtTime(0.2 * this.sfxVolume, t);
      gain.gain.exponentialRampToValueAtTime(0.001, t + 0.28);

      osc.connect(gain);
      gain.connect(ctx!.destination);
      osc.start(t);
      osc.stop(t + 0.3);
    });
    this.playSummon();
  }

  /**
   * Spell Activate (Mystic crystal chime)
   */
  playSpellActivate() {
    if (this.muted) return;
    this.ensureContext();
    if (!this.ctx) return;

    const t = this.ctx.currentTime;
    const osc = this.ctx.createOscillator();
    const gain = this.ctx.createGain();

    osc.type = 'sine';
    osc.frequency.setValueAtTime(880, t);
    osc.frequency.exponentialRampToValueAtTime(1760, t + 0.2);

    gain.gain.setValueAtTime(0.25 * this.sfxVolume, t);
    gain.gain.exponentialRampToValueAtTime(0.001, t + 0.35);

    osc.connect(gain);
    gain.connect(this.ctx.destination);
    osc.start(t);
    osc.stop(t + 0.36);
  }

  /**
   * Trap Activate (Warning alarm zap)
   */
  playTrapActivate() {
    if (this.muted) return;
    this.ensureContext();
    if (!this.ctx) return;

    const t = this.ctx.currentTime;
    const osc = this.ctx.createOscillator();
    const gain = this.ctx.createGain();

    osc.type = 'sawtooth';
    osc.frequency.setValueAtTime(900, t);
    osc.frequency.exponentialRampToValueAtTime(400, t + 0.25);

    gain.gain.setValueAtTime(0.3 * this.sfxVolume, t);
    gain.gain.exponentialRampToValueAtTime(0.001, t + 0.3);

    osc.connect(gain);
    gain.connect(this.ctx.destination);
    osc.start(t);
    osc.stop(t + 0.32);
  }

  /**
   * Attack Rush & Strike Sound
   */
  playAttack() {
    if (this.muted) return;
    this.ensureContext();
    if (!this.ctx) return;

    const t = this.ctx.currentTime;
    // Whoosh
    const osc = this.ctx.createOscillator();
    const gain = this.ctx.createGain();

    osc.type = 'triangle';
    osc.frequency.setValueAtTime(200, t);
    osc.frequency.exponentialRampToValueAtTime(800, t + 0.15);

    gain.gain.setValueAtTime(0.2 * this.sfxVolume, t);
    gain.gain.exponentialRampToValueAtTime(0.001, t + 0.2);

    osc.connect(gain);
    gain.connect(this.ctx.destination);
    osc.start(t);
    osc.stop(t + 0.22);
  }

  /**
   * Damage / Hit Explosion
   */
  playDamage() {
    if (this.muted) return;
    this.ensureContext();
    if (!this.ctx) return;

    const t = this.ctx.currentTime;
    // Low rumble boom
    const osc = this.ctx.createOscillator();
    const gain = this.ctx.createGain();

    osc.type = 'square';
    osc.frequency.setValueAtTime(120, t);
    osc.frequency.exponentialRampToValueAtTime(30, t + 0.3);

    gain.gain.setValueAtTime(0.35 * this.sfxVolume, t);
    gain.gain.exponentialRampToValueAtTime(0.001, t + 0.35);

    osc.connect(gain);
    gain.connect(this.ctx.destination);
    osc.start(t);
    osc.stop(t + 0.36);
  }

  /**
   * Counter Add/Remove — 原版 SOUND_COUNTER_ADD/REMOVE（duelclient.cpp:3390/3410）
   * 的合成等价：放置 = 短促上跳双音；取除 = 下沉单音。
   */
  playCounter(adding: boolean) {
    if (this.muted) return;
    this.ensureContext();
    if (!this.ctx) return;

    const t = this.ctx.currentTime;
    const notes = adding ? [660, 880] : [440, 330];
    notes.forEach((freq, i) => {
      const osc = this.ctx!.createOscillator();
      const gain = this.ctx!.createGain();
      osc.type = 'triangle';
      const start = t + i * 0.06;
      osc.frequency.setValueAtTime(freq, start);
      gain.gain.setValueAtTime(0.0001, start);
      gain.gain.exponentialRampToValueAtTime(0.2 * this.sfxVolume, start + 0.02);
      gain.gain.exponentialRampToValueAtTime(0.001, start + 0.16);
      osc.connect(gain);
      gain.connect(this.ctx!.destination);
      osc.start(start);
      osc.stop(start + 0.18);
    });
  }

  /**
   * LP Recover — 原版 SOUND_RECOVER（duelclient.cpp:3211）的合成等价：
   * 上扬的明亮双音。
   */
  playRecover() {
    if (this.muted) return;
    this.ensureContext();
    if (!this.ctx) return;

    const t = this.ctx.currentTime;
    [523.25, 783.99].forEach((freq, i) => {
      const osc = this.ctx!.createOscillator();
      const gain = this.ctx!.createGain();
      osc.type = 'sine';
      osc.frequency.setValueAtTime(freq, t + i * 0.08);
      gain.gain.setValueAtTime(0.0001, t + i * 0.08);
      gain.gain.exponentialRampToValueAtTime(0.25 * this.sfxVolume, t + i * 0.08 + 0.03);
      gain.gain.exponentialRampToValueAtTime(0.001, t + i * 0.08 + 0.3);
      osc.connect(gain);
      gain.connect(this.ctx!.destination);
      osc.start(t + i * 0.08);
      osc.stop(t + i * 0.08 + 0.32);
    });
  }

  /**
   * Deck Shuffle — 原版 SOUND_SHUFFLE（duelclient.cpp:2263/2345 卡组/额外
   * 洗切）的合成等价：几段短促的噪声沙沙声模拟拨牌。
   */
  playShuffle() {
    if (this.muted) return;
    this.ensureContext();
    if (!this.ctx) return;

    const t0 = this.ctx.currentTime;
    const len = Math.floor(this.ctx.sampleRate * 0.07);
    for (let burst = 0; burst < 5; burst++) {
      const t = t0 + burst * 0.09;
      const buffer = this.ctx.createBuffer(1, len, this.ctx.sampleRate);
      const data = buffer.getChannelData(0);
      for (let i = 0; i < len; i++) {
        data[i] = (Math.random() * 2 - 1) * (1 - i / len);
      }
      const src = this.ctx.createBufferSource();
      src.buffer = buffer;
      const filter = this.ctx.createBiquadFilter();
      filter.type = 'bandpass';
      filter.frequency.value = 2400 + burst * 300;
      const gain = this.ctx.createGain();
      gain.gain.setValueAtTime(0.22 * this.sfxVolume, t);
      gain.gain.exponentialRampToValueAtTime(0.001, t + 0.08);
      src.connect(filter);
      filter.connect(gain);
      gain.connect(this.ctx.destination);
      src.start(t);
    }
  }

  /**
   * Card Destroyed — sharp shatter over a low thud（原版 destroy.wav 的合成等价）
   */
  playDestroy() {
    if (this.muted) return;
    this.ensureContext();
    if (!this.ctx) return;

    const t = this.ctx.currentTime;
    const osc = this.ctx.createOscillator();
    const gain = this.ctx.createGain();

    osc.type = 'sawtooth';
    osc.frequency.setValueAtTime(520, t);
    osc.frequency.exponentialRampToValueAtTime(60, t + 0.28);

    gain.gain.setValueAtTime(0.3 * this.sfxVolume, t);
    gain.gain.exponentialRampToValueAtTime(0.001, t + 0.3);

    osc.connect(gain);
    gain.connect(this.ctx.destination);
    osc.start(t);
    osc.stop(t + 0.31);
  }

  /**
   * LP Tick Counter
   */
  playLPTick() {
    if (this.muted) return;
    this.ensureContext();
    if (!this.ctx) return;
    const t = this.ctx.currentTime;
    const osc = this.ctx.createOscillator();
    const gain = this.ctx.createGain();

    osc.type = 'sine';
    osc.frequency.setValueAtTime(900, t);

    gain.gain.setValueAtTime(0.08 * this.sfxVolume, t);
    gain.gain.exponentialRampToValueAtTime(0.001, t + 0.05);

    osc.connect(gain);
    gain.connect(this.ctx.destination);
    osc.start(t);
    osc.stop(t + 0.06);
  }

  /**
   * Phase Change Chime
   */
  playPhaseChange() {
    if (this.muted) return;
    this.ensureContext();
    if (!this.ctx) return;

    const t = this.ctx.currentTime;
    const osc = this.ctx.createOscillator();
    const gain = this.ctx.createGain();

    osc.type = 'sine';
    osc.frequency.setValueAtTime(587.33, t); // D5
    osc.frequency.setValueAtTime(880.00, t + 0.08); // A5

    gain.gain.setValueAtTime(0.15 * this.sfxVolume, t);
    gain.gain.exponentialRampToValueAtTime(0.001, t + 0.25);

    osc.connect(gain);
    gain.connect(this.ctx.destination);
    osc.start(t);
    osc.stop(t + 0.26);
  }

  /**
   * Chain Link sound
   */
  playChain() {
    if (this.muted) return;
    this.ensureContext();
    if (!this.ctx) return;

    const t = this.ctx.currentTime;
    const osc = this.ctx.createOscillator();
    const gain = this.ctx.createGain();

    osc.type = 'sawtooth';
    osc.frequency.setValueAtTime(440, t);
    osc.frequency.linearRampToValueAtTime(1100, t + 0.18);

    gain.gain.setValueAtTime(0.25 * this.sfxVolume, t);
    gain.gain.exponentialRampToValueAtTime(0.001, t + 0.22);

    osc.connect(gain);
    gain.connect(this.ctx.destination);
    osc.start(t);
    osc.stop(t + 0.25);
  }

  /**
   * Victory Fanfare
   */
  playVictory() {
    if (this.muted) return;
    this.ensureContext();
    if (!this.ctx) return;

    const notes = [523.25, 659.25, 783.99, 1046.50, 1318.51];
    const ctx = this.ctx;
    notes.forEach((f, i) => {
      const t = ctx!.currentTime + i * 0.12;
      const osc = ctx!.createOscillator();
      const gain = ctx!.createGain();
      osc.type = 'triangle';
      osc.frequency.setValueAtTime(f, t);
      gain.gain.setValueAtTime(0.3 * this.sfxVolume, t);
      gain.gain.exponentialRampToValueAtTime(0.001, t + 0.5);
      osc.connect(gain);
      gain.connect(ctx!.destination);
      osc.start(t);
      osc.stop(t + 0.52);
    });
  }

  /**
   * Defeat Sound
   */
  playDefeat() {
    if (this.muted) return;
    this.ensureContext();
    if (!this.ctx) return;

    const notes = [440, 415.3, 392.0, 369.99];
    const ctx = this.ctx;
    notes.forEach((f, i) => {
      const t = ctx!.currentTime + i * 0.2;
      const osc = ctx!.createOscillator();
      const gain = ctx!.createGain();
      osc.type = 'sawtooth';
      osc.frequency.setValueAtTime(f, t);
      gain.gain.setValueAtTime(0.25 * this.sfxVolume, t);
      gain.gain.exponentialRampToValueAtTime(0.001, t + 0.4);
      osc.connect(gain);
      gain.connect(ctx!.destination);
      osc.start(t);
      osc.stop(t + 0.42);
    });
  }

  setVolume(sfxVal?: number, bgmVal?: number) {
    if (sfxVal !== undefined) this.sfxVolume = sfxVal;
    if (bgmVal !== undefined) this.bgmVolume = bgmVal;
  }

  toggleMute() {
    this.muted = !this.muted;
    return this.muted;
  }
}

export const soundManager = new SoundManager();
