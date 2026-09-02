/**
 * Main Application Controller & View Router
 */

import { DuelField3D } from './duel/field3d.js';
import { DuelHUD } from './ui/hud.js';
import { DuelManager } from './duel/duel_manager.js';
import { DeckBuilder } from './deck/deck_builder.js';
import { LobbyManager } from './lobby/lobby.js';
import { eventBus, WailsBridge } from './wails_bridge.js';

class AppController {
  constructor() {
    this.currentScreen = 'main-menu-screen';
    this.field3D = null;
    this.hud = null;
    this.duelManager = null;
    this.deckBuilder = null;
    this.lobbyManager = null;

    this.initDOM();
    this.initModules();
    this.initNavigation();
  }

  initDOM() {
    this.screens = {
      menu: document.getElementById('main-menu-screen'),
      lobby: document.getElementById('lobby-screen'),
      duel: document.getElementById('duel-screen'),
      deck: document.getElementById('deck-screen')
    };
  }

  showScreen(screenId) {
    Object.values(this.screens).forEach(s => s.classList.remove('active'));
    if (this.screens[screenId]) {
      this.screens[screenId].classList.add('active');
      this.currentScreen = screenId;
    }
  }

  initModules() {
    // 1. Initialize 3D Field
    const container = document.getElementById('duel-canvas-container');
    this.field3D = new DuelField3D(
      container,
      (code, info) => this.hud.inspectCard(code, info),
      (mesh, data) => {
        console.log('Clicked 3D card:', data);
      }
    );

    // 2. Initialize HUD
    const hudElements = {
      playerLP: document.getElementById('player-lp-val'),
      playerLPFill: document.getElementById('player-lp-fill'),
      opponentLP: document.getElementById('opponent-lp-val'),
      opponentLPFill: document.getElementById('opponent-lp-fill'),
      handContainer: document.getElementById('hand-cards-dock'),
      actionPopup: document.getElementById('action-popup'),
      inspectorPicCanvas: document.getElementById('inspector-pic-canvas'),
      inspectorName: document.getElementById('inspector-card-name'),
      inspectorBadges: document.getElementById('inspector-badges'),
      inspectorStats: document.getElementById('inspector-stats'),
      inspectorDesc: document.getElementById('inspector-card-desc'),
      modalOverlay: document.getElementById('modal-overlay'),
      logList: document.getElementById('duel-log-list')
    };

    this.hud = new DuelHUD(hudElements, (type, data) => {
      this.duelManager.handlePlayerAction(type, data);
    });

    // 3. Initialize Duel Coordinator
    this.duelManager = new DuelManager(this.field3D, this.hud);

    // 4. Initialize Deck Builder
    const deckElements = {
      deckSelect: document.getElementById('deck-select-dropdown'),
      deckNameInput: document.getElementById('deck-name-input'),
      saveDeckBtn: document.getElementById('save-deck-btn'),
      newDeckBtn: document.getElementById('new-deck-btn'),
      sampleHandBtn: document.getElementById('sample-hand-btn'),
      searchInput: document.getElementById('deck-search-input'),
      typeFilter: document.getElementById('deck-type-filter'),
      mainDeckGrid: document.getElementById('main-deck-grid'),
      extraDeckGrid: document.getElementById('extra-deck-grid'),
      sideDeckGrid: document.getElementById('side-deck-grid'),
      searchResultsGrid: document.getElementById('search-results-grid'),
      mainCountBadge: document.getElementById('main-deck-count'),
      extraCountBadge: document.getElementById('extra-deck-count'),
      sideCountBadge: document.getElementById('side-deck-count')
    };
    this.deckBuilder = new DeckBuilder(deckElements, (code, info) => {
      this.hud.inspectCard(code, info);
    });

    // 5. Initialize Lobby Manager
    const lobbyElements = {
      connectBtn: document.getElementById('server-connect-btn'),
      hostLocalBtn: document.getElementById('server-host-btn'),
      serverAddrInput: document.getElementById('server-addr-input'),
      usernameInput: document.getElementById('username-input'),
      passwordInput: document.getElementById('password-input'),
      localPortInput: document.getElementById('local-port-input'),
      serverConnectPanel: document.getElementById('server-connect-panel'),
      roomLobbyPanel: document.getElementById('room-lobby-panel'),
      roomNameInput: document.getElementById('room-name-input'),
      roomPassInput: document.getElementById('room-pass-input'),
      createRoomBtn: document.getElementById('create-room-btn'),
      player1Name: document.getElementById('lobby-p1-name'),
      player1Status: document.getElementById('lobby-p1-status'),
      player2Name: document.getElementById('lobby-p2-name'),
      player2Status: document.getElementById('lobby-p2-status'),
      readyBtn: document.getElementById('lobby-ready-btn'),
      startDuelBtn: document.getElementById('lobby-start-btn'),
      roomChatBox: document.getElementById('lobby-chat-box'),
      chatInput: document.getElementById('lobby-chat-input'),
      chatSendBtn: document.getElementById('lobby-chat-send')
    };
    this.lobbyManager = new LobbyManager(lobbyElements, () => {
      this.showScreen('duel');
    });
  }

  initNavigation() {
    // Menu buttons
    document.getElementById('menu-duel-btn').onclick = () => this.showScreen('lobby');
    document.getElementById('menu-deck-btn').onclick = () => this.showScreen('deck');
    document.getElementById('menu-practice-btn').onclick = () => this.startPracticeDuel();

    // Back buttons
    document.getElementById('lobby-back-btn').onclick = () => this.showScreen('menu');
    document.getElementById('deck-back-btn').onclick = () => this.showScreen('menu');
    document.getElementById('duel-surrender-btn').onclick = () => {
      if (confirm('Are you sure you want to surrender?')) {
        WailsBridge.surrender();
        this.showScreen('menu');
      }
    };
  }

  startPracticeDuel() {
    this.showScreen('duel');
    setTimeout(async () => {
      eventBus.emit('duel:start', { lp0: 8000, lp1: 8000 });
      // Draw opening hands
      eventBus.emit('duel:draw', { player: 0, count: 5, cards: [89631139, 46986414, 83764718, 55144522, 44095762] });
      eventBus.emit('duel:draw', { player: 1, count: 5, cards: [0, 0, 0, 0, 0] });
      eventBus.emit('duel:new_phase', { phase: 0x04 }); // M1

      // Demonstrate AI summon Blue-Eyes
      setTimeout(() => {
        eventBus.emit('duel:summoning', { code: 89631139, cc: 0, cl: 4, cs: 2, cp: 0x1 });
      }, 1200);
    }, 400);
  }
}

window.addEventListener('DOMContentLoaded', () => {
  window.app = new AppController();
});
