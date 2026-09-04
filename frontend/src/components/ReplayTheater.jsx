import React, { useEffect, useRef, useState } from 'react';
import DuelStage from './DuelStage.jsx';
import { WailsBridge } from '../wails_bridge.js';
import { ReplayApplier } from '../duel/replay_apply.js';

/**
 * Replay Theater: lists recorded .yrp duels, asks the Go backend to replay one
 * (ocgcore regenerates the duel deterministically from the replay's seed), and
 * steps through the returned engine events against the shared 3D board.
 *
 * Playback is driven by the synchronous ReplayApplier rather than the event
 * bus: that keeps step order deterministic (no async card-info fetches in
 * between) and makes step-back/seek cheap — rebuilding state at step N is
 * clearBoard() + N instant re-applies.
 */
export default function ReplayTheater({ onNavigate }) {
  const [replays, setReplays] = useState([]);
  const [selected, setSelected] = useState('');
  const [meta, setMeta] = useState(null);
  const [events, setEvents] = useState([]);
  const [currentStep, setCurrentStep] = useState(0);
  const [isPlaying, setIsPlaying] = useState(false);
  const [speed, setSpeed] = useState(1);
  const [truncated, setTruncated] = useState(false);
  const [applierReady, setApplierReady] = useState(false);
  // Bumped whenever DuelStage hands us a fresh stage, so the applier rebuild
  // effect also re-runs when the stage remounts under an already-loaded event
  // stream (e.g. navigating away and back).
  const [stageVersion, setStageVersion] = useState(0);

  const timerRef = useRef(null);
  const stageRef = useRef(null);          // { field3D, hud, duelManager } from DuelStage
  const applierRef = useRef(null);
  const stepRef = useRef(0);              // mirror of currentStep for the play timer
  const eventsRef = useRef([]);

  useEffect(() => {
    (async () => {
      const names = await WailsBridge.listReplays();
      setReplays(names || []);
    })();
    return () => { if (timerRef.current) clearInterval(timerRef.current); };
  }, []);

  // Builds the applier for a loaded event stream once the 3D stage reports in.
  // Card info is preloaded so every subsequent apply() is synchronous.
  useEffect(() => {
    const stage = stageRef.current;
    if (!stage || !events.length) return undefined;
    let cancelled = false;
    setApplierReady(false);
    (async () => {
      const applier = new ReplayApplier(stage.field3D, stage.hud, events);
      await applier.preload();
      if (cancelled) return;
      applierRef.current = applier;
      applier.reset();
      stepRef.current = 0;
      setCurrentStep(0);
      setApplierReady(true);
    })();
    return () => { cancelled = true; };
  }, [events, stageVersion]);

  const setStep = (step) => {
    stepRef.current = step;
    setCurrentStep(step);
  };

  const loadReplay = async () => {
    if (!selected) return;
    const res = await WailsBridge.playReplay(selected);
    if (!res || !res.success) {
      alert(res && res.error ? res.error : '加载回放失败');
      return;
    }
    pause();
    applierRef.current = null;
    setMeta({ name: res.name || selected, players: res.players || [], startLp: res.startLp, duelRule: res.duelRule });
    eventsRef.current = res.events || [];
    setEvents(res.events || []);
    setTruncated(!!res.truncated);
    setStep(0);
  };

  // Applies the next event. Animated (tweens run); the applier still updates
  // all bookkeeping synchronously, so the step counter and board never desync.
  const stepForward = () => {
    const applier = applierRef.current;
    if (!applier) return;
    if (stepRef.current < eventsRef.current.length) {
      applier.apply(eventsRef.current[stepRef.current]);
      setStep(stepRef.current + 1);
    } else {
      pause();
    }
  };

  const stepBack = () => {
    const applier = applierRef.current;
    if (!applier || stepRef.current === 0) return;
    applier.seek(stepRef.current - 1);
    setStep(stepRef.current - 1);
  };

  // Click on the progress bar to jump; the board is rebuilt instantly.
  const seekToRatio = (ratio) => {
    const applier = applierRef.current;
    if (!applier || !eventsRef.current.length) return;
    const target = Math.max(0, Math.min(eventsRef.current.length, Math.round(ratio * eventsRef.current.length)));
    applier.seek(target);
    setStep(target);
  };

  const play = () => {
    if (!applierRef.current) return;
    setIsPlaying(true);
    const ms = Math.max(60, Math.round(1000 / (speed || 1)));
    clearInterval(timerRef.current);
    timerRef.current = setInterval(() => {
      const applier = applierRef.current;
      if (!applier || stepRef.current >= eventsRef.current.length) {
        setIsPlaying(false);
        clearInterval(timerRef.current);
        timerRef.current = null;
        return;
      }
      applier.apply(eventsRef.current[stepRef.current]);
      setStep(stepRef.current + 1);
    }, ms);
  };

  const pause = () => {
    setIsPlaying(false);
    if (timerRef.current) { clearInterval(timerRef.current); timerRef.current = null; }
  };

  const togglePlay = () => { if (isPlaying) pause(); else play(); };

  const changeSpeed = (v) => {
    setSpeed(v);
    if (isPlaying) { pause(); setSpeed(v); play(); }
  };

  const handleStageReady = (stage) => {
    stageRef.current = stage;
    if (!stage) {
      applierRef.current = null;
    } else {
      setStageVersion((v) => v + 1);
    }
  };

  const progressPct = events.length ? (currentStep / events.length) * 100 : 0;
  const playersText = meta && meta.players.length ? meta.players.join(' vs ') : '— vs —';

  return (
    <div id="replay-screen" className="screen active" style={{ flexDirection: 'column', padding: '20px 40px' }}>
      <div className="lobby-header">
        <div className="lobby-title">回放剧场</div>
        <button className="btn btn-secondary" onClick={() => { pause(); onNavigate('menu'); }}>← 返回主菜单</button>
      </div>

      <div style={{ display: 'flex', gap: '16px', flex: 1, minHeight: 0 }}>
        <div style={{ width: '300px', background: 'var(--bg-card)', border: '1px solid var(--border-cyan)', borderRadius: '12px', padding: '20px', display: 'flex', flexDirection: 'column', gap: '12px' }}>
          <h3 style={{ color: 'var(--primary)', margin: 0 }}>对局：{meta ? meta.name : '尚未选择回放'}</h3>
          <p style={{ color: 'var(--text-muted)', fontSize: '13px', lineHeight: 1.6, margin: 0 }}>
            规则：大师规则 {meta && meta.duelRule != null ? meta.duelRule : '—'}<br />
            选手：{playersText}<br />
            初始 LP：{meta && meta.startLp != null ? meta.startLp : '—'}
          </p>

          <div style={{ display: 'flex', gap: '8px' }}>
            <select className="form-select" style={{ flex: 1 }} value={selected} onChange={(e) => setSelected(e.target.value)}>
              <option value="">选择回放文件...</option>
              {replays.map((n) => <option key={n} value={n}>{n}</option>)}
            </select>
            <button className="btn btn-primary" style={{ padding: '0 20px' }} onClick={loadReplay} disabled={!selected}>加载</button>
          </div>

          {truncated && (
            <div style={{ fontSize: '11px', color: '#f59e0b' }}>回放已截断：缺少卡牌脚本。</div>
          )}

          <div style={{ background: 'rgba(0,0,0,0.4)', padding: '16px', borderRadius: '8px', border: '1px solid var(--border-color)', marginTop: 'auto' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '13px', marginBottom: '8px' }}>
              <span>进度</span>
              <span>{currentStep} / {events.length} 步</span>
            </div>
            <div
              id="replay-progress-track"
              style={{ width: '100%', height: '8px', background: '#1e293b', borderRadius: '4px', overflow: 'hidden', marginBottom: '16px', cursor: applierReady ? 'pointer' : 'default' }}
              onClick={(e) => {
                const rect = e.currentTarget.getBoundingClientRect();
                seekToRatio((e.clientX - rect.left) / rect.width);
              }}
            >
              <div style={{ width: `${progressPct}%`, height: '100%', background: 'linear-gradient(90deg, #00d2ff, #f59e0b)' }}></div>
            </div>

            <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', gap: '10px', flexWrap: 'wrap' }}>
              <button className="btn btn-secondary" onClick={stepBack} disabled={!applierReady || currentStep === 0}>⏮</button>
              <button
                className="btn btn-primary"
                style={{ padding: '8px 24px' }}
                onClick={togglePlay}
                disabled={!applierReady || !events.length}
              >
                {isPlaying ? '⏸ 暂停' : '▶ 播放'}
              </button>
              <button className="btn btn-secondary" onClick={stepForward} disabled={!applierReady || currentStep >= events.length}>⏭</button>
              <select className="form-select" style={{ width: '90px' }} value={speed} onChange={(e) => changeSpeed(parseFloat(e.target.value))}>
                <option value="0.5">0.5x</option>
                <option value="1">1.0x</option>
                <option value="2">2.0x</option>
                <option value="4">4.0x</option>
              </select>
            </div>
          </div>
        </div>

        <div style={{ flex: 1, position: 'relative', minWidth: 0, borderRadius: '12px', overflow: 'hidden', border: '1px solid var(--border-color)' }}>
          <DuelStage onExit={onNavigate} showSurrender={false} onReady={handleStageReady} />
        </div>
      </div>
    </div>
  );
}
