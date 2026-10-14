/**
 * Card animations + particle FX (P5-4): tween-driven draw/summon/set/attack
 * movements, destroy flourish, shockwave & hit sparks, camera shake. Extracted from
 * field3d.ts; each function takes the owning DuelField3D as `self` and only
 * does `import type` on it (plus ZONE_COORDS from the shared geometry module),
 * so there is no runtime import cycle back into field3d.ts.
 */
import * as THREE from '../../libs/three.module.js';
import * as TWEEN from '../../libs/tween.esm.js';
import { soundManager } from '../audio/sound_manager.ts';
import type { DuelField3D } from './field3d.ts';

export function animateDrawCard(self: DuelField3D, player: number, cardCode: number, cardInfo: any = null, onComplete: (() => void) | null = null): void {
  soundManager.playDraw();
  const mesh = self.handMeshes[player as 0|1].slice().reverse().find((m:any)=>m.userData.cardCode===cardCode&&!m.userData.drawing);
  if (!mesh) { onComplete?.(); return; }
  const target = {x:mesh.position.x,y:mesh.position.y,z:mesh.position.z};
  const deck = self.getZonePosition(player,'deck');
  mesh.userData.drawing=true;
  mesh.position.set(deck.x,0.02,deck.z);
  self.makeTween(mesh.position).to(target,10*1000/60).onComplete(()=>{mesh.userData.drawing=false;onComplete?.();}).start();
}

export function animateSummon(self: DuelField3D, player: number, cardCode: number, slotIndex: number, position = 0x1, cardInfo: any = null, isSpecial = false): any {
  if (isSpecial) soundManager.playSpecialSummon(); else soundManager.playSummon();
  // MSG_MOVE places the card; SUMMONING only adds gframe's DrawSpec reveal.
  const mesh = self.cardsOnField[player].mzone[slotIndex];
  return mesh || self.placeCard(player,'mzone',slotIndex,cardCode,cardInfo,position);
}

export function animateSetCard(self: DuelField3D, player: number, cardCode: number, slotIndex: number, isMonster = false, cardInfo: any = null): any {
  soundManager.playDraw();
  const loc = isMonster?'mzone':'szone';
  return self.cardsOnField[player][loc][slotIndex] || self.placeCard(player,loc,slotIndex,cardCode,cardInfo,isMonster?8:2);
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
  const attacker = self.cardsOnField[attackerPlayer].mzone[attackerSlot];
  if (!attacker) return;
  const target = isDirect || targetSlot === undefined || targetSlot < 0
    ? {x:-0.1,z:attackerPlayer===0?-7:7} : self.getZonePosition(targetPlayer,'mzone',targetSlot);
  if (!target) return;
  const start = attacker.position;
  const dx=target.x-start.x,dz=target.z-start.z,d=Math.hypot(dx,dz)||1;
  const positions:number[]=[],colors:number[]=[];
  // Materials::GenArrow: 20 pairs, parabolic height, widening arrowhead.
  for(let i=0;i<20;i++) {
    const ay=1-i*.1,t=i===19?1:i*.05;
    const width=i===19?0:i===18?.4:.2;
    for(const sign of [1,-1]) {
      positions.push(start.x+dx*t+sign*width*dz/d,i===19?0:4*(1-ay*ay),start.z+dz*t-sign*width*dx/d);
      colors.push(i===19?1:0,1,i===19?1:0);
    }
  }
  const geo=new (THREE as any).BufferGeometry();
  geo.setAttribute('position',new (THREE as any).Float32BufferAttribute(positions,3));
  geo.setAttribute('color',new (THREE as any).Float32BufferAttribute(colors,3));
  const mat=new (THREE as any).MeshBasicMaterial({vertexColors:true,transparent:true,opacity:192/255,side:THREE.DoubleSide});
  const arrow=new (THREE as any).Mesh(geo,mat);arrow.userData.attackArrow=true;self.scene.add(arrow);
  const progress={frame:0};
  const draw=()=>{const sv=(Math.floor(progress.frame)%8)*4,idx:number[]=[];for(let i=0;i<10;i++){const a=sv+i;idx.push(a,a+1,a+2);}geo.setIndex(idx);};draw();
  self.makeTween(progress).to({frame:40},40*1000/60).onUpdate(draw).onComplete(()=>{self.scene.remove(arrow);geo.dispose();mat.dispose();}).start();
}

/**
 * MSG_MOVE's REASON_DESTROY only plays SOUND_DESTROYED in gframe.
 * Movement remains owned by moveCard; no additional particles or scaling.
 */
export function animateDestroy(self: DuelField3D, mesh: any): void {
  if (!mesh) return;
  soundManager.playDestroy();
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
