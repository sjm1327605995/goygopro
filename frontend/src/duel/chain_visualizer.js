/**
 * 3D Chain Link Visualizer System
 * Renders floating chain link stacks, connecting energy beams, and step-by-step resolution animations.
 */

import * as THREE from '../../libs/three.module.js';
import * as TWEEN from '../../libs/tween.esm.js';
import { soundManager } from '../audio/sound_manager.js';

export class ChainVisualizer {
  constructor(scene, field3D) {
    this.scene = scene;
    this.field3D = field3D;
    this.chainStack = []; // [{ linkNumber, cardCode, slot, mesh, badgeMesh }]
    this.chainGroup = new THREE.Group();
    this.scene.add(this.chainGroup);
  }

  addChainLink(cardCode, slot, cardInfo = null) {
    const linkNum = this.chainStack.length + 1;
    soundManager.playChain();

    // Create 3D Chain Link Badge
    const badge = this.createChainBadgeMesh(linkNum);
    const targetCoord = this.getSlotPosition(slot);

    badge.position.set(targetCoord.x, targetCoord.y + 2.2 + linkNum * 0.4, targetCoord.z);
    badge.scale.set(0.1, 0.1, 0.1);
    this.chainGroup.add(badge);

    // Pop-in animation
    this.field3D.makeTween(badge.scale)
      .to({ x: 1, y: 1, z: 1 }, 250)
      .easing(TWEEN.Easing.Back.Out)
      .start();

    // Pulse effect on targeted slot
    this.field3D.createShockwave(targetCoord.x, targetCoord.z, 0xf59e0b);

    this.chainStack.push({
      linkNumber: linkNum,
      cardCode,
      slot,
      badgeMesh: badge
    });
  }

  createChainBadgeMesh(linkNum) {
    const canvas = document.createElement('canvas');
    canvas.width = 256;
    canvas.height = 128;
    const ctx = canvas.getContext('2d');

    // Badge background
    ctx.fillStyle = 'rgba(15, 23, 42, 0.9)';
    ctx.roundRect(8, 8, canvas.width - 16, canvas.height - 16, 20);
    ctx.fill();

    // Border
    ctx.strokeStyle = '#f59e0b';
    ctx.lineWidth = 8;
    ctx.roundRect(8, 8, canvas.width - 16, canvas.height - 16, 20);
    ctx.stroke();

    // Text
    ctx.fillStyle = '#ffffff';
    ctx.font = 'bold 36px sans-serif';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText(`CHAIN ${linkNum}`, canvas.width / 2, canvas.height / 2);

    const texture = new THREE.CanvasTexture(canvas);
    const mat = new THREE.SpriteMaterial({ map: texture, transparent: true });
    const sprite = new THREE.Sprite(mat);
    sprite.scale.set(1.8, 0.9, 1);
    return sprite;
  }

  highlightSolvingLink(linkNum) {
    const item = this.chainStack.find(c => c.linkNumber === linkNum);
    if (!item || !item.badgeMesh) return;

    soundManager.playSpellActivate();

    // Pulsing highlight
    this.field3D.makeTween(item.badgeMesh.scale)
      .to({ x: 1.5, y: 1.35, z: 1.5 }, 200)
      .yoyo(true)
      .repeat(1)
      .start();
  }

  removeSolvingLink(linkNum) {
    const idx = this.chainStack.findIndex(c => c.linkNumber === linkNum);
    if (idx !== -1) {
      const item = this.chainStack[idx];
      if (item.badgeMesh) {
        this.field3D.makeTween(item.badgeMesh.scale)
          .to({ x: 0.01, y: 0.01, z: 0.01 }, 200)
          .onComplete(() => {
            this.chainGroup.remove(item.badgeMesh);
          })
          .start();
      }
      this.chainStack.splice(idx, 1);
    }
  }

  clearChain() {
    this.chainStack.forEach(item => {
      if (item.badgeMesh) this.chainGroup.remove(item.badgeMesh);
    });
    this.chainStack = [];
  }

  getSlotPosition(slot) {
    if (!slot) return { x: 0, y: 0, z: 0 };
    return this.field3D.getZonePosition(
      slot.player || 0,
      slot.loc || 'mzone',
      slot.seq || 0
    );
  }
}
