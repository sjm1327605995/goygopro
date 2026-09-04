// Headless smoke test for the replay pipeline: drives the real ReplayApplier
// (same source the production bundle compiles) through a recorded-style event
// stream against the real DuelField3D + DuelHUD, then checks board state
// after animated playback, after seeks, and after a full rebuild.
import { DuelField3D } from '../src/duel/field3d.js';
import { DuelHUD } from '../src/ui/hud.js';
import { ReplayApplier } from '../src/duel/replay_apply.js';
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
const hud = new DuelHUD(hudElements, () => {});

// A realistic slice of the event stream App.PlayReplay produces: draws, a
// normal summon, a set, a destroyed set card moving to grave, an attack, LP
// damage, a hand discard into the opponent's grave, and the win.
const EVENTS = [
  { type: 'duel:start', data: { lp0: 8000, lp1: 8000 } },
  { type: 'duel:draw', data: { player: 0, count: 5, cards: [89631139, 46986414, 83764718, 55144522, 44095762] } },
  { type: 'duel:draw', data: { player: 1, count: 5, cards: [0, 0, 0, 0, 0] } },
  { type: 'duel:new_phase', data: { phase: 0x04 } },
  { type: 'duel:summoning', data: { code: 89631139, cc: 0, cl: 0x4, cs: 2, cp: 0x1 } },
  { type: 'duel:set', data: { code: 44095762, cc: 1, cl: 0x8, cs: 2, cp: 0xa } },
  { type: 'duel:move', data: { code: 44095762, pc: 1, pl: 0x08, ps: 2, pp: 0xa, cc: 1, cl: 0x10, cs: 0, cp: 0x1, reason: 0x40 } },
  { type: 'duel:attack', data: { attacker: { c: 0, l: 4, s: 2 }, target: { c: 1, l: 0, s: 0 } } },
  { type: 'duel:damage', data: { player: 1, amount: 3000 } },
  { type: 'duel:move', data: { code: 83764718, pc: 0, pl: 0x02, ps: 2, pp: 0, cc: 1, cl: 0x10, cs: 1, cp: 0x1, reason: 0x4000 } },
  { type: 'duel:win', data: { winner: 0, type: 0 } },
];

const applier = new ReplayApplier(field, hud, EVENTS);

// Snap every in-flight tween to its end state (headless Chrome throttles rAF).
const settle = () => {
  field.tweenGroup.update(performance.now() + 5000);
  field.tweenGroup.update(performance.now() + 5000);
  field.renderer.render(field.scene, field.camera);
};

const snapshot = () => ({
  mzoneMeshes: field.cardMeshes.filter(m => m.userData.slot && m.userData.slot.loc === 'mzone').length,
  grave1: field.cardsOnField[1].grave.length,
  graveMeshes: field.cardsOnField[1].grave.filter(Boolean).length,
  hand0: hud.handCards.length,
  lp0: hud.playerLP,
  lp1: hud.opponentLP,
  blueEyesRotY: field.cardsOnField[0].mzone[2] ? Math.round(field.cardsOnField[0].mzone[2].rotation.y * 100) / 100 : null,
});

window.__replaySmoke = { field, hud, applier, checks: {}, ready: false };

(async () => {
  await applier.preload();

  // 1. Animated playback of every event (bookkeeping is synchronous even
  //    though tweens are still in flight).
  EVENTS.forEach((evt) => applier.apply(evt));
  settle();
  const played = snapshot();

  // 2. Full rebuild via seek — must reproduce the same board state.
  applier.seek(EVENTS.length);
  settle();
  const rebuilt = snapshot();

  // 3. Seek to just after the destroyed set card entered the grave (step 7 =
  //    events 0..6): the set card is in player 1's grave, no damage yet, the
  //    summon is still on the board.
  applier.seek(7);
  settle();
  const mid = snapshot();

  // 4. Seek back to the very beginning: empty board, starting LP.
  applier.seek(0);
  settle();
  const zero = snapshot();

  const eq = (a, b) => JSON.stringify(a) === JSON.stringify(b);
  window.__replaySmoke.checks = {
    played,
    rebuilt,
    mid,
    zero,
    boardRebuiltIdentically: eq(
      { ...played, lp0: 0, lp1: 0 }, { ...rebuilt, lp0: 0, lp1: 0 }
    ) && played.lp0 === rebuilt.lp0 && played.lp1 === rebuilt.lp1,
    summonOnBoard: played.mzoneMeshes === 1 && played.blueEyesRotY === 0,
    graveHasTwoCards: played.grave1 === 2 && played.graveMeshes === 2,
    // Hand: 5 drawn − 1 summoned (Blue-Eyes) − 1 discarded (Monster Reborn).
    handDiscarded: played.hand0 === 3,
    damageApplied: played.lp1 === 5000 && played.lp0 === 8000,
    // Mid-state hand: 5 drawn − 1 summoned.
    midState: mid.mzoneMeshes === 1 && mid.graveMeshes === 1 && mid.lp1 === 8000 && mid.hand0 === 4,
    zeroState: zero.mzoneMeshes === 0 && zero.graveMeshes === 0 && zero.hand0 === 0 && zero.lp1 === 8000,
  };
  window.__replaySmoke.ready = true;
})();
