// Headless render smoke test for the 3D duel field. Mounts the real
// DuelField3D (same source the production bundle compiles) against the built
// assets, places cards on the board, and flags when the card-back texture has
// decoded so the caller can screenshot and assert on it.
import { DuelField3D } from '../src/duel/field3d.js';
import { soundManager } from '../src/audio/sound_manager.js';

// Keep the headless run silent; every play*() method short-circuits on muted.
soundManager.muted = true;

const container = document.getElementById('stage');
const field = new DuelField3D(container, () => {}, () => {});

// Populate the board so the card-back texture and the generated card face are
// actually visible in the screenshot:
//   - face-down monsters (card back) on both sides
//   - face-down spell/trap
//   - face-up attack-position monster (generated face texture)
//   - face-up defense-position monster (verifies horizontal defense, not the
//     vertical edge-on rendering)
field.animateSetCard(0, 44508094, 2, true); // face-down monster, player 0 mzone slot 2
field.animateSetCard(1, 44508094, 1, true); // face-down monster, player 1 mzone slot 1
field.animateSetCard(0, 5318639, 0, false); // face-down spell/trap, player 0 szone slot 0
field.animateSummon(0, 46986414, 0, 0x1);   // face-up attack monster, player 0 mzone slot 0
field.animateSummon(1, 44508094, 2, 0x4);   // face-up defense monster, player 1 mzone slot 2

window.__smoke = {
  field,
  ready: false,
  canvasAttached: false,
};

// Headless Chrome throttles requestAnimationFrame for background tabs, which
// would leave cards stuck at their animation start positions (floating above
// the board). Snap every in-flight tween to its end state and render once so
// the screenshot shows the settled board. The group treats its argument as an
// absolute clock, so jumping far ahead completes all tweens.
const settle = () => {
  field.tweenGroup.update(performance.now() + 5000);
  field.tweenGroup.update(performance.now() + 5000);
  field.renderer.render(field.scene, field.camera);
};

const markReady = () => {
  // Re-resolve texture.image on each check: TextureLoader attaches it
  // asynchronously, so it may not exist when this script first runs.
  const img = field.cardBackTexture && field.cardBackTexture.image;
  window.__smoke.ready = !!(img && img.complete && img.naturalWidth > 0);
  window.__smoke.canvasAttached = !!(
    field.renderer &&
    field.renderer.domElement &&
    field.renderer.domElement.parentNode
  );
  settle();
  return window.__smoke.ready;
};
const readyPoll = setInterval(() => {
  if (markReady()) clearInterval(readyPoll);
}, 50);
setTimeout(() => clearInterval(readyPoll), 10000);
markReady();
