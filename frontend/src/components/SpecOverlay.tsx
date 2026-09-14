/**
 * DrawSpec 等价物（原版 drawing.cpp:851 Game::DrawSpec）：屏幕右上
 * 固定位置的 2D 特效卡图动画 + stACMessage 文本弹条。
 *
 * 触发映射（与 vendored duelclient.cpp 一致）：
 *   duel:summoning / duel:flipsummoning → showcard=7  翻卡（rotateY 半周）
 *   duel:spsummoning                    → showcard=5  中心放大淡入
 *   duel:chaining                       → showcard=1+2 mask.png 揭示扫过
 *   duel:chain_negated                  → showcard=3  negated.png 收缩罩（盖在最后一张连锁卡上）
 *   duel:toss_coin / duel:toss_dice     → ACMessage 文本弹条（duelclient.cpp:3538/3561，正面/反面 + 点数）
 *
 * 事件到达快于动画时排队依次播放；卡片插画走 WailsBridge.getCardImage。
 */
import React, { useEffect, useRef, useState } from 'react';
import { eventBus, WailsBridge } from '../wails_bridge.ts';

type SpecKind = 'flip' | 'zoom' | 'reveal' | 'negated';

interface Spec {
  kind: SpecKind;
  code?: number;
  key: number;
}

const SPEC_MS: Record<SpecKind, number> = { flip: 800, zoom: 500, reveal: 900, negated: 600 };

export default function SpecOverlay() {
  const [spec, setSpec] = useState<Spec | null>(null);
  const [artUrl, setArtUrl] = useState<string | null>(null);
  const [acText, setAcText] = useState<string | null>(null);
  // 队列 + 播放锁；lastChainCode 供 chain_negated 复用卡图。
  const queue = useRef<Spec[]>([]);
  const playing = useRef(false);
  const lastChainCode = useRef<number | undefined>(undefined);
  const keySeq = useRef(0);
  const timers = useRef<number[]>([]);

  useEffect(() => {
    const push = (kind: SpecKind, code?: number) => {
      if (queue.current.length > 4) return; // 防洪：积压时丢多余的特效
      queue.current.push({ kind, code, key: ++keySeq.current });
      drain();
    };

    const showAc = (text: string) => {
      setAcText(text);
      const t = window.setTimeout(() => setAcText(null), 1400);
      timers.current.push(t);
    };

    const onSummoning = (d: any) => push('flip', d.code);
    const onFlipSummoning = (d: any) => push('flip', d.code);
    const onSpSummoning = (d: any) => push('zoom', d.code);
    const onChaining = (d: any) => {
      lastChainCode.current = d.code;
      push('reveal', d.code);
    };
    const onChainNegated = () => push('negated', lastChainCode.current);
    const onTossCoin = (d: any) => {
      const faces = (d.results || []).map((r: number) => (r ? '正面' : '反面'));
      showAc(`硬币抛掷结果：[ ${faces.join(' ] [ ')} ]`);
    };
    const onTossDice = (d: any) => {
      const rolls = (d.results || []).join(' ] [ ');
      showAc(`骰子点数：[ ${rolls} ]`);
    };

    eventBus.on('duel:summoning', onSummoning);
    eventBus.on('duel:flipsummoning', onFlipSummoning);
    eventBus.on('duel:spsummoning', onSpSummoning);
    eventBus.on('duel:chaining', onChaining);
    eventBus.on('duel:chain_negated', onChainNegated);
    eventBus.on('duel:toss_coin', onTossCoin);
    eventBus.on('duel:toss_dice', onTossDice);

    return () => {
      eventBus.off('duel:summoning', onSummoning);
      eventBus.off('duel:flipsummoning', onFlipSummoning);
      eventBus.off('duel:spsummoning', onSpSummoning);
      eventBus.off('duel:chaining', onChaining);
      eventBus.off('duel:chain_negated', onChainNegated);
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
              ? <img src={artUrl} alt="" draggable={false} style={{ width: '100%', height: '100%', objectFit: 'cover', display: 'block' }} />
              : <div style={{ width: '100%', height: '100%', background: '#0f172a', border: '1px solid #94a3b8' }} />}
            {spec.kind === 'reveal' && <div className="spec-mask" style={{ backgroundImage: "url('textures/mask.png')" }} />}
            {spec.kind === 'negated' && <div className="spec-negated" style={{ backgroundImage: "url('textures/negated.png')" }} />}
          </div>
        )}
      </div>
      <div
        id="ac-message"
        style={{
          position: 'absolute', left: '50%', top: '38%', transform: 'translateX(-50%)', zIndex: 950,
          display: acText ? 'block' : 'none',
          background: 'rgba(10,12,20,0.85)', color: '#fff', border: '1px solid var(--border-cyan)',
          padding: '10px 22px', borderRadius: '6px', fontSize: '14px', pointerEvents: 'none',
          whiteSpace: 'nowrap',
        }}
      >
        {acText || ''}
      </div>
    </React.Fragment>
  );
}
