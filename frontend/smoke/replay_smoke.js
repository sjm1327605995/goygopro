// Headless smoke test for the replay pipeline: drives the SAME path the
// ReplayTheater uses — raw {type,data} events re-emitted through eventBus,
// consumed by duelStore (reducer) + a non-interactive DuelManager — through
// animated playback, instant seeks, and a full rebuild. There is no separate
// replay code path anymore (replay_apply.js is gone), so this fixture also
// guards the live path's event semantics.
//
// The fixture mirrors what App.PlayReplay produces: a summoning is preceded
// by its hand→field MSG_MOVE (the reducer's hand sync relies on the move),
// and a discard's move index reflects the hand AFTER the summon removed
// its card.
import { DuelField3D } from '../src/duel/field3d.ts';
import { DuelManager } from '../src/duel/duel_manager.ts';
import { eventBus } from '../src/wails_bridge.ts';
import { soundManager } from '../src/audio/sound_manager.ts';
// store.js wires eventBus → duelStore (reducer); importing it activates the path.
import { duelStore } from '../src/duel/store.ts';

soundManager.muted = true;

const field = new DuelField3D(document.getElementById('stage'), () => {}, () => {});
const manager = new DuelManager(field, { interactive: false });
void manager;

// A realistic slice of the event stream App.PlayReplay produces: draws, a
// normal summon (move + summoning), an AI set (move + set), a destroyed set
// card moving to grave, an attack, LP damage, a hand discard into the
// opponent's grave, and the win.
const EVENTS = [
  { type: 'duel:start', data: { lp0: 8000, lp1: 8000 } },
  { type: 'duel:draw', data: { player: 0, count: 5, cards: [89631139, 46986414, 83764718, 55144522, 44095762] } },
  { type: 'duel:draw', data: { player: 1, count: 5, cards: [0, 0, 0, 0, 0] } },
  { type: 'duel:new_phase', data: { phase: 0x04 } },
  // summon: hand index 0 → mzone slot 2, then the announcement
  { type: 'duel:move', data: { code: 89631139, pc: 0, pl: 0x02, ps: 0, pp: 0x1, cc: 0, cl: 0x4, cs: 2, cp: 0x1, reason: 0 } },
  { type: 'duel:summoning', data: { code: 89631139, cc: 0, cl: 0x4, cs: 2, cp: 0x1 } },
  // AI set: hand → szone slot 2, then the facedown announcement
  { type: 'duel:move', data: { code: 44095762, pc: 1, pl: 0x02, ps: 4, pp: 0xa, cc: 1, cl: 0x8, cs: 2, cp: 0xa, reason: 0 } },
  { type: 'duel:set', data: { code: 44095762, cc: 1, cl: 0x8, cs: 2, cp: 0xa } },
  // the set card is destroyed → player 1's grave
  { type: 'duel:move', data: { code: 44095762, pc: 1, pl: 0x08, ps: 2, pp: 0xa, cc: 1, cl: 0x10, cs: 0, cp: 0x1, reason: 0x40 } },
  { type: 'duel:attack', data: { attacker: { c: 0, l: 4, s: 2 }, target: { c: 1, l: 0, s: 0 } } },
  { type: 'duel:damage', data: { player: 1, amount: 3000 } },
  // hand discard into player 1's grave — index 1 because the summon already
  // removed hand index 0 (Blue-Eyes), shifting Monster Reborn down
  { type: 'duel:move', data: { code: 83764718, pc: 0, pl: 0x02, ps: 1, pp: 0, cc: 1, cl: 0x10, cs: 1, cp: 0x1, reason: 0x4000 } },
  { type: 'duel:win', data: { winner: 0, type: 0 } },
];

// Mirrors ReplayTheater.emitEvent: seek re-applies ride the __instant flag
// (reducer ignores unknown fields; DuelManager snaps instead of tweening).
const emitEvent = (evt, instant) => {
  const data = instant ? { ...(evt.data || {}), __instant: true } : (evt.data || {});
  eventBus.emit(evt.type, data);
};

// Snap every in-flight tween to its end state (headless Chrome throttles rAF).
const settle = () => {
  field.tweenGroup.update(performance.now() + 5000);
  field.tweenGroup.update(performance.now() + 5000);
  field.renderer.render(field.scene, field.camera);
};
// The emit handlers are async (getCard → place); give the microtasks a beat.
const breathe = (ms = 40) => new Promise((r) => setTimeout(r, ms));

const state = () => duelStore.getState();
const snapshot = () => ({
  mzoneMeshes: field.cardMeshes.filter((m) => m.userData.slot && m.userData.slot.loc === 'mzone').length,
  grave1: field.cardsOnField[1].grave.length,
  graveMeshes: field.cardsOnField[1].grave.filter(Boolean).length,
  hand0: state().hand.length,
  lp0: state().lp[0],
  lp1: state().lp[1],
  blueEyesRotY: field.cardsOnField[0].mzone[2]
    ? Math.round(field.cardsOnField[0].mzone[2].rotation.y * 100) / 100 : null,
  // Store-side truth: the reducer's board is what seek(0)→rebuild must reproduce.
  storeBoard: JSON.stringify(state().board),
  storeHand: JSON.stringify(state().hand),
  win: state().win ? `${state().win.winner}:${state().win.type}` : null,
});

window.__replaySmoke = { field, manager, store: duelStore, checks: {}, ready: false };

(async () => {
  // 1. Animated playback of every event (bookkeeping is synchronous in the
  //    store; 3D tweens snap via settle()).
  for (const evt of EVENTS) {
    emitEvent(evt, false);
    await breathe();
  }
  settle();
  await breathe();
  const played = snapshot();

  // 2. Full rebuild via instant seek — must reproduce the same board state.
  duelStore.reset();
  field.clearBoard();
  for (let i = 0; i < EVENTS.length; i++) emitEvent(EVENTS[i], true);
  await breathe();
  settle();
  await breathe();
  const rebuilt = snapshot();

  // 3. Seek to just after the destroyed set card entered the grave (prefix
  //    0..8): the set card is in player 1's grave, no damage yet, the summon
  //    is still on the board.
  duelStore.reset();
  field.clearBoard();
  for (let i = 0; i < 9; i++) emitEvent(EVENTS[i], true);
  await breathe();
  settle();
  await breathe();
  const mid = snapshot();

  // 4. Seek back to the very beginning: empty board, starting LP.
  duelStore.reset();
  field.clearBoard();
  await breathe();
  settle();
  const zero = snapshot();

  const eq = (a, b) => JSON.stringify(a) === JSON.stringify(b);
  window.__replaySmoke.checks = {
    played,
    rebuilt,
    mid,
    zero,
    boardRebuiltIdentically: eq(played, rebuilt),
    summonOnBoard: played.mzoneMeshes === 1 && played.blueEyesRotY === 0,
    graveHasTwoCards: played.grave1 === 2 && played.graveMeshes === 2,
    // Hand: 5 drawn − 1 summoned (Blue-Eyes) − 1 discarded (Monster Reborn).
    handDiscarded: played.hand0 === 3,
    damageApplied: played.lp1 === 5000 && played.lp0 === 8000,
    winRecorded: played.win === '0:0',
    // Mid-state: 5 drawn − 1 summoned; the grave holds only the set card.
    midState: mid.mzoneMeshes === 1 && mid.graveMeshes === 1 && mid.lp1 === 8000 && mid.hand0 === 4,
    zeroState: zero.mzoneMeshes === 0 && zero.graveMeshes === 0 && zero.hand0 === 0 && zero.lp1 === 8000,
  };
  window.__replaySmoke.ready = true;
})().catch((err) => {
  window.__replaySmoke.checks = { fatal: String(err && err.stack || err) };
  window.__replaySmoke.ready = true;
});
