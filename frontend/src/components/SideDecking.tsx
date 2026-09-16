/**
 * 换副卡组界面（原版 wDeckManager 换侧流程的最小版）。
 *
 * STOC_CHANGE_SIDE 后（三局两胜局间），列出本地卡组；确认后按协议发送
 * CTOS_UPDATE_DECK（mainList = 主卡组+额外，服务端按卡类型自动拆分，
 * 见 core/duel/deck_manager.go LoadDeck / LoadSide），成功后服务端回
 * STOC_DUEL_START 开下一局。发送前先 SetReady(false) 不需要——换侧阶段
 * 服务端用 UpdateDeck 本身作为"再准备"信号（single_duel.go:392-400）。
 */
import React, { useEffect, useState } from 'react';
import { eventBus, WailsBridge } from '../wails_bridge.ts';

export default function SideDecking() {
  const [visible, setVisible] = useState(false);
  const [waiting, setWaiting] = useState(false);
  const [decks, setDecks] = useState<string[]>([]);
  const [picked, setPicked] = useState('');
  const [error, setError] = useState('');

  useEffect(() => {
    const onChangeSide = async () => {
      setVisible(true);
      setWaiting(false);
      setError('');
      try {
        const list = await WailsBridge.listDecks();
        setDecks(list || []);
        setPicked((prev) => prev || (list && list[0]) || '');
      } catch {
        setDecks([]);
      }
    };
    const onDuelStart = () => {
      // 服务端已开下一局（双方都确认了卡组）
      setVisible(false);
    };
    eventBus.on('stoc:change_side', onChangeSide);
    eventBus.on('stoc:duel_start', onDuelStart);
    return () => {
      eventBus.off('stoc:change_side', onChangeSide);
      eventBus.off('stoc:duel_start', onDuelStart);
    };
  }, []);

  if (!visible) return null;

  const confirm = async () => {
    if (!picked) return;
    try {
      const deck = await WailsBridge.loadDeck(picked);
      if (!deck) { setError('卡组加载失败'); return; }
      setWaiting(true);
      WailsBridge.updateDeck([...deck.main, ...deck.extra], deck.side);
    } catch {
      setError('卡组加载失败');
      setWaiting(false);
    }
  };

  return (
    <div id="side-decking" className="side-decking">
      <div className="side-decking-box">
        <div className="side-decking-title">换副卡组</div>
        <div className="side-decking-hint">选择下一局使用的卡组（主+副需与上局数量一致）</div>
        <select
          id="side-deck-select"
          className="form-select"
          value={picked}
          onChange={(e) => setPicked(e.target.value)}
        >
          {decks.map((d) => <option key={d} value={d}>{d}</option>)}
        </select>
        {error && <div className="side-decking-error">{error}</div>}
        <button id="side-deck-confirm" className="btn btn-gold" disabled={waiting || !picked} onClick={confirm}>
          {waiting ? '等待更换副卡组中...' : '副卡组更换完成'}
        </button>
      </div>
    </div>
  );
}
