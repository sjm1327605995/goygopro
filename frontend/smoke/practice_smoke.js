// Headless smoke test for practice mode: drives the real interactive stack —
// DuelField3D + DuelHUD + DuelManager + AISimulator — through real player
// actions (summon → battle → attack → end turn) and checks that each action
// actually drove the corresponding state transition, exactly as it would in
// a network duel. The simulator emits the same eventBus events the Go engine
// produces, so DuelManager needs no practice-mode special casing.
import { DuelField3D } from '../src/duel/field3d.js';
import { DuelHUD } from '../src/ui/hud.js';
import { DuelManager } from '../src/duel/duel_manager.js';
import { aiSimulator } from '../src/duel/ai_simulator.js';
import { eventBus } from '../src/wails_bridge.js';
import { soundManager } from '../src/audio/sound_manager.js';

soundManager.muted = true;

// --- Minimal HUD DOM (the real ids DuelStage would provide) ---
const el = (id, tag = 'div', parent = document.body) => {
  const node = document.createElement(tag);
  node.id = id;
  parent.appendChild(node);
  return node;
};
const hudElements = {
  playerLP: el('player-lp-val', 'span'),
  playerLPFill: el('player-lp-fill', 'div'),
  opponentLP: el('opponent-lp-val', 'span'),
  opponentLPFill: el('opponent-lp-fill', 'div'),
  handContainer: el('hand-cards-dock', 'div'),
  actionPopup: el('action-popup', 'div'),
  inspectorPicCanvas: el('inspector-pic-canvas', 'canvas'),
  inspectorName: el('inspector-card-name', 'div'),
  inspectorBadges: el('inspector-badges', 'div'),
  inspectorStats: el('inspector-stats', 'div'),
  inspectorDesc: el('inspector-card-desc', 'div'),
  modalOverlay: el('modal-overlay', 'div'),
  logList: el('duel-log-list', 'div'),
};
hudElements.playerLPFill.style.width = '100%';
hudElements.opponentLPFill.style.width = '100%';

const field = new DuelField3D(document.getElementById('stage'), () => {}, () => {});
const hud = new DuelHUD(hudElements, (type, data) => manager.handlePlayerAction(type, data));
const manager = new DuelManager(field, hud);
hud.handActionProvider = (code) => manager.getHandOptions(code);

aiSimulator.DELAY = 40;

// Record every duel event so checks can inspect the full transcript.
const transcript = [];
const origEmit = eventBus.emit.bind(eventBus);
eventBus.emit = (event, data) => {
  if (event.startsWith('duel:') || event.startsWith('stoc:')) transcript.push({ event, data });
  return origEmit(event, data);
};
const emitted = (type) => transcript.some((t) => t.event === type);

// Polls until `predicate` holds (draw tweens finish asynchronously in the
// headless renderer before the hand dock updates).
const waitFor = (predicate, timeoutMs = 5000) => new Promise((resolve, reject) => {
  const started = Date.now();
  const tick = () => {
    if (predicate()) return resolve();
    if (Date.now() - started > timeoutMs) return reject(new Error('waitFor timeout'));
    setTimeout(tick, 25);
  };
  tick();
});

// Resolves when `type` fires after the call (subscribe before acting — no races).
const nextEvent = (type, timeoutMs = 5000) => new Promise((resolve, reject) => {
  const timer = setTimeout(() => {
    eventBus.off(type, listener);
    reject(new Error('timeout waiting for ' + type));
  }, timeoutMs);
  const listener = (data) => {
    clearTimeout(timer);
    eventBus.off(type, listener);
    resolve(data);
  };
  eventBus.on(type, listener);
});

window.__practiceSmoke = { field, hud, manager, aiSimulator, transcript, checks: {}, ready: false };

(async () => {
  aiSimulator.start();

  // Turn 1: the simulator prompts the player with an idle command built from
  // the actual opening hand.
  const firstIdle = await nextEvent('duel:select_idlecmd');
  await waitFor(() => hud.handCards.length === 5);
  const checks = {
    // Blue-Eyes is level 8: rules-correct prompts offer it as a monster set,
    // not a normal summon (no tributes in this simplified simulator).
    idlePromptHasMset: firstIdle.mset.some((c) => c.code === 89631139),
    handDealt: hud.handCards.length === 5,
  };

  // Player normal-summons Blue-Eyes (first card of the scripted deck) through
  // the same entry point the HUD popup uses.
  const rePrompt = nextEvent('duel:select_idlecmd');
  manager.handlePlayerAction('summon', { code: 89631139 });
  await rePrompt;
  checks.summonPlacedMesh = !!(field.cardsOnField[0].mzone[0]);
  checks.summonLeftHand = hud.handCards.length === 4;
  checks.summonLogged = emitted('duel:summoning');

  // End turn → the AI takes its turn (sets a facedown monster, cannot attack)
  // and hands the turn back.
  const aiIdle = nextEvent('duel:select_idlecmd');
  manager.handlePlayerAction('phase_change', 'EP');
  await aiIdle;
  checks.aiSetFacedown = !!(field.cardsOnField[1].mzone[0]) &&
    Math.round(field.cardsOnField[1].mzone[0].rotation.y * 100) / 100 === 4.71;
  checks.turnReturned = emitted('duel:new_turn');

  // Turn 2: go to battle and attack — the AI's only monster is face-down, so
  // the attack resolves as direct damage for full ATK.
  const battlePrompt = nextEvent('duel:select_battlecmd');
  manager.handlePlayerAction('phase_change', 'BP');
  const battleCmd = await battlePrompt;
  checks.battlePromptHasAttack = battleCmd.attack.some((a) => a.s === 0 && a.code === 89631139);

  const damageEvent = nextEvent('duel:damage');
  const reBattlePrompt = nextEvent('duel:select_battlecmd');
  manager.handlePlayerAction('attack', { s: 0 });
  const dmg = await damageEvent;
  await reBattlePrompt;
  checks.directAttackDamage = dmg.player === 1 && dmg.amount === 3000 && hud.opponentLP === 5000;
  checks.attackAnimated = emitted('duel:attack');

  // End turn again: the AI draws, sets a second monster, passes back.
  const thirdTurn = nextEvent('duel:select_idlecmd');
  manager.handlePlayerAction('phase_change', 'EP');
  await thirdTurn;
  await waitFor(() => hud.handCards.length === 6); // 4 + turn-2 + turn-3 draws
  checks.turnThreePrompted = hud.handCards.length === 6;
  checks.noCrashAfterAiTurn = aiSimulator.running;

  checks.logIsChinese = hudElements.logList.innerText.includes('通常召唤')
    && hudElements.logList.innerText.includes('宣告攻击');

  window.__practiceSmoke.checks = checks;
  window.__practiceSmoke.ready = true;
})().catch((err) => {
  window.__practiceSmoke.checks = { fatal: String(err && err.stack || err) };
  window.__practiceSmoke.ready = true;
});
