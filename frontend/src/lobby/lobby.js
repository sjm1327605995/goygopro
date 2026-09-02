/**
 * Lobby & Server Connection Manager
 * Manages Server connections, Room creation, and Waiting Lobby.
 */

import { WailsBridge, eventBus } from '../wails_bridge.js';

export class LobbyManager {
  constructor(elements, onDuelStart) {
    this.elements = elements;
    this.onDuelStart = onDuelStart;

    this.isHost = false;
    this.isReady = false;
    this.playerSlot = 0;

    this.initEventListeners();
    this.initBridgeEvents();
  }

  initEventListeners() {
    // Connect to remote server
    this.elements.connectBtn.addEventListener('click', () => this.connectServer());

    // Host local duel server
    this.elements.hostLocalBtn.addEventListener('click', () => this.startLocalServer());

    // Create Game / Room
    this.elements.createRoomBtn.addEventListener('click', () => this.createRoom());

    // Waiting Room Ready Toggle
    this.elements.readyBtn.addEventListener('click', () => {
      this.isReady = !this.isReady;
      WailsBridge.setReady(this.isReady);
      this.elements.readyBtn.innerText = this.isReady ? 'Cancel Ready' : 'Ready Up';
      this.elements.readyBtn.className = this.isReady ? 'btn btn-secondary' : 'btn btn-gold';
    });

    // Start Duel (Host only)
    this.elements.startDuelBtn.addEventListener('click', () => {
      WailsBridge.startDuel();
    });

    // Send Chat
    this.elements.chatSendBtn.addEventListener('click', () => this.sendChat());
    this.elements.chatInput.addEventListener('keypress', (e) => {
      if (e.key === 'Enter') this.sendChat();
    });
  }

  initBridgeEvents() {
    eventBus.on('stoc:type_change', (data) => {
      this.isHost = data.isHost;
      this.playerSlot = data.pos;
      this.elements.startDuelBtn.style.display = this.isHost ? 'inline-flex' : 'none';
    });

    eventBus.on('stoc:player_enter', (data) => {
      if (data.pos === 0) {
        this.elements.player1Name.innerText = data.name;
        this.elements.player1Status.innerText = 'Connected';
      } else if (data.pos === 1) {
        this.elements.player2Name.innerText = data.name;
        this.elements.player2Status.innerText = 'Connected';
      }
    });

    eventBus.on('stoc:player_change', (data) => {
      const statusText = data.ready ? 'READY' : 'NOT READY';
      const statusColor = data.ready ? '#10b981' : '#9ca3af';

      if (data.pos === 0) {
        this.elements.player1Status.innerText = statusText;
        this.elements.player1Status.style.color = statusColor;
      } else if (data.pos === 1) {
        this.elements.player2Status.innerText = statusText;
        this.elements.player2Status.style.color = statusColor;
      }
    });

    eventBus.on('stoc:duel_start', () => {
      if (this.onDuelStart) this.onDuelStart();
    });

    eventBus.on('stoc:chat', (data) => {
      const msgItem = document.createElement('div');
      msgItem.className = 'log-entry';
      msgItem.innerText = `[Duelist ${data.player}]: ${data.msg}`;
      this.elements.roomChatBox.appendChild(msgItem);
      this.elements.roomChatBox.scrollTop = this.elements.roomChatBox.scrollHeight;
    });
  }

  async connectServer() {
    const addr = this.elements.serverAddrInput.value.trim() || '127.0.0.1:7911';
    const name = this.elements.usernameInput.value.trim() || 'Duelist';
    const pass = this.elements.passwordInput.value.trim();

    const res = await WailsBridge.connectServer(addr, name, pass);
    if (res.success) {
      this.elements.serverConnectPanel.style.display = 'none';
      this.elements.roomLobbyPanel.style.display = 'flex';
      this.elements.player1Name.innerText = name;
    } else {
      alert(`Connection failed: ${res.error}`);
    }
  }

  async startLocalServer() {
    const port = parseInt(this.elements.localPortInput.value, 10) || 7911;
    const res = await WailsBridge.startLocalServer(port);
    if (res.success) {
      alert(`Local server started on port ${port}! Connecting...`);
      this.elements.serverAddrInput.value = `127.0.0.1:${port}`;
      this.connectServer();
    } else {
      alert(`Failed to start local server: ${res.error}`);
    }
  }

  async createRoom() {
    const roomName = this.elements.roomNameInput.value.trim() || 'Duel Room';
    const pass = this.elements.roomPassInput.value.trim();

    const req = {
      lflist: 0,
      rule: 0,
      mode: 0,
      duelRule: 5, // Master Rule 2020 / MR5
      startLp: 8000,
      startHand: 5,
      drawCount: 1,
      timeLimit: 180
    };

    const res = await WailsBridge.createGame(req, roomName, pass);
    if (res.success) {
      this.isHost = true;
      this.elements.startDuelBtn.style.display = 'inline-flex';
    }
  }

  sendChat() {
    const msg = this.elements.chatInput.value.trim();
    if (!msg) return;
    WailsBridge.sendChat(msg);
    this.elements.chatInput.value = '';
  }
}
