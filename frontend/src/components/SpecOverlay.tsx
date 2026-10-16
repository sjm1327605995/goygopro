/**
 * DrawSpec 等价物（原版 drawing.cpp:851 Game::DrawSpec）：屏幕右上
 * 固定位置的 2D 特效卡图动画 + stACMessage 文本弹条 + 猜拳/掷币掷骰动画。
 *
 * 单卡特效队列（事件到达快于动画时排队依次播放；卡图走 WailsBridge.getCardImage）：
 *   duel:summoning / duel:flipsummoning → showcard=7  沿纵向展开的梯形
 *   duel:spsummoning                    → showcard=5  中心放大淡入
 *   duel:chaining / duel:hint(EFFECT/CARD) → showcard=1+2 mask.png 揭示扫过
 *   duel:chain_negated                  → showcard=3  negated.png 收缩罩
 *   duel:card_hint（CHINT_TURN）         → showcard=6  回合数大字盖戳
 *
 * 独立轻量动画（不进队列，原版为独立 showcard 状态）：
 *   duel:hand_res    → showcard=100 双拳下落对碰
 *   duel:toss_coin / duel:toss_dice → ACMessage 文本
 */
import React, { useContext, useEffect, useRef, useState } from 'react';
import { eventBus, WailsBridge } from '../wails_bridge.ts';
import { duelStore } from '../duel/store.ts';
import { sysString } from '../domain/sys_strings.ts';
import { formatRace, formatAttribute } from '../domain/constants.ts';
import { seatToDisplay } from '../domain/reducer.ts';
import { DesignScaleContext } from '../ui/DesignSpace.tsx';
import { rpsOffset, type SpecKind } from '../duel/draw_spec.ts';
import SpecCardAnimation from './SpecCardAnimation.tsx';

interface Spec {
  kind: SpecKind;
  code?: number;
  /** showcard=6 的回合数（CHINT_TURN 的 value） */
  num?: number;
  key: number;
}

// ocgcore common.h HINT_*：MSG_HINT 的 type 字段
const HINT_OPSELECTED = 4;
const HINT_EFFECT = 5;
const HINT_RACE = 6;
const HINT_ATTRIB = 7;
const HINT_CODE = 8;
const HINT_NUMBER = 9;
const HINT_CARD = 10;
// ocgcore CHINT_TURN：MSG_CARD_HINT 的回合数提示（showcard=6 来源）
const CHINT_TURN = 1;

// 出拳值（gframe f1/f2/f3 贴图语义）：1=剪刀 2=石头 3=布
const RPS_HANDS: Record<number, string> = { 1: 'textures/f1.jpg', 2: 'textures/f2.jpg', 3: 'textures/f3.jpg' };

interface RpsShow {
  top: string;
  bottom: string;
  key: number;
}


export default function SpecOverlay() {
  const scale = useContext(DesignScaleContext);
  const [spec, setSpec] = useState<Spec | null>(null);
  const [artUrl, setArtUrl] = useState<string | null>(null);
  const [acText, setAcText] = useState<string | null>(null);
  const [rps, setRps] = useState<RpsShow | null>(null);
  const [rpsFrame, setRpsFrame] = useState(0);
  // 队列 + 播放锁；lastChainCode 供 chain_negated 复用卡图。
  const queue = useRef<Spec[]>([]);
  const playing = useRef(false);
  const lastChainCode = useRef<number | undefined>(undefined);
  const keySeq = useRef(0);
  const timers = useRef<number[]>([]);
  const generation = useRef(0);
  const chainCodes = useRef<number[]>([]);
  const acKey = useRef(0);

  useEffect(() => {
    const later = (fn: () => void, ms: number) => {
      const t = window.setTimeout(fn, ms);
      timers.current.push(t);
    };

    const push = (kind: SpecKind, code?: number, num?: number) => {
      queue.current.push({ kind, code, num, key: ++keySeq.current });
      drain();
    };

    const showAc = (text: string) => {
      const key = ++keySeq.current;
      acKey.current = key;
      setAcText(text);
      later(() => { if (acKey.current === key) setAcText(null); }, 40 * 1000 / 60);
    };

    const onSummoning = (d: any) => push('flip', d.code);
    const onFlipSummoning = (d: any) => push('flip', d.code);
    const onSpSummoning = (d: any) => { if (d.code) push('zoom', d.code); };
    const onChaining = (d: any) => {
      lastChainCode.current = d.code;
      chainCodes.current.push(d.code);
      push('reveal', d.code);
    };
    const onChainNegated = (d: any) => push('negated', chainCodes.current[d.count - 1] || lastChainCode.current);
    const onChainEnd = () => { chainCodes.current = []; lastChainCode.current = undefined; };

    // MSG_HINT：HINT_EFFECT/HINT_CARD 的 data 是卡号 → mask 揭示
    // （原版 duelclient.cpp:1113/1162 的 showcard=1；事件/消息文本走提示条）。
    // HINT_OPSELECTED/RACE/ATTRIB/CODE/NUMBER → stACMessage 浮条
    // （duelclient.cpp:1108-1145：SysString 1510「玩家选择了」/1511「宣言
    // 了」/1512 + 卡名/数字/种族属性名；日志由 reducer 同步落）。
    const fmt151x = (id: number, val: string) =>
      (sysString(id) || '').replace('%ls', val).replace('%d', val);
    const onHint = async (d: any) => {
      const current = generation.current;
      if ((d.type === HINT_EFFECT || d.type === HINT_CARD) && d.data > 0) {
        push('reveal', d.data);
        return;
      }
      switch (d.type) {
        case HINT_OPSELECTED: {
          const text = sysString(d.data) || (await WailsBridge.resolveDesc(d.data));
          if (text && current === generation.current) showAc(fmt151x(1510, text));
          break;
        }
        case HINT_RACE: {
          const race = formatRace(d.data);
          if (race) showAc(fmt151x(1511, race));
          break;
        }
        case HINT_ATTRIB: {
          const attrib = formatAttribute(d.data);
          if (attrib) showAc(fmt151x(1511, attrib));
          break;
        }
        case HINT_CODE: {
          const info = await WailsBridge.getCard(d.data);
          if (current === generation.current) showAc(fmt151x(1511, (info && info.name) || `#${d.data}`));
          break;
        }
        case HINT_NUMBER:
          showAc(fmt151x(1512, String(d.data)));
          break;
      }
    };

    // MSG_CARD_HINT：CHINT_TURN 回合数大字（showcard=6）。载荷只有位置
    // 没有卡号，卡图从 store.board 按 seat→显示座换算取
    const onCardHint = (d: any) => {
      if (d.type !== CHINT_TURN || !d.data || !d.card) return;
      const st = duelStore.getState();
      const disp = seatToDisplay(st, d.card.c);
      const locName = d.card.l === 0x04 ? 'mzone' : d.card.l === 0x08 ? 'szone' : null;
      const code = locName ? (st.board[disp][locName][d.card.s] || {}).code : 0;
      push('number', code || undefined, d.data);
    };

    // ---- showcard=100：猜拳双拳下落对碰 ----
    const onHandRes = (d: any) => {
      const first = RPS_HANDS[d.res & 3] || RPS_HANDS[2];
      const second = RPS_HANDS[(d.res >> 2) & 3] || RPS_HANDS[2];
      const mine = seatToDisplay(duelStore.getState(), 0) === 0 ? first : second;
      const theirs = mine === first ? second : first;
      setRpsFrame(0);
      setRps({ bottom: mine, top: theirs, key: ++keySeq.current });
    };

    // ---- 掷币/掷骰：图标动画 + ACMessage 文本 ----
    const onTossCoin = (d: any) => {
      const faces = (d.results || []).map((r: number) => (r ? sysString(60) : sysString(61)));
      showAc(`投掷硬币结果：${faces.map((face: string) => `[${face}]`).join('')}`);
    };
    const onTossDice = (d: any) => {
      showAc(`投掷骰子结果：${(d.results || []).map((value: number) => `[${value}]`).join('')}`);
    };
    const reset = () => {
      generation.current++;
      timers.current.forEach(clearTimeout);
      timers.current = [];
      queue.current = [];
      playing.current = false;
      setSpec(null); setArtUrl(null); setAcText(null); setRps(null);
      onChainEnd();
    };

    eventBus.on('duel:summoning', onSummoning);
    eventBus.on('duel:flipsummoning', onFlipSummoning);
    eventBus.on('duel:spsummoning', onSpSummoning);
    eventBus.on('duel:chaining', onChaining);
    eventBus.on('duel:chain_negated', onChainNegated);
    eventBus.on('duel:chain_end', onChainEnd);
    eventBus.on('duel:hint', onHint);
    eventBus.on('duel:card_hint', onCardHint);
    eventBus.on('duel:hand_res', onHandRes);
    eventBus.on('duel:toss_coin', onTossCoin);
    eventBus.on('duel:toss_dice', onTossDice);
    eventBus.on('duel:start', reset);
    eventBus.on('duel:win', reset);

    return () => {
      eventBus.off('duel:summoning', onSummoning);
      eventBus.off('duel:flipsummoning', onFlipSummoning);
      eventBus.off('duel:spsummoning', onSpSummoning);
      eventBus.off('duel:chaining', onChaining);
      eventBus.off('duel:chain_negated', onChainNegated);
      eventBus.off('duel:chain_end', onChainEnd);
      eventBus.off('duel:hint', onHint);
      eventBus.off('duel:card_hint', onCardHint);
      eventBus.off('duel:hand_res', onHandRes);
      eventBus.off('duel:toss_coin', onTossCoin);
      eventBus.off('duel:toss_dice', onTossDice);
      eventBus.off('duel:start', reset);
      eventBus.off('duel:win', reset);
      generation.current++;
      timers.current.forEach(clearTimeout);
      timers.current = [];
      queue.current = [];
      playing.current = false;
    };
  }, []);

  useEffect(() => {
    if (!rps) return;
    const started = performance.now();
    let raf = 0;
    const tick = (now: number) => {
      const frame = Math.floor((now - started) * 60 / 1000);
      if (frame >= 60) { setRps(null); return; }
      setRpsFrame(frame);
      raf = requestAnimationFrame(tick);
    };
    raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf);
  }, [rps]);

  const drain = async (): Promise<void> => {
    if (playing.current) return;
    const next = queue.current.shift();
    if (!next) return;
    playing.current = true;
    const current = generation.current;
    let url: string | null = null;
    if (next.code) {
      try { url = (await WailsBridge.getCardImage(next.code))?.url || null; } catch { /* Render available effect textures. */ }
    }
    if (current !== generation.current) return;
    setArtUrl(url);
    setSpec(next);
  };

  return (
    <React.Fragment>
      <div
        id="draw-spec-overlay"
        style={{
          position: 'fixed', left: 0, top: 0, zIndex: 900,
          width: '1024px', height: '640px', pointerEvents: 'none',
          display: spec ? 'block' : 'none',
        }}
      >
        {spec && (
          <div key={spec.key} className={`spec-card spec-${spec.kind}`}>
            <SpecCardAnimation kind={spec.kind} url={artUrl} num={spec.num} onComplete={() => {
              playing.current = false; setSpec(null); void drain();
            }} />
          </div>
        )}
      </div>

      {/* showcard=100：猜拳双拳对碰 */}
      {rps && (
        <div key={rps.key} id="rps-arena" className="rps-arena">
          <img className="rps-hand rps-hand-top" src={rps.top} alt="对方出拳" style={{ left: 659.5, top: rpsOffset(rpsFrame) + 64, transform: `scale(${1 / scale.sx}, ${1 / scale.sy}) translate(-44.5px, -64px)` }} />
          <img className="rps-hand rps-hand-bottom" src={rps.bottom} alt="我方出拳" style={{ left: 659.5, top: 604 - rpsOffset(rpsFrame), transform: `scale(${1 / scale.sx}, ${1 / scale.sy}) translate(-44.5px, -64px)` }} />
        </div>
      )}

      <div
        id="ac-message"
        style={{
          // 原版 wACMessage/stACMessage（game.cpp:527-533）：(490,240)-(840,300)
          // 350×60 无边框、0xc0c0c0ff 浅蓝白底、黑字居中
          position: 'fixed', left: '665px', top: '270px', width: '350px', height: '60px', zIndex: 950,
          transformOrigin: '0 0', transform: `scale(${1 / scale.sx}, ${1 / scale.sy}) translate(-175px, -30px)`,
          display: acText ? 'block' : 'none',
          textAlign: 'center', lineHeight: '58px',
          background: 'rgba(192, 192, 255, 0.75)', color: '#000',
          border: '1px solid #666', fontSize: '12px', pointerEvents: 'none',
          whiteSpace: 'nowrap', boxSizing: 'border-box',
        }}
      >
        <div style={{ width: 310, height: '100%', margin: '0 auto', display: 'flex', alignItems: 'center', justifyContent: 'center', lineHeight: 'normal', whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>{acText || ''}</div>
      </div>
    </React.Fragment>
  );
}
