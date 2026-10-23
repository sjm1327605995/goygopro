import React, { useEffect, useState } from 'react';
import UiRoot from './ui/UiRoot.tsx';
import { eventBus, WailsBridge } from './wails_bridge.ts';
import { soundManager } from './audio/sound_manager.ts';
import { bgmManager } from './audio/bgm_manager.ts';
import { settingsStore } from './domain/settings.ts';
import { duelStore } from './duel/store.ts';
import chainPrefs, { chainPrefFromSettings } from './duel/chain_prefs.ts';
import MainMenu from './components/MainMenu.tsx';
import Lobby from './components/Lobby.tsx';
import DeckBuilder from './components/DeckBuilder.tsx';
import DuelScreen from './components/DuelScreen.tsx';
import SingleModeWindow from './components/SingleModeWindow.tsx';
import ReplayTheater from './components/ReplayTheater.tsx';
import SideDecking from './components/SideDecking.tsx';
import ReplaySavePrompt from './components/ReplaySavePrompt.tsx';
import SettingsPanel from './components/SettingsPanel.tsx';

/**
 * Root React component: screen router + global chrome.
 * Most game logic lives in the Go backend (Wails service); this layer only
 * renders the GUI and forwards player input across the Wails bridge.
 */
export default function App() {
  const [screen, setScreen] = useState('menu');
  const [duelMode, setDuelMode] = useState('online');
  const [muted, setMuted] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);

  useEffect(() => {
    // The imperative HUD (and anything else) can request a screen change by
    // emitting 'nav' on the event bus.
    const onNav = (name: string) => setScreen(name);
    eventBus.on('nav', onNav);
    // STOC_DUEL_END：对局彻底结束（含三局两胜终局）→ 回等候区
    const onDuelEnd = () => setScreen((s) => (s === 'duel' ? 'lobby' : s));
    eventBus.on('stoc:duel_end', onDuelEnd);
    // 连接断开（服务器关闭/掉线/离开房间后连接终止）→ 回主菜单
    const onDisconnected = () => setScreen('menu');
    eventBus.on('stoc:disconnected', onDisconnected);
    // 启动加载 system.conf（音量/屏蔽聊天/大厅记忆等消费方都读 settingsStore）
    settingsStore.load();
    // 连锁三键初始态来自配置（autochain/waitchain）；用户手动点过（mode 非
    // null）后不再被配置覆盖
    const syncChainPrefs = () => {
      if (chainPrefs.get() !== null) return;
      const s = settingsStore.getSnapshot();
      const pref = chainPrefFromSettings(Number(s.autochain), Number(s.waitchain));
      if (pref !== null) chainPrefs.set(pref);
    };
    settingsStore.load().then(syncChainPrefs);
    const offSettings = settingsStore.subscribe(syncChainPrefs);
    return () => {
      eventBus.off('nav', onNav);
      eventBus.off('stoc:duel_end', onDuelEnd);
      eventBus.off('stoc:disconnected', onDisconnected);
      offSettings();
    };
  }, []);

  // 音量设置 → soundManager（sound_volume 0-100 → 0-1；enable_sound 关 = 全局静音）
  useEffect(() => {
    const apply = () => {
      const s = settingsStore.getSnapshot();
      soundManager.setVolume(
        Number(s.sound_volume) / 100,
        Number(s.music_volume) / 100,
      );
      const shouldMute = !s.enable_sound;
      if (shouldMute !== soundManager.muted) {
        setMuted(soundManager.toggleMute());
      }
      // BGM：enable_music 开/关、music_volume 音量、music_mode 场景细分
      bgmManager.setVolume(Number(s.music_volume) / 100);
      bgmManager.setEnabled(!!s.enable_music);
      bgmManager.setMode(Number(s.music_mode));
    };
    apply();
    return settingsStore.subscribe(apply);
  }, []);

  // 路由 → BGM 基础场景（menu/deck/duel；lobby/single/replay 归 menu）
  useEffect(() => {
    bgmManager.setRoute(screen);
  }, [screen]);

  // 决斗内状态 → BGM 场景（LP 优劣 / 胜负；原版 advantage/disadvantage/win/lose）
  useEffect(() => {
    const sync = () => bgmManager.syncDuel(duelStore.getState());
    sync();
    return duelStore.subscribe(sync);
  }, []);

  const toggleMute = () => setMuted(soundManager.toggleMute());

  const startOnlineDuel = () => {
    setDuelMode('online');
    setScreen('duel');
  };
  // 单人模式（wSinglePlay 确定 → BUTTON_LOAD_SINGLEPLAY → StartPlay）：
  // 谜题引擎事件流与在线对局同构，复用决斗画面。
  const startSingleDuel = () => {
    setDuelMode('single');
    setScreen('duel');
  };

  return (
    <UiRoot>
    <div id="app">
      <button
        id="btn-sound-toggle"
        className="app-utility-btn"
        aria-label={muted ? '开启音效' : '静音'}
        aria-pressed={!muted}
        onClick={toggleMute}
      >
        <svg className="app-utility-icon" viewBox="0 0 24 24" aria-hidden="true">
          <path d="M4 9.5h3.5L12 6v12l-4.5-3.5H4z" />
          {muted ? (
            <path d="M16 9l5 6M21 9l-5 6" />
          ) : (
            <>
              <path d="M16 8.5a5 5 0 0 1 0 7" />
              <path d="M18.6 6a8.5 8.5 0 0 1 0 12" />
            </>
          )}
        </svg>
      </button>
      <button
        id="btn-open-settings"
        className="app-utility-btn"
        aria-label="环境设置"
        onClick={() => setSettingsOpen(true)}
        title="环境设置"
      >
        <svg className="app-utility-icon" viewBox="0 0 24 24" aria-hidden="true">
          <circle cx="12" cy="12" r="3.2" />
          <path d="M19.4 15a1.7 1.7 0 0 0 .34 1.87l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.7 1.7 0 0 0-1.87-.34 1.7 1.7 0 0 0-1.03 1.56V21a2 2 0 1 1-4 0v-.09a1.7 1.7 0 0 0-1.11-1.56 1.7 1.7 0 0 0-1.87.34l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.7 1.7 0 0 0 4.6 15a1.7 1.7 0 0 0-1.56-1.03H3a2 2 0 1 1 0-4h.09A1.7 1.7 0 0 0 4.6 8.86a1.7 1.7 0 0 0-.34-1.87l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.7 1.7 0 0 0 8.96 4.5a1.7 1.7 0 0 0 1.03-1.56V3a2 2 0 1 1 4 0v.09a1.7 1.7 0 0 0 1.03 1.56 1.7 1.7 0 0 0 1.87-.34l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.7 1.7 0 0 0-.34 1.87V9a1.7 1.7 0 0 0 1.56 1.03H21a2 2 0 1 1 0 4h-.09a1.7 1.7 0 0 0-1.51 1.03z" />
        </svg>
      </button>

      {screen === 'menu' && (
        <MainMenu
          onDuel={() => setScreen('lobby')}
          onPractice={() => setScreen('single')}
          onDeck={() => setScreen('deck')}
          onReplay={() => setScreen('replay')}
          onQuit={() => WailsBridge.quit()}
        />
      )}
      {screen === 'lobby' && <Lobby onNavigate={setScreen} onDuelStart={startOnlineDuel} />}
      {screen === 'deck' && <DeckBuilder onNavigate={setScreen} />}
      {screen === 'single' && <SingleModeWindow onNavigate={setScreen} onStart={startSingleDuel} />}
      {screen === 'duel' && <DuelScreen mode={duelMode} onNavigate={setScreen} />}
      {screen === 'replay' && <ReplayTheater onNavigate={setScreen} />}

      <div id="modal-overlay" className="modal-overlay"></div>
      {/* 换副卡组（三局两胜局间 STOC_CHANGE_SIDE）；平时自隐藏 */}
      <SideDecking />
      {/* 录像保存确认（决斗结束 STOC_REPLAY；平时自隐藏） */}
      <ReplaySavePrompt />
      <SettingsPanel open={settingsOpen} onClose={() => setSettingsOpen(false)} />
    </div>
    </UiRoot>
  );
}
