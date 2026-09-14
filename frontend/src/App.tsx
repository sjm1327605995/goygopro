import React, { useEffect, useState } from 'react';
import { eventBus, WailsBridge } from './wails_bridge.ts';
import { soundManager } from './audio/sound_manager.ts';
import { settingsStore } from './domain/settings.ts';
import chainPrefs, { chainPrefFromSettings } from './duel/chain_prefs.ts';
import MainMenu from './components/MainMenu.tsx';
import Lobby from './components/Lobby.tsx';
import DeckBuilder from './components/DeckBuilder.tsx';
import DuelScreen from './components/DuelScreen.tsx';
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
    };
    apply();
    return settingsStore.subscribe(apply);
  }, []);

  const toggleMute = () => setMuted(soundManager.toggleMute());

  const startPracticeDuel = () => {
    setDuelMode('practice');
    setScreen('duel');
  };
  const startOnlineDuel = () => {
    setDuelMode('online');
    setScreen('duel');
  };

  return (
    <div id="app">
      <button
        id="btn-sound-toggle"
        style={{
          position: 'fixed', top: '16px', right: '16px', zIndex: 999,
          background: 'rgba(0,0,0,0.6)', border: '1px solid var(--border-cyan)',
          borderRadius: '50%', width: '40px', height: '40px', color: '#fff',
          fontSize: '18px', cursor: 'pointer', display: 'flex',
          alignItems: 'center', justifyContent: 'center',
        }}
        onClick={toggleMute}
      >
        {muted ? '🔇' : '🔊'}
      </button>
      <button
        id="btn-open-settings"
        style={{
          position: 'fixed', top: '16px', right: '64px', zIndex: 999,
          background: 'rgba(0,0,0,0.6)', border: '1px solid var(--border-cyan)',
          borderRadius: '50%', width: '40px', height: '40px', color: '#fff',
          fontSize: '18px', cursor: 'pointer', display: 'flex',
          alignItems: 'center', justifyContent: 'center',
        }}
        onClick={() => setSettingsOpen(true)}
        title="环境设置"
      >
        ⚙
      </button>

      {screen === 'menu' && (
        <MainMenu
          onDuel={() => setScreen('lobby')}
          onPractice={startPracticeDuel}
          onDeck={() => setScreen('deck')}
          onReplay={() => setScreen('replay')}
          onQuit={() => WailsBridge.quit()}
        />
      )}
      {screen === 'lobby' && <Lobby onNavigate={setScreen} onDuelStart={startOnlineDuel} />}
      {screen === 'deck' && <DeckBuilder onNavigate={setScreen} />}
      {screen === 'duel' && <DuelScreen mode={duelMode} onNavigate={setScreen} />}
      {screen === 'replay' && <ReplayTheater onNavigate={setScreen} />}

      <div id="modal-overlay" className="modal-overlay"></div>
      {/* 换副卡组（三局两胜局间 STOC_CHANGE_SIDE）；平时自隐藏 */}
      <SideDecking />
      {/* 录像保存确认（决斗结束 STOC_REPLAY；平时自隐藏） */}
      <ReplaySavePrompt />
      <SettingsPanel open={settingsOpen} onClose={() => setSettingsOpen(false)} />
    </div>
  );
}
