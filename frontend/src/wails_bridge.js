/**
 * YGOPro Wails Bridge
 * Unifies communication between Web Frontend and Go backend.
 * Provides fallback mock data for testing in standalone browser preview.
 */

// Sample mock card database for standalone preview & testing
const MOCK_CARD_DB = [
  {
    code: 89631139,
    name: "Blue-Eyes White Dragon",
    type: 0x11, // Monster | Normal
    attack: 3000,
    defense: 2500,
    level: 8,
    race: 0x200, // Dragon
    attribute: 0x10, // LIGHT
    desc: "This legendary dragon is a powerful engine of destruction. Virtually invincible, very few have faced this awesome creature and lived to tell the tale."
  },
  {
    code: 46986414,
    name: "Dark Magician",
    type: 0x11, // Monster | Normal
    attack: 2500,
    defense: 2100,
    level: 7,
    race: 0x2, // Spellcaster
    attribute: 0x20, // DARK
    desc: "The ultimate wizard in terms of attack and defense."
  },
  {
    code: 38033121,
    name: "Dark Magician Girl",
    type: 0x21, // Monster | Effect
    attack: 2000,
    defense: 1700,
    level: 6,
    race: 0x2, // Spellcaster
    attribute: 0x20, // DARK
    desc: "Gains 300 ATK for every 'Dark Magician' or 'Magician of Black Chaos' in either player's GY."
  },
  {
    code: 83764718,
    name: "Monster Reborn",
    type: 0x2, // Spell
    attack: 0,
    defense: 0,
    level: 0,
    race: 0,
    attribute: 0,
    desc: "Target 1 monster in either player's GY; Special Summon it."
  },
  {
    code: 55144522,
    name: "Pot of Greed",
    type: 0x2, // Spell
    attack: 0,
    defense: 0,
    level: 0,
    race: 0,
    attribute: 0,
    desc: "Draw 2 cards from your Deck."
  },
  {
    code: 44095762,
    name: "Mirror Force",
    type: 0x4, // Trap
    attack: 0,
    defense: 0,
    level: 0,
    race: 0,
    attribute: 0,
    desc: "When an opponent's monster declares an attack: Destroy all Attack Position monsters your opponent controls."
  },
  {
    code: 12580477,
    name: "Raigeki",
    type: 0x2, // Spell
    attack: 0,
    defense: 0,
    level: 0,
    race: 0,
    attribute: 0,
    desc: "Destroy all monsters your opponent controls."
  },
  {
    code: 5318639,
    name: "Mystical Space Typhoon",
    type: 0x10002, // Spell | Quick-Play
    attack: 0,
    defense: 0,
    level: 0,
    race: 0,
    attribute: 0,
    desc: "Target 1 Spell/Trap on the field; destroy that target."
  },
  {
    code: 41420027,
    name: "Solemn Judgment",
    type: 0x100004, // Trap | Counter
    attack: 0,
    defense: 0,
    level: 0,
    race: 0,
    attribute: 0,
    desc: "When a monster(s) would be Summoned, OR a Spell/Trap Card is activated: Pay half your LP; negate the Summon or activation, and if you do, destroy that card."
  },
  {
    code: 14558127,
    name: "Ash Blossom & Joyous Spring",
    type: 0x21 | 0x800, // Monster | Effect | Tuner
    attack: 0,
    defense: 1800,
    level: 3,
    race: 0x8, // Zombie
    attribute: 0x04, // FIRE
    desc: "When a card or effect is activated that includes any of these effects (Quick Effect): You can discard this card; negate that effect."
  },
  {
    code: 84013237,
    name: "Number 39: Utopia",
    type: 0x800021, // Monster | Effect | Xyz
    attack: 2500,
    defense: 2000,
    level: 4,
    race: 0x1, // Warrior
    attribute: 0x10, // LIGHT
    desc: "2 Level 4 monsters. When any player's monster declares an attack: You can detach 1 material from this card; negate the attack."
  },
  {
    code: 86066372,
    name: "Accesscode Talker",
    type: 0x4000021, // Monster | Effect | Link
    attack: 2300,
    defense: 0,
    level: 4,
    race: 0x1000000, // Cyberse
    attribute: 0x20, // DARK
    desc: "2+ Effect Monsters. If this card is Link Summoned: You can target 1 Link Monster used as material; this card gains ATK equal to that monster's Link Rating x 1000."
  }
];

class EventBus {
  constructor() {
    this.listeners = {};
  }
  on(event, callback) {
    if (!this.listeners[event]) this.listeners[event] = [];
    this.listeners[event].push(callback);
  }
  emit(event, data) {
    if (this.listeners[event]) {
      this.listeners[event].forEach(cb => cb(data));
    }
  }
}

export const eventBus = new EventBus();

// Check if Wails runtime is injected
const isWails = typeof window !== 'undefined' && !!(window.go && window.go.main && window.go.main.App);

if (typeof window !== 'undefined' && window.runtime && window.runtime.EventsOn) {
  // Bridge Wails events to internal eventBus
  const eventList = [
    "stoc:join_game", "stoc:type_change", "stoc:player_enter", "stoc:player_change",
    "stoc:watch_change", "stoc:duel_start", "stoc:deck_count", "stoc:select_hand",
    "stoc:hand_result", "stoc:select_tp", "stoc:time_limit", "stoc:chat",
    "stoc:error_msg", "stoc:duel_end", "stoc:change_side", "stoc:waiting_side",
    "duel:start", "duel:draw", "duel:new_turn", "duel:new_phase", "duel:move",
    "duel:pos_change", "duel:set", "duel:summoning", "duel:summoned",
    "duel:spsummoning", "duel:spsummoned", "duel:flipsummoning", "duel:flipsummoned",
    "duel:chaining", "duel:chained", "duel:chain_solving", "duel:chain_solved",
    "duel:chain_end", "duel:damage", "duel:recover", "duel:lp_update",
    "duel:attack", "duel:battle", "duel:win", "duel:select_idlecmd",
    "duel:select_battlecmd", "duel:select_effectyn", "duel:select_yesno",
    "duel:select_option", "duel:select_card", "duel:select_position",
    "duel:select_place", "duel:select_chain", "duel:waiting"
  ];
  eventList.forEach(name => {
    window.runtime.EventsOn(name, (data) => eventBus.emit(name, data));
  });
}

export const WailsBridge = {
  isWails,

  async connectServer(addr, username, pass) {
    if (isWails) {
      return await window.go.main.App.ConnectServer(addr, username, pass);
    }
    console.log("[MockBridge] ConnectServer:", addr, username);
    setTimeout(() => {
      eventBus.emit("stoc:type_change", { type: 0x10, isHost: true, pos: 0 });
      eventBus.emit("stoc:player_enter", { pos: 0, name: username || "Duelist" });
    }, 200);
    return { success: true };
  },

  async startLocalServer(port) {
    if (isWails) {
      return await window.go.main.App.StartLocalServer(port);
    }
    return { success: true, port: port || 7911 };
  },

  async createGame(req, roomName, pass) {
    if (isWails) {
      return await window.go.main.App.CreateGame(req, roomName, pass);
    }
    return { success: true };
  },

  async joinGame(pass) {
    if (isWails) {
      return await window.go.main.App.JoinGame(pass);
    }
    return { success: true };
  },

  leaveGame() {
    if (isWails) window.go.main.App.LeaveGame();
  },

  surrender() {
    if (isWails) window.go.main.App.Surrender();
  },

  setReady(ready) {
    if (isWails) window.go.main.App.SetReady(ready);
    else {
      eventBus.emit("stoc:player_change", { pos: 0, ready, status: ready ? 9 : 0 });
    }
  },

  startDuel() {
    if (isWails) window.go.main.App.StartDuel();
  },

  sendChat(msg) {
    if (isWails) window.go.main.App.SendChat(msg);
    else {
      eventBus.emit("stoc:chat", { player: 0, msg });
    }
  },

  sendHandResult(res) {
    if (isWails) window.go.main.App.SendHandResult(res);
  },

  sendTPResult(res) {
    if (isWails) window.go.main.App.SendTPResult(res);
  },

  sendResponseI(val) {
    if (isWails) window.go.main.App.SendResponseI(val);
    else console.log("[MockBridge] SendResponseI:", val);
  },

  sendResponseB(bytes) {
    if (isWails) window.go.main.App.SendResponseB(bytes);
    else console.log("[MockBridge] SendResponseB:", bytes);
  },

  async getCard(code) {
    if (isWails) {
      return await window.go.main.App.GetCard(code);
    }
    return MOCK_CARD_DB.find(c => c.code === code) || {
      code,
      name: "Card #" + code,
      type: 0x11,
      attack: 1500,
      defense: 1200,
      level: 4,
      race: 0x1,
      attribute: 0x10,
      desc: "Card details."
    };
  },

  async searchCards(filter) {
    if (isWails) {
      return await window.go.main.App.SearchCards(filter);
    }
    let res = MOCK_CARD_DB;
    if (filter.keyword) {
      const kw = filter.keyword.toLowerCase();
      res = res.filter(c => c.name.toLowerCase().includes(kw) || c.desc.toLowerCase().includes(kw) || String(c.code).includes(kw));
    }
    return res;
  },

  async listDecks() {
    if (isWails) {
      return await window.go.main.App.ListDecks();
    }
    return ["Blue-Eyes Beatdown", "Dark Magician Control", "Cyber Dragon OTK"];
  },

  async loadDeck(name) {
    if (isWails) {
      return await window.go.main.App.LoadDeck(name);
    }
    return {
      name,
      main: [
        89631139, 89631139, 89631139, 46986414, 46986414, 38033121, 38033121,
        83764718, 55144522, 12580477, 5318639, 5318639, 44095762, 44095762,
        41420027, 14558127, 14558127, 14558127
      ],
      extra: [84013237, 86066372],
      side: [41420027, 5318639]
    };
  },

  async saveDeck(deck) {
    if (isWails) {
      return await window.go.main.App.SaveDeck(deck);
    }
    console.log("[MockBridge] Saved deck:", deck.name);
    return true;
  }
};
