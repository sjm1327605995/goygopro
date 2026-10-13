/**
 * DrawSpec 等价物（原版 drawing.cpp:851 Game::DrawSpec）：屏幕右上
 * 固定位置的 2D 特效卡图动画 + stACMessage 文本弹条 + 猜拳/掷币掷骰动画。
 *
 * 单卡特效队列（事件到达快于动画时排队依次播放；卡图走 WailsBridge.getCardImage）：
 *   duel:summoning / duel:flipsummoning → showcard=7  翻卡（rotateY 半周）
 *   duel:spsummoning                    → showcard=5  中心放大淡入
 *   duel:chaining / duel:hint(EFFECT/CARD) → showcard=1+2 mask.png 揭示扫过
 *   duel:chain_negated                  → showcard=3  negated.png 收缩罩
 *   duel:confirm_cards（单卡）           → showcard=4  淡入揭示
 *   duel:card_hint（CHINT_TURN）         → showcard=6  回合数大字盖戳
 *
 * 独立轻量动画（不进队列，原版为独立 showcard 状态）：
 *   duel:hand_res    → showcard=100 双拳下落对碰
 *   duel:toss_coin   → 硬币翻转 + ACMessage 文本（duelclient.cpp:3538）
 *   duel:toss_dice   → 骰子弹跳 + ACMessage 文本（duelclient.cpp:3561）
 */
import React, { useEffect, useRef, useState } from 'react';
import { eventBus, WailsBridge } from '../wails_bridge.ts';
import { duelStore } from '../duel/store.ts';
import { sysString } from '../domain/sys_strings.ts';
import { formatRace, formatAttribute } from '../domain/constants.ts';

type SpecKind = 'flip' | 'zoom' | 'reveal' | 'negated' | 'fade' | 'number';

interface Spec {
  kind: SpecKind;
  code?: number;
  /** showcard=6 的回合数（CHINT_TURN 的 value） */
  num?: number;
  key: number;
}

const SPEC_MS: Record<SpecKind, number> = {
  flip: 800, zoom: 500, reveal: 900, negated: 600, fade: 700, number: 900,
};

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
  const [spec, setSpec] = useState<Spec | null>(null);
  const [artUrl, setArtUrl] = useState<string | null>(null);
  const [acText, setAcText] = useState<string | null>(null);
  const [rps, setRps] = useState<RpsShow | null>(null);
  // 队列 + 播放锁；lastChainCode 供 chain_negated 复用卡图。
  const queue = useRef<Spec[]>([]);
  const playing = useRef(false);
  const lastChainCode = useRef<number | undefined>(undefined);
  const keySeq = useRef(0);
  const timers = useRef<number[]>([]);

  useEffect(() => {
    const later = (fn: () => void, ms: number) => {
      const t = window.setTimeout(fn, ms);
      timers.current.push(t);
    };

    const push = (kind: SpecKind, code?: number, num?: number) => {
      if (queue.current.length > 4) return; // 防洪：积压时丢多余的特效
      queue.current.push({ kind, code, num, key: ++keySeq.current });
      drain();
    };

    const showAc = (text: string) => {
      setAcText(text);
      later(() => setAcText(null), 1400);
    };

    const onSummoning = (d: any) => push('flip', d.code);
    const onFlipSummoning = (d: any) => push('flip', d.code);
    const onSpSummoning = (d: any) => push('zoom', d.code);
    const onChaining = (d: any) => {
      lastChainCode.current = d.code;
      push('reveal', d.code);
    };
    const onChainNegated = () => push('negated', lastChainCode.current);

    // MSG_HINT：HINT_EFFECT/HINT_CARD 的 data 是卡号 → mask 揭示
    // （原版 duelclient.cpp:1113/1162 的 showcard=1；事件/消息文本走提示条）。
    // HINT_OPSELECTED/RACE/ATTRIB/CODE/NUMBER → stACMessage 浮条
    // （duelclient.cpp:1108-1145：SysString 1510「玩家选择了」/1511「宣言
    // 了」/1512 + 卡名/数字/种族属性名；日志由 reducer 同步落）。
    const fmt151x = (id: number, val: string) =>
      (sysString(id) || '').replace('[%ls]', val).replace('[%d]', val);
    const onHint = async (d: any) => {
      if ((d.type === HINT_EFFECT || d.type === HINT_CARD) && d.data > 0) {
        push('reveal', d.data);
        return;
      }
      switch (d.type) {
        case HINT_OPSELECTED: {
          const text = sysString(d.data) || (await WailsBridge.resolveDesc(d.data));
          if (text) showAc(fmt151x(1510, text));
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
          showAc(fmt151x(1511, (info && info.name) || `#${d.data}`));
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
      const disp = d.card.c === st.playerSlot ? 0 : 1;
      const locName = d.card.l === 0x04 ? 'mzone' : d.card.l === 0x08 ? 'szone' : null;
      const code = locName ? (st.board[disp][locName][d.card.s] || {}).code : 0;
      push('number', code || undefined, d.data);
    };

    // MSG_CONFIRM_CARDS：单卡确认 → 淡入揭示（多卡走 field3d 揭示序列+日志）
    const onConfirmCards = (d: any) => {
      const cards = d.cards || [];
      if (cards.length === 1 && cards[0].code) push('fade', cards[0].code);
    };

    // ---- showcard=100：猜拳双拳下落对碰 ----
    const onHandRes = (d: any) => {
      const mine = RPS_HANDS[d.res & 3] || RPS_HANDS[2];
      const theirs = RPS_HANDS[(d.res >> 2) & 3] || RPS_HANDS[2];
      // 下拳=自己（低 2 位），上拳=对方（高 2 位）
      setRps({ bottom: mine, top: theirs, key: ++keySeq.current });
      later(() => setRps(null), 1700);
    };

    // ---- 掷币/掷骰：图标动画 + ACMessage 文本 ----
    const onTossCoin = (d: any) => {
      const faces = (d.results || []).map((r: number) => (r ? sysString(60) : sysString(61)));
      showAc(`投掷硬币结果：[ ${faces.join(' ] [ ')} ]`);
    };
    const onTossDice = (d: any) => {
      const rolls = (d.results || []).join(' ] [ ');
      showAc(`投掷骰子结果：[ ${rolls} ]`);
    };

    eventBus.on('duel:summoning', onSummoning);
    eventBus.on('duel:flipsummoning', onFlipSummoning);
    eventBus.on('duel:spsummoning', onSpSummoning);
    eventBus.on('duel:chaining', onChaining);
    eventBus.on('duel:chain_negated', onChainNegated);
    eventBus.on('duel:hint', onHint);
    eventBus.on('duel:card_hint', onCardHint);
    eventBus.on('duel:confirm_cards', onConfirmCards);
    eventBus.on('duel:hand_res', onHandRes);
    eventBus.on('duel:toss_coin', onTossCoin);
    eventBus.on('duel:toss_dice', onTossDice);

    return () => {
      eventBus.off('duel:summoning', onSummoning);
      eventBus.off('duel:flipsummoning', onFlipSummoning);
      eventBus.off('duel:spsummoning', onSpSummoning);
      eventBus.off('duel:chaining', onChaining);
      eventBus.off('duel:chain_negated', onChainNegated);
      eventBus.off('duel:hint', onHint);
      eventBus.off('duel:card_hint', onCardHint);
      eventBus.off('duel:confirm_cards', onConfirmCards);
      eventBus.off('duel:hand_res', onHandRes);
      eventBus.off('duel:toss_coin', onTossCoin);
      eventBus.off('duel:toss_dice', onTossDice);
      timers.current.forEach(clearTimeout);
      timers.current = [];
      queue.current = [];
      playing.current = false;
    };
  }, []);

  const drain = (): void => {
    if (playing.current) return;
    const next = queue.current.shift();
    if (!next) return;
    playing.current = true;
    setSpec(next);
    if (next.code) {
      // 位置无关：拿到插画再播放也来得及（动画本身有 ~1s 时长）。
      WailsBridge.getCardImage(next.code).then((p) => {
        setArtUrl(p ? p.url : null);
      });
    }
    const t = window.setTimeout(() => {
      playing.current = false;
      setSpec(null);
      drain();
    }, SPEC_MS[next.kind]);
    timers.current.push(t);
  };

  return (
    <React.Fragment>
      <div
        id="draw-spec-overlay"
        style={{
          position: 'absolute', left: '56%', top: '23%', zIndex: 900,
          width: '177px', height: '254px', pointerEvents: 'none',
          display: spec ? 'block' : 'none',
        }}
      >
        {spec && (
          <div key={spec.key} className={`spec-card spec-${spec.kind}`}>
            {artUrl
              ? <img src={artUrl} alt="" draggable={false} style={{ width: '100%', height: '100%', objectFit: 'contain', display: 'block' }} />
              : <div style={{ width: '100%', height: '100%', background: '#0f172a', border: '1px solid #94a3b8' }} />}
            {spec.kind === 'reveal' && <div className="spec-mask" style={{ backgroundImage: "url('textures/mask.png')" }} />}
            {spec.kind === 'negated' && <div className="spec-negated" style={{ backgroundImage: "url('textures/negated.png')" }} />}
            {spec.kind === 'number' && <div className="spec-number"><span>{spec.num}</span></div>}
          </div>
        )}
      </div>

      {/* showcard=100：猜拳双拳对碰 */}
      {rps && (
        <div key={rps.key} id="rps-arena" className="rps-arena">
          <img className="rps-hand rps-hand-top" src={rps.top} alt="对方出拳" />
          <img className="rps-hand rps-hand-bottom" src={rps.bottom} alt="我方出拳" />
        </div>
      )}

      <div
        id="ac-message"
        style={{
          // 原版 wACMessage/stACMessage（game.cpp:527-533）：(490,240)-(840,300)
          // 350×60 无边框、0xc0c0c0ff 浅蓝白底、黑字居中
          position: 'absolute', left: '490px', top: '240px', width: '350px', height: '60px', zIndex: 950,
          display: acText ? 'block' : 'none',
          textAlign: 'center', lineHeight: '58px',
          background: 'rgba(192, 192, 255, 0.75)', color: '#000',
          border: '1px solid #666', fontSize: '14px', pointerEvents: 'none',
          whiteSpace: 'nowrap', boxSizing: 'border-box',
        }}
      >
        {acText || ''}
      </div>
    </React.Fragment>
  );
}
