/**
 * MSG_BATTLE 攻防对撞浮层（原版 duelclient.cpp:3480-3519 的强化版：
 * 原版只把结算后的 ATK/DEF 刷到卡片角标上，这里额外做数值对撞展示）。
 *
 * 触发：duel:battle（26 字节结算体：双方位置 + ATK/DEF + ocgcore 战破
 * 旗标 attackerDestroyed/targetDestroyed = processor.cpp 的 bd[0]/bd[1]）。
 * 表现：屏幕中央双方攻守数值从两侧对撞 → 结果行（战斗破坏 / 直接攻击）
 * → 2.4s 后整体淡出。卡名从 store.board 按引擎 seat→显示座换算后解析。
 */
import React, { useEffect, useRef, useState } from 'react';
import { eventBus, WailsBridge } from '../wails_bridge.ts';
import { duelStore } from '../duel/store.ts';
import { LOC_NAMES } from '../domain/constants.ts';

interface BattleInfo {
  key: number;
  attacker: { c: number; l: number; s: number };
  attackerATK: number;
  attackerDEF: number;
  attackerDestroyed: boolean;
  target: { c: number; l: number; s: number } | null;
  targetATK: number;
  targetDEF: number;
  targetDestroyed: boolean;
  aName?: string;
  tName?: string;
}

const DISPLAY_MS = 2400;

export default function BattleOverlay() {
  const [battle, setBattle] = useState<BattleInfo | null>(null);
  const keySeq = useRef(0);

  useEffect(() => {
    let timer = 0;
    const onBattle = (d: any): void => {
      // target.l === 0 表示直接攻击（ocgcore 写全 0，processor.cpp:2989-2992）
      const direct = !d.target || !d.target.l;
      const key = ++keySeq.current;
      setBattle({
        key,
        attacker: d.attacker,
        attackerATK: d.attackerATK,
        attackerDEF: d.attackerDEF,
        attackerDestroyed: !!d.attackerDestroyed,
        target: direct ? null : d.target,
        targetATK: d.targetATK,
        targetDEF: d.targetDEF,
        targetDestroyed: !!d.targetDestroyed,
      });
      // 卡名异步解析（board 是显示座，事件的 c 是引擎 seat）
      const st = duelStore.getState();
      const codeOf = (loc: { c: number; l: number; s: number }): number => {
        const disp = loc.c === st.playerSlot ? 0 : 1;
        const zone = st.board[disp] && (st.board[disp] as any)[LOC_NAMES[loc.l]];
        const card = Array.isArray(zone) ? zone[loc.s] : null;
        return (card && card.code) || 0;
      };
      const aCode = codeOf(d.attacker);
      const tCode = direct ? 0 : codeOf(d.target);
      Promise.all([
        aCode ? WailsBridge.getCard(aCode) : null,
        tCode ? WailsBridge.getCard(tCode) : null,
      ]).then(([aInfo, tInfo]) => {
        setBattle((cur) => (cur && cur.key === key
          ? { ...cur, aName: (aInfo && aInfo.name) || undefined, tName: (tInfo && tInfo.name) || undefined }
          : cur));
      });
      clearTimeout(timer);
      timer = window.setTimeout(() => setBattle((cur) => (cur && cur.key === key ? null : cur)), DISPLAY_MS);
    };
    eventBus.on('duel:battle', onBattle);
    return () => {
      eventBus.off('duel:battle', onBattle);
      clearTimeout(timer);
    };
  }, []);

  if (!battle) return null;

  const direct = !battle.target;
  const results: string[] = [];
  if (direct) {
    results.push('直接攻击！');
  } else {
    if (battle.targetDestroyed) results.push('防守怪兽被战斗破坏');
    if (battle.attackerDestroyed) results.push('攻击怪兽被战斗破坏');
  }

  return (
    <div id="battle-overlay" className="battle-overlay" key={battle.key}>
      <div className="battle-side battle-side-a">
        <div className="battle-name">{battle.aName || '攻击怪兽'}</div>
        <div className="battle-stat">
          <span className="battle-atk">{battle.attackerATK}</span>
          <span className="battle-def">{battle.attackerDEF}</span>
        </div>
      </div>
      <div className="battle-clash">
        <span className="battle-vs">VS</span>
        {results.length > 0 && <div className="battle-result">{results.join('，')}</div>}
      </div>
      <div className="battle-side battle-side-d">
        {direct ? (
          <div className="battle-name">玩家直接攻击</div>
        ) : (
          <>
            <div className="battle-name">{battle.tName || '防守怪兽'}</div>
            <div className="battle-stat">
              <span className="battle-atk">{battle.targetATK}</span>
              <span className="battle-def">{battle.targetDEF}</span>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
