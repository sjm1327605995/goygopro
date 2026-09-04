import React, { useEffect, useState } from 'react';
import { eventBus } from './wails_bridge.js';
import { soundManager } from './audio/sound_manager.js';
import MainMenu from './components/MainMenu.jsx';
import Lobby from './components/Lobby.jsx';
import DeckBuilder from './components/DeckBuilder.jsx';
import DuelScreen from './components/DuelScreen.jsx';
import ReplayTheater from './components/ReplayTheater.jsx';

/**
 * Root React component: screen router + global chrome.
 * Most game logic lives in the Go backend (Wails service); this layer only
 * renders the GUI and forwards player input across the Wails bridge.
 */
export default function App() {
  const [screen, setScreen] = useState('menu');
  const [duelMode, setDuelMode] = useState('online');
  const [muted, setMuted] = useState(false);

  useEffect(() => {
    // The imperative HUD (and anything else) can request a screen change by
    // emitting 'nav' on the event bus.
    eventBus.on('nav', (name) => setScreen(name));
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

      {screen === 'menu' && (
        <MainMenu
          onDuel={() => setScreen('lobby')}
          onPractice={startPracticeDuel}
          onDeck={() => setScreen('deck')}
          onReplay={() => setScreen('replay')}
        />
      )}
      {screen === 'lobby' && <Lobby onNavigate={setScreen} onDuelStart={startOnlineDuel} />}
      {screen === 'deck' && <DeckBuilder onNavigate={setScreen} />}
      {screen === 'duel' && <DuelScreen mode={duelMode} onNavigate={setScreen} />}
      {screen === 'replay' && <ReplayTheater onNavigate={setScreen} />}

      <div id="modal-overlay" className="modal-overlay"></div>
    </div>
  );
}
