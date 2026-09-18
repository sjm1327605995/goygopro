/**
 * Card animations + particle FX (P5-4): tween-driven draw/summon/set/attack/
 * destroy movements, shockwave & hit sparks, camera shake. Extracted from
 * field3d.ts; each function takes the owning DuelField3D as `self` and only
 * does `import type` on it (plus ZONE_COORDS from the shared geometry module),
 * so there is no runtime import cycle back into field3d.ts.
 */
import * as THREE from '../../libs/three.module.js';
import * as TWEEN from '../../libs/tween.esm.js';
import { soundManager } from '../audio/sound_manager.ts';
import { ZONE_COORDS } from './field3d_geometry.ts';
import type { DuelField3D } from './field3d.ts';

export function animateDrawCard(self: DuelField3D, player: number, cardCode: number, cardInfo: any = null, onComplete: (() => void) | null = null): void {
  soundManager.playDraw();
  const cardMesh = self.createCardMesh(cardCode, cardInfo);
  const startCoord = ZONE_COORDS[player].deck;

  cardMesh.position.set(startCoord.x, 0.4, startCoord.z);
  cardMesh.rotation.set(Math.PI, 0, 0);
  self.scene.add(cardMesh);

  const targetPos = player === 0 ? { x: 0, y: 3, z: 8.5 } : { x: 0, y: 3, z: -8.5 };

  self.makeTween(cardMesh.position)
    .to({ x: targetPos.x, y: 4.5, z: (startCoord.z + targetPos.z) / 2 }, 350)
    .easing(TWEEN.Easing.Quadratic.Out)
    .chain(
      self.makeTween(cardMesh.position)
        .to({ x: targetPos.x, y: targetPos.y, z: targetPos.z }, 300)
        .easing(TWEEN.Easing.Quadratic.In)
        .onComplete(() => {
          self.scene.remove(cardMesh);
          const idx = self.cardMeshes.indexOf(cardMesh);
          if (idx > -1) self.cardMeshes.splice(idx, 1);
          if (onComplete) onComplete();
        })
    )
    .start();

  self.makeTween(cardMesh.rotation)
    .to({ x: player === 0 ? 0 : Math.PI, y: 0, z: 0 }, 650)
    .easing(TWEEN.Easing.Cubic.Out)
    .start();
}

export function animateSummon(self: DuelField3D, player: number, cardCode: number, slotIndex: number, position = 0x1, cardInfo: any = null, isSpecial = false): any {
  if (isSpecial) soundManager.playSpecialSummon();
  else soundManager.playSummon();

  const targetCoord = ZONE_COORDS[player].mzone[slotIndex];
  if (!targetCoord) return;

  // The engine often moves the card into the slot (e.g. grave → mzone for a
  // special summon) before announcing the summoning; recycle that mesh
  // instead of stacking a second card in the same slot. The same applies to
  // flip summons, where the set card already occupies the slot.
  const existing = self.cardsOnField[player].mzone[slotIndex];
  if (existing) self.retireMesh(existing, true);

  const cardMesh = self.createCardMesh(cardCode, cardInfo);

  cardMesh.position.set(targetCoord.x, 5.0, targetCoord.z + (player === 0 ? 2 : -2));
  cardMesh.scale.set(0.2, 0.2, 0.2);
  self.scene.add(cardMesh);

  // A card on the board is a thin box lying flat (thickness along y). Turning
  // it sideways for defense means rotating about y (the face normal), NOT z:
  // a z-rotation tips the card up onto its edge, perpendicular to the board.
  const targetRot = self.cardRotationFor(player, position);

  self.makeTween(cardMesh.scale)
    .to({ x: 1, y: 1, z: 1 }, 450)
    .easing(TWEEN.Easing.Back.Out)
    .start();

  self.makeTween(cardMesh.position)
    .to({ x: targetCoord.x, y: targetCoord.y + 0.05, z: targetCoord.z }, 450)
    .easing(TWEEN.Easing.Bounce.Out)
    .onComplete(() => {
      createShockwave(self, targetCoord.x, targetCoord.z, isSpecial ? 0xf59e0b : 0x00d2ff);
    })
    .start();

  self.makeTween(cardMesh.rotation)
    .to(targetRot, 450)
    .easing(TWEEN.Easing.Cubic.Out)
    .start();

  cardMesh.userData.slot = { player, loc: 'mzone', seq: slotIndex };
  cardMesh.userData.positionState = self.positionStateFor(position);
  self.cardsOnField[player].mzone[slotIndex] = cardMesh;
  return cardMesh;
}

export function animateSetCard(self: DuelField3D, player: number, cardCode: number, slotIndex: number, isMonster = false, cardInfo: any = null): any {
  soundManager.playDraw();
  const targetCoord = isMonster
    ? ZONE_COORDS[player].mzone[slotIndex]
    : ZONE_COORDS[player].szone[slotIndex];
  if (!targetCoord) return;

  // Validate the slot before creating anything, and recycle any mesh the
  // preceding MSG_MOVE already placed there.
  const locName = isMonster ? 'mzone' : 'szone';
  const existing = self.cardsOnField[player][locName][slotIndex];
  if (existing) self.retireMesh(existing, true);

  const cardMesh = self.createCardMesh(cardCode, cardInfo);

  cardMesh.position.set(targetCoord.x, 3.5, targetCoord.z);
  // Face-down sets lie with the back up (x=PI); set monsters are set in
  // defense, so their top edge follows the same owner's-right-hand rule as
  // cardRotationFor. Set spells keep their top edge toward the owner.
  cardMesh.rotation.set(
    Math.PI,
    isMonster ? (player === 1 ? Math.PI / 2 : -Math.PI / 2) : 0,
    0
  );
  self.scene.add(cardMesh);

  self.makeTween(cardMesh.position)
    .to({ x: targetCoord.x, y: targetCoord.y + 0.05, z: targetCoord.z }, 350)
    .easing(TWEEN.Easing.Quadratic.Out)
    .start();

  cardMesh.userData.slot = { player, loc: locName, seq: slotIndex };
  cardMesh.userData.positionState = 'FACEDOWN';
  self.cardsOnField[player][locName][slotIndex] = cardMesh;
  self.refreshFieldSpell();
  return cardMesh;
}

export function animateReposition(self: DuelField3D, player: number, locName: any = 'mzone', slotIndex?: any, newPos?: any): void {
  soundManager.playDraw();
  // MSG_POS_CHANGE carries the zone; szone flips matter here (a set field
  // spell activating turns face-up). Older callers may pass the slot as the
  // 2nd argument with a numeric locName-less form — fall back to mzone.
  if (typeof locName !== 'string') {
    slotIndex = locName;
    locName = 'mzone';
  }
  const zone = self.cardsOnField[player] && self.cardsOnField[player][locName];
  const cardMesh = zone ? zone[slotIndex] : self.cardsOnField[player].mzone[slotIndex];
  if (!cardMesh) return;

  cardMesh.userData.positionState = self.positionStateFor(newPos);

  self.makeTween(cardMesh.position)
    .to({ y: 0.8 }, 150)
    .chain(
      self.makeTween(cardMesh.position)
        .to({ y: 0.05 }, 150)
    )
    .start();

  const rot = self.cardRotationFor(player, newPos);
  self.makeTween(cardMesh.rotation)
    .to({ x: rot.x, y: rot.y, z: rot.z }, 300)
    .easing(TWEEN.Easing.Cubic.InOut)
    .start();

  self.refreshFieldSpell();
}

export function animateAttack(self: DuelField3D, attackerPlayer: number, attackerSlot: number, targetPlayer: number, targetSlot: number | undefined, isDirect = false): void {
  soundManager.playAttack();
  const attackerMesh = self.cardsOnField[attackerPlayer].mzone[attackerSlot];
  if (!attackerMesh) return;

  const startPos = { ...attackerMesh.position };
  let targetPos;

  if (isDirect || targetSlot === undefined || targetSlot < 0) {
    targetPos = { x: 0, y: 1.5, z: targetPlayer === 1 ? -6.0 : 6.0 };
  } else {
    const targetCoord = ZONE_COORDS[targetPlayer].mzone[targetSlot];
    targetPos = { x: targetCoord.x, y: 0.5, z: targetCoord.z };
  }

  self.makeTween(attackerMesh.position)
    .to({ y: 1.6 }, 200)
    .easing(TWEEN.Easing.Quadratic.Out)
    .chain(
      self.makeTween(attackerMesh.position)
        .to({ x: targetPos.x, y: targetPos.y + 0.3, z: targetPos.z }, 180)
        .easing(TWEEN.Easing.Exponential.In)
        .onComplete(() => {
          soundManager.playDamage();
          createHitSparks(self, targetPos.x, targetPos.z);
          cameraShake(self, 0.2, 200);
        })
        .chain(
          self.makeTween(attackerMesh.position)
            .to(startPos, 300)
            .easing(TWEEN.Easing.Cubic.Out)
        )
    )
    .start();
}

export function animateDestroy(self: DuelField3D, player: number, loc: string, slotIndex: number): void {
  soundManager.playDamage();
  const cardMesh = self.cardsOnField[player][loc][slotIndex];
  if (!cardMesh) return;

  self.cardsOnField[player][loc][slotIndex] = null;
  const graveCoord = ZONE_COORDS[player].grave;

  self.makeTween(cardMesh.position)
    .to({ y: 2.0 }, 150)
    .chain(
      self.makeTween(cardMesh.position)
        .to({ x: graveCoord.x, y: 0.2, z: graveCoord.z }, 350)
        .easing(TWEEN.Easing.Cubic.In)
        .onComplete(() => {
          self.scene.remove(cardMesh);
          const idx = self.cardMeshes.indexOf(cardMesh);
          if (idx > -1) self.cardMeshes.splice(idx, 1);
        })
    )
    .start();

  self.makeTween(cardMesh.rotation)
    .to({ x: 0, y: 0, z: Math.PI }, 350)
    .start();

  self.refreshFieldSpell();
}

export function createShockwave(self: DuelField3D, x: number, z: number, colorHex = 0x00d2ff): void {
  const ringGeo = new (THREE as any).RingGeometry(0.1, 0.4, 32);
  const ringMat = new (THREE as any).MeshBasicMaterial({
    color: colorHex,
    transparent: true,
    opacity: 0.9,
    side: THREE.DoubleSide
  });
  const ringMesh = new (THREE as any).Mesh(ringGeo, ringMat);
  ringMesh.rotation.x = -Math.PI / 2;
  ringMesh.position.set(x, 0.03, z);
  self.scene.add(ringMesh);

  self.makeTween(ringMesh.scale)
    .to({ x: 6, y: 6, z: 6 }, 400)
    .easing(TWEEN.Easing.Quadratic.Out)
    .start();

  self.makeTween(ringMat)
    .to({ opacity: 0 }, 400)
    .easing(TWEEN.Easing.Quadratic.Out)
    .onComplete(() => self.scene.remove(ringMesh))
    .start();
}

export function createHitSparks(self: DuelField3D, x: number, z: number): void {
  const sparkCount = 20;
  const pGeo = new (THREE as any).BufferGeometry();
  const positions = new Float32Array(sparkCount * 3);
  const velocities: { vx: number; vy: number; vz: number }[] = [];

  for (let i = 0; i < sparkCount; i++) {
    positions[i * 3] = x;
    positions[i * 3 + 1] = 0.5;
    positions[i * 3 + 2] = z;
    velocities.push({
      vx: (Math.random() - 0.5) * 6,
      vy: Math.random() * 5 + 2,
      vz: (Math.random() - 0.5) * 6
    });
  }

  pGeo.setAttribute('position', new (THREE as any).BufferAttribute(positions, 3));
  const pMat = new (THREE as any).PointsMaterial({
    color: 0xf59e0b,
    size: 0.25,
    transparent: true,
    opacity: 1
  });

  const pMesh = new (THREE as any).Points(pGeo, pMat);
  self.scene.add(pMesh);

  let progress = { t: 0 };
  self.makeTween(progress)
    .to({ t: 1 }, 350)
    .onUpdate(() => {
      const posAttr = pMesh.geometry.attributes.position;
      for (let i = 0; i < sparkCount; i++) {
        posAttr.array[i * 3] += velocities[i].vx * 0.02;
        posAttr.array[i * 3 + 1] += velocities[i].vy * 0.02 - 0.05;
        posAttr.array[i * 3 + 2] += velocities[i].vz * 0.02;
      }
      posAttr.needsUpdate = true;
      pMat.opacity = 1 - progress.t;
    })
    .onComplete(() => self.scene.remove(pMesh))
    .start();
}

export function cameraShake(self: DuelField3D, intensity = 0.25, duration = 200): void {
  const startCamPos = { x: 0, y: 13.5, z: 12.0 };
  const startTime = performance.now();

  const shakeInterval = setInterval(() => {
    const elapsed = performance.now() - startTime;
    if (elapsed >= duration) {
      clearInterval(shakeInterval);
      self.camera.position.set(startCamPos.x, startCamPos.y, startCamPos.z);
      return;
    }
    const damping = 1 - elapsed / duration;
    self.camera.position.x = startCamPos.x + (Math.random() - 0.5) * intensity * damping;
    self.camera.position.y = startCamPos.y + (Math.random() - 0.5) * intensity * damping;
  }, 16);
}