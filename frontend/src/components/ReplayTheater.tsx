import React, { useEffect, useRef, useState } from 'react';
import DuelStage from './DuelStage.tsx';
import type { StageHandle } from './DuelStage.tsx';
import { WailsBridge, eventBus } from '../wails_bridge.ts';
import { duelStore } from '../duel/store.ts';
import GfwSelect from './ui/GfwSelect.tsx';
import UiButton from '../ui/UiButton.tsx';
import { UiInput } from '../ui/UiInput.tsx';
import DesignSpace from '../ui/DesignSpace.tsx';

/**
 * Replay Theater: lists recorded .yrp duels, asks the Go backend to replay one
 * (ocgcore regenerates the duel deterministically from the replay's seed), and
 * steps through the returned engine events — through the SAME event bus the
 * live duel uses. The store (reducer.ts) rebuilds 2D state, DuelManager
 * animates the 3D board; there is no separate replay code path anymore
 * (replay_apply.js is gone).
 *
 * Seek/step-back = duelStore.reset() + field3D.clearBoard() + re-emitting the
 * prefix with the __instant flag (tweens snap instead of tweening). Animated
 * playback emits plain events — tween order is driven by the scheduler below.
 */
/** 回放事件流条目（Go playReplay 返回的 { type, data } 序列） */
interface ReplayEvent {
  type: string;
  data?: any;
}

interface ReplayTheaterProps {
  onNavigate: (screen: string) => void;
}

export default function ReplayTheater({ onNavigate }: ReplayTheaterProps) {
  const [replays, setReplays] = useState<string[]>([]);
  const [selected, setSelected] = useState('');
  const [meta, setMeta] = useState<any>(null);
  const [info, setInfo] = useState<any>(null);
  const [startTurn, setStartTurn] = useState('1');
  const [toast, setToast] = useState('');
  const [events, setEvents] = useState<ReplayEvent[]>([]);
  const [currentStep, setCurrentStep] = useState(0);
  const [isPlaying, setIsPlaying] = useState(false);
  const [speed, setSpeed] = useState(1);
  const [truncated, setTruncated] = useState(false);
  const [stageReady, setStageReady] = useState(false);
  // Bumped whenever DuelStage hands us a fresh stage, so the reset effect
  // also re-runs when the stage remounts under an already-loaded event
  // stream (e.g. navigating away and back).
  const [stageVersion, setStageVersion] = useState(0);

  const timerRef = useRef<number | null>(null);
  // 动画推进轮询读真值，避免闭包里的旧 isPlaying/speed state 造成误判
  const isPlayingRef = useRef(false);
  const playSpeedRef = useRef(1);
  const toastTimerRef = useRef<number | null>(null);
  const stageRef = useRef<StageHandle | null>(null); // { field3D, duelManager } from DuelStage
  const stepRef = useRef(0);              // mirror of currentStep for the play timer
  const eventsRef = useRef<ReplayEvent[]>([]);
  // 载入时要从第几回合开始看（原版 ebRepStartTurn → skip_turn）。
  // reset effect 消费一次：第 1..N-1 回合的事件瞬时应用后停在那里。
  const pendingTurnRef = useRef(0);

  useEffect(() => {
    (async () => {
      const names = await WailsBridge.listReplays();
      setReplays(names || []);
    })();
    return () => { if (timerRef.current) window.clearTimeout(timerRef.current); };
  }, []);

  const refreshList = async (): Promise<void> => {
    const names = await WailsBridge.listReplays();
    setReplays(names || []);
  };

  // 列表选中（原版 EGET_LISTBOX_CHANGED / LISTBOX_REPLAY_LIST，
  // menu_handler.cpp:519-559）：填充录像信息面板，起始回合重置为 1。
  const selectReplay = async (name: string): Promise<void> => {
    setSelected(name);
    setStartTurn('1');
    setMeta(null);
    if (!name) { setInfo(null); return; }
    const res = await WailsBridge.replayInfo(name);
    setInfo(res.success ? res : null);
  };

  const deleteReplay = async (): Promise<void> => {
    if (!selected) { alert('请先选择要删除的回放。'); return; }
    if (!window.confirm(`确定删除回放「${selected}」？此操作不可撤销。`)) return;
    await WailsBridge.deleteReplay(selected);
    setSelected('');
    setInfo(null);
    await refreshList();
  };

  const renameReplay = async (): Promise<void> => {
    if (!selected) { alert('请先选择要重命名的回放。'); return; }
    const newName = window.prompt('新的回放名称：', selected.replace(/\.yrp$/i, ''));
    if (!newName || !newName.trim() || newName.trim() === selected.replace(/\.yrp$/i, '')) return;
    await WailsBridge.renameReplay(selected, newName.trim());
    setSelected('');
    setInfo(null);
    await refreshList();
  };

  // 提取卡组（BUTTON_EXPORT_DECK，SysString 1369）：把录像里双方的卡组
  // 各存一份 .ydk 到 deck 目录，成功后弹「保存成功」（1335）。
  const exportDeck = async (): Promise<void> => {
    if (!selected) { alert('请先选择要提取卡组的录像。'); return; }
    const res = await WailsBridge.exportReplayDeck(selected);
    if (res.success) {
      setToast('保存成功');
      if (toastTimerRef.current) clearTimeout(toastTimerRef.current);
      toastTimerRef.current = window.setTimeout(() => setToast(''), 2000);
    } else {
      alert(res.error || '提取卡组失败');
    }
  };

  // Reset board + store whenever a loaded stream meets a (re)mounted stage.
  useEffect(() => {
    const stage = stageRef.current;
    if (!stage || !events.length) return undefined;
    duelStore.reset();
    stage.field3D.clearBoard();
    stepRef.current = 0;
    setCurrentStep(0);
    // 播放起始于回合（原版 skip_turn 语义，replay_mode.cpp:504-512）：
    // 起始回合为 N 时，第 1..N-1 回合的事件全部瞬时应用，画面停在回合 N 开始。
    const n = pendingTurnRef.current;
    pendingTurnRef.current = 0;
    if (n > 1) {
      let target = events.length; // 回合数不足 → 等价原版一路 skip 到终局
      let seen = 0;
      for (let i = 0; i < events.length; i++) {
        if (events[i].type === 'duel:new_turn') {
          seen++;
          if (seen === n) { target = i; break; }
        }
      }
      for (let i = 0; i < target; i++) emitEvent(events[i], true);
      stepRef.current = target;
      setCurrentStep(target);
    }
    setStageReady(true);
    return undefined;
  }, [events, stageVersion]);

  const setStep = (step: number): void => {
    stepRef.current = step;
    setCurrentStep(step);
  };

  // Replays one event through the shared event bus. `instant` marks seek
  // re-applies: the flag rides on the payload (reducer ignores unknown
  // fields) and DuelManager snaps placements instead of tweening.
  const emitEvent = (evt: ReplayEvent | undefined, instant: boolean): void => {
    if (!evt || !evt.type) return;
    const data = instant ? { ...(evt.data || {}), __instant: true } : (evt.data || {});
    eventBus.emit(evt.type, data);
  };

  const loadReplay = async (): Promise<void> => {
    if (!selected) return;
    // 原版载入时读 ebRepStartTurn（menu_handler.cpp:241）：1 起步，负值/空值兜底
    const n = Math.floor(Number(startTurn));
    pendingTurnRef.current = Number.isFinite(n) && n > 1 ? n : 0;
    const res = await WailsBridge.playReplay(selected);
    if (!res || !res.success) {
      alert(res && res.error ? res.error : '加载回放失败');
      return;
    }
    pause();
    setStageReady(false);
    setMeta({ name: res.name || selected, players: res.players || [], startLp: res.startLp, duelRule: res.duelRule });
    eventsRef.current = res.events || [];
    setEvents(res.events || []);
    setTruncated(!!res.truncated);
    setStep(0);
  };

  const stepForward = () => {
    if (stepRef.current < eventsRef.current.length) {
      emitEvent(eventsRef.current[stepRef.current], false);
      setStep(stepRef.current + 1);
    } else {
      pause();
    }
  };

  const seek = (target: number): void => {
    const stage = stageRef.current;
    const t = Math.max(0, Math.min(target, eventsRef.current.length));
    if (!stage) return;
    duelStore.reset();
    stage.field3D.clearBoard();
    for (let i = 0; i < t; i++) {
      emitEvent(eventsRef.current[i], true);
    }
    setStep(t);
  };

  const stepBack = () => {
    if (stepRef.current === 0) return;
    seek(stepRef.current - 1);
  };

  // Click on the progress bar to jump; the board is rebuilt instantly.
  const seekToRatio = (ratio: number): void => {
    if (!eventsRef.current.length) return;
    seek(Math.round(ratio * eventsRef.current.length));
  };

  // 动画驱动的推进：emit 一个事件后，等它触发的 3D tween 播完再进下一步
  // （代替固定 1 秒时钟——动作动画 ~300ms 就结束，一刀切时钟会让动作一闪
  // 而过、流程割裂）。速度档作 timeScale；minGap 保证纯数据事件（无动画）
  // 不瞬间连发，maxAnim 是动画等待上限（tween 卡死时兜底推进）。
  const MIN_GAP_MS = 350;
  const MAX_ANIM_MS = 2500;
  const advance = (): void => {
    if (!isPlayingRef.current) return;
    if (stepRef.current >= eventsRef.current.length) {
      isPlayingRef.current = false;
      setIsPlaying(false);
      timerRef.current = null;
      return;
    }
    emitEvent(eventsRef.current[stepRef.current], false);
    setStep(stepRef.current + 1);
    waitThenAdvance(Date.now());
  };
  const waitThenAdvance = (started: number): void => {
    const sp = playSpeedRef.current || 1;
    const minGap = Math.max(120, Math.round(MIN_GAP_MS / sp));
    const maxAnim = Math.round(MAX_ANIM_MS / sp);
    const poll = (): void => {
      if (!isPlayingRef.current) return; // 暂停/变速时被 timer 清空打断
      const field = stageRef.current;
      const animating = !!(field && field.field3D.hasActiveTweens());
      const elapsed = Date.now() - started;
      if (animating && elapsed < maxAnim) {
        timerRef.current = window.setTimeout(poll, 16);
      } else if (elapsed < minGap) {
        timerRef.current = window.setTimeout(poll, 16);
      } else {
        advance();
      }
    };
    timerRef.current = window.setTimeout(poll, 16);
  };

  const play = (speedOverride?: number): void => {
    if (!stageReady) return;
    // Accept an explicit speed: changeSpeed() restarts the timer while the
    // state update from setSpeed() has not landed in this render's closure.
    playSpeedRef.current = speedOverride ?? speed;
    isPlayingRef.current = true;
    setIsPlaying(true);
    if (timerRef.current) { window.clearTimeout(timerRef.current); timerRef.current = null; }
    advance();
  };

  const pause = (): void => {
    isPlayingRef.current = false;
    setIsPlaying(false);
    if (timerRef.current) { window.clearTimeout(timerRef.current); timerRef.current = null; }
  };

  const togglePlay = (): void => { if (isPlaying) pause(); else play(); };

  const changeSpeed = (v: number): void => {
    setSpeed(v);
    if (isPlaying) { pause(); play(v); }
  };

  const handleStageReady = (stage: StageHandle | null): void => {
    stageRef.current = stage;
    if (!stage) {
      setStageReady(false);
    } else {
      setStageVersion((v) => v + 1);
    }
  };

  const progressPct = events.length ? (currentStep / events.length) * 100 : 0;
  const playersText = meta && meta.players.length ? meta.players.join(' vs ') : '— vs —';
  // 录像信息面板文本（原版 stReplayInfo：日期/脚本名/===VS=== 布局）
  const infoText = (i: any): string => {
    const lines: string[] = [];
    if (i.date) lines.push(i.date);
    if (i.isSingle && i.script) lines.push(i.script);
    const ps = i.players || [];
    if (i.isTag && ps.length >= 4) {
      lines.push(`${ps[0]}\n${ps[1]}\n===VS===\n${ps[2]}\n${ps[3]}`);
    } else if (ps.length >= 2) {
      lines.push(`${ps[0]}\n===VS===\n${ps[1]}`);
    } else if (ps.length) {
      lines.push(ps.join('\n'));
    }
    return lines.join('\n');
  };

  return (
    // 原版 wReplay（game.cpp:831-845）：窗口 (220,100)-(800,520) 580×420，内部全绝对
    // 定位（内容区 = 窗口相对 y-24 标题栏）；DuelStage 作背景层（不进 DesignSpace），
    // 播放控制扩展件放在窗下方面板。
    <div
      id="replay-screen"
      className="screen active"
      style={{ background: "url('../textures/bg_menu.jpg') center/cover no-repeat", flexDirection: 'column' }}
    >
      <DuelStage onExit={onNavigate} showSurrender={false} interactive={false} compact onReady={handleStageReady} />

      <DesignSpace>
      <div className="gfw-window gfw-replay">
        <div className="gfw-title">观看录像</div>
        {/* 内容区 396px（420-24）：
            lstReplayList (10,30)-(350,400) 340×370 / 录像信息 label (360,30) +
            stReplayInfo (360,60)-(570,320) 210×260 / 播放起始 label (360,275) +
            ebRepStartTurn (360,300) 100×20 / 提取卡组 (470,325) / 删除 (360,355) /
            载入 (470,355) / 重命名 (360,385) / 退出 (470,385)，按钮均 100×25 */}
        <div style={{ position: 'relative', height: '396px' }}>
          <div id="replay-list" className="gfw-list" style={{ position: 'absolute', left: '10px', top: '6px', width: '340px', height: '370px' }}>
            {replays.length === 0 && (
              <div className="gfw-list-item" style={{ color: '#7b89ab' }}>（没有录像文件）</div>
            )}
            {replays.map((n) => (
              <div
                key={n}
                className={`gfw-list-item${n === selected ? ' gfw-selected' : ''}`}
                onClick={() => selectReplay(n)}
              >{n}</div>
            ))}
          </div>
          <div className="gfw-label" style={{ position: 'absolute', left: '360px', top: '6px', fontWeight: 'bold' }}>录像信息：</div>
          <div
            id="replay-info"
            style={{ position: 'absolute', left: '360px', top: '36px', width: '210px', height: '260px', overflow: 'auto', whiteSpace: 'pre-wrap', fontSize: '12px', lineHeight: 1.5, background: 'rgba(8, 12, 26, 0.85)', color: '#c9d6f2', border: '1px solid rgba(126, 156, 222, 0.35)', borderRadius: '4px', padding: '4px 6px', boxSizing: 'border-box' }}
          >{selected && info ? infoText(info) : '（未选择录像）'}</div>
          <div className="gfw-label" style={{ position: 'absolute', left: '360px', top: '251px' }}>播放起始于回合：</div>
          <UiInput
            id="replay-start-turn"
            type="number"
            min={1}
            style={{ position: 'absolute', left: '360px', top: '276px', width: '100px', textAlign: 'center' }}
            value={startTurn}
            onChange={(e) => setStartTurn(e.target.value)}
          />
          {toast && (
            <span
              id="replay-toast"
              style={{ position: 'absolute', left: '360px', top: '303px', background: '#e2f0d9', border: '1px solid #4c7a3f', color: '#1e4620', fontSize: '12px', padding: '2px 10px' }}
            >{toast}</span>
          )}
          {/* 原版右下按钮组：提取卡组 1369 / 删除 1361 / 载入 1348 / 重命名 1362 / 退出 1347 */}
          <UiButton id="replay-export-btn" style={{ position: 'absolute', left: '470px', top: '301px', width: '100px' }} onClick={exportDeck} disabled={!selected}>提取卡组</UiButton>
          <UiButton id="replay-delete-btn" style={{ position: 'absolute', left: '360px', top: '331px', width: '100px' }} onClick={deleteReplay} disabled={!selected}>删除录像</UiButton>
          <UiButton id="replay-load-btn" style={{ position: 'absolute', left: '470px', top: '331px', width: '100px' }} onClick={loadReplay} disabled={!selected}>载入录像</UiButton>
          <UiButton id="replay-rename-btn" style={{ position: 'absolute', left: '360px', top: '361px', width: '100px' }} onClick={renameReplay} disabled={!selected}>重命名</UiButton>
          <UiButton style={{ position: 'absolute', left: '470px', top: '361px', width: '100px' }} onClick={() => { pause(); onNavigate('menu'); }}>退出</UiButton>
          {truncated && (
            <div style={{ position: 'absolute', left: '10px', top: '378px', fontSize: '11px', color: '#b35c00' }}>回放已截断：缺少卡牌脚本。</div>
          )}
        </div>
      </div>

      {/* 播放控制（扩展件；原版对应决斗中的 wReplayControl 浮窗 game.cpp:897-904。
          进度条/倍速无原版对应物，集中放在录像窗正下方 (220,530)） */}
      <div
        id="replay-playback-panel"
        className="gfw-window"
        style={{ left: '220px', top: '530px', width: '580px', transform: 'none', padding: '8px 12px', display: 'flex', flexDirection: 'column', gap: '6px', boxSizing: 'border-box' }}
      >
          <div>
            <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '12px', marginBottom: '6px' }}>
              <span>进度</span>
              <span>{currentStep} / {events.length} 步</span>
            </div>
            <div
              id="replay-progress-track"
              style={{ width: '100%', height: '8px', background: 'rgba(8, 12, 26, 0.85)', border: '1px solid rgba(126, 156, 222, 0.35)', borderRadius: '4px', overflow: 'hidden', cursor: stageReady ? 'pointer' : 'default', boxSizing: 'border-box' }}
              onClick={(e) => {
                const rect = e.currentTarget.getBoundingClientRect();
                seekToRatio((e.clientX - rect.left) / rect.width);
              }}
            >
              <div style={{ width: `${progressPct}%`, height: '100%', background: '#4c5a74' }}></div>
            </div>
          </div>

          <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', gap: '10px', flexWrap: 'wrap' }}>
            <UiButton style={{ width: '48px' }} onClick={stepBack} disabled={!stageReady || currentStep === 0}>⏮</UiButton>
            <UiButton
             
              style={{ width: '96px' }}
              onClick={togglePlay}
              disabled={!stageReady || !events.length}
            >
              {isPlaying ? '⏸ 暂停' : '▶ 播放'}
            </UiButton>
            <UiButton style={{ width: '48px' }} onClick={stepForward} disabled={!stageReady || currentStep >= events.length}>⏭</UiButton>
            {/* btnReplaySwap（game.cpp:901 → ReplayMode::SwapField）：交换双方视角 */}
            <UiButton
              id="replay-swap-btn"
             
              style={{ width: '96px' }}
              title="交换双方视角（场地旋转 180°）"
              onClick={() => duelStore.toggleViewSwap()}
              disabled={!stageReady || !events.length}
            >切换视角</UiButton>
            <GfwSelect
              aria-label="回放速度"
              width={90}
              value={String(speed)}
              onValueChange={(v) => changeSpeed(parseFloat(v))}
              options={[
                { value: '0.5', label: '0.5x' }, { value: '1', label: '1.0x' },
                { value: '2', label: '2.0x' }, { value: '4', label: '4.0x' },
              ]}
            />
          </div>
      </div>
      </DesignSpace>
    </div>
  );
}
