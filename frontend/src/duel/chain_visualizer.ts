/**
 * 3D Chain Link Visualizer System
 * Renders floating chain link stacks, connecting energy beams, and step-by-step resolution animations.
 */

import * as THREE from '../../libs/three.module.js';
import * as TWEEN from '../../libs/tween.esm.js';
import { soundManager } from '../audio/sound_manager.ts';

/** field3d 提供给连锁可视化依赖的最小面（field3d.ts strict 化后收敛为真实类型） */
interface ChainHost {
  makeTween(obj: { x: number; y: number; z: number }): {
    to(target: Record<string, number>, duration: number): any;
    easing(easing: any): any;
    yoyo(yoyo: boolean): any;
    repeat(count: number): any;
    onComplete(cb: () => void): any;
    start(): void;
  };
  createShockwave(x: number, z: number, color: number): void;
  getZonePosition(player: number, loc: string, seq: number): { x: number; y: number; z: number };
  getCardMesh(player: number, loc: string, seq: number): any | null;
  chainMarkTexture: any;
  numberTexture: any;
}

/** 连锁槽位（duel:chaining 载荷里的 { c, l, s } 转换后的对象） */
interface ChainSlot {
  player?: number;
  loc?: string;
  seq?: number;
}

interface ChainStackItem {
  linkNumber: number;
  cardCode: number;
  slot: ChainSlot | null;
  badgeMesh: any;
  /** 贴卡连锁角标（chain.png + number.png，原版 drawing.cpp:541-562） */
  iconMesh: any | null;
  pinMesh: any | null;
}

export class ChainVisualizer {
  scene: any;
  field3D: ChainHost;
  chainStack: ChainStackItem[];
  chainGroup: any;

  constructor(scene: any, field3D: ChainHost) {
    this.scene = scene;
    this.field3D = field3D;
    this.chainStack = []; // [{ linkNumber, cardCode, slot, mesh, badgeMesh }]
    this.chainGroup = new THREE.Group();
    this.scene.add(this.chainGroup);
  }

  addChainLink(cardCode: number, slot: ChainSlot | null, cardInfo: any = null) {
    const linkNum = this.chainStack.length + 1;
    soundManager.playChain();

    // 原版：连锁序号角标贴在连锁卡上（chain.png 旋转底图叠 number.png
    // 数字，drawing.cpp:541-562 / duelclient.cpp:3009-3011 的 chain_pos）。
    // 浮空徽章只保留给手牌/墓地等无固定槽位的发动（原版对无场卡的
    // chain 不画角标，这里保留徽章作信息提示）。
    let iconMesh = null;
    let pinMesh = null;
    if (slot) {
      pinMesh = this.field3D.getCardMesh(slot.player || 0, slot.loc || 'mzone', slot.seq || 0);
      if (pinMesh) {
        iconMesh = this.createChainLinkSprite(linkNum);
        this.chainGroup.add(iconMesh);
      }
    }

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
      badgeMesh: badge,
      iconMesh,
      pinMesh
    });
  }

  /**
   * 贴卡连锁角标贴图：chain.png 上叠 number.png 的第 N 个数字
   * （number.png 为 5 列×4 行 64×64 网格，drawing.cpp:545-554 的 UV
   * 宽 0.19375 = 62/320、高 0.2421875 = 62/256）。贴图异步加载，
   * 就绪后重绘一次。
   */
  createChainLinkSprite(linkNum: number): any {
    const canvas = document.createElement('canvas');
    canvas.width = 128;
    canvas.height = 128;
    const ctx = canvas.getContext('2d')!;

    const redraw = () => {
      const chainImg = this.field3D.chainMarkTexture?.image;
      const numImg = this.field3D.numberTexture?.image;
      ctx.clearRect(0, 0, 128, 128);
      if (chainImg) ctx.drawImage(chainImg, 8, 8, 112, 112);
      if (numImg) {
        const idx = Math.min(Math.max(linkNum, 1), 20) - 1;
        const sx = (idx % 5) * 64;
        const sy = Math.floor(idx / 5) * 64;
        // 0.6 倍缩放叠在中心（原版 number 0.6 缩放、叠在 chain 上旋转）
        ctx.drawImage(numImg, sx, sy, 64, 64, 34, 34, 60, 60);
      } else {
        ctx.fillStyle = '#fff';
        ctx.font = 'bold 56px sans-serif';
        ctx.textAlign = 'center';
        ctx.textBaseline = 'middle';
        ctx.fillText(String(linkNum), 64, 64);
      }
      texture.needsUpdate = true;
    };

    const texture = new THREE.CanvasTexture(canvas);
    redraw();
    const tryRedraw = () => {
      if (this.field3D.chainMarkTexture?.image && this.field3D.numberTexture?.image) {
        redraw();
      } else {
        requestAnimationFrame(tryRedraw);
      }
    };
    tryRedraw();

    const sprite: any = new THREE.Sprite(new THREE.SpriteMaterial({
      map: texture, transparent: true, depthTest: false,
    }));
    sprite.scale.set(0.9, 0.9, 1);
    sprite.renderOrder = 11;
    return sprite;
  }

  /** 每帧把贴卡角标钉在卡片世界位置上（跟随移动/攻击动画） */
  tick() {
    for (const item of this.chainStack) {
      if (!item.iconMesh || !item.pinMesh) continue;
      const p = item.pinMesh.position;
      // chain_pos：卡的 X+0.35、随连锁次序再抬 0.25（duelclient.cpp:3009）
      item.iconMesh.position.set(p.x + 0.4, p.y + 1.25, p.z + 0.1);
      item.iconMesh.material.rotation += 0.01;
    }
  }

  createChainBadgeMesh(linkNum: number): any {
    const canvas = document.createElement('canvas');
    canvas.width = 256;
    canvas.height = 128;
    const ctx = canvas.getContext('2d')!;

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
    const sprite: any = new THREE.Sprite(mat);
    sprite.scale.set(1.8, 0.9, 1);
    return sprite;
  }

  highlightSolvingLink(linkNum: number) {
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

  removeSolvingLink(linkNum: number) {
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
      if (item.iconMesh) this.chainGroup.remove(item.iconMesh);
      this.chainStack.splice(idx, 1);
    }
  }

  clearChain() {
    this.chainStack.forEach(item => {
      if (item.badgeMesh) this.chainGroup.remove(item.badgeMesh);
      if (item.iconMesh) this.chainGroup.remove(item.iconMesh);
    });
    this.chainStack = [];
  }

  getSlotPosition(slot: ChainSlot | null) {
    if (!slot) return { x: 0, y: 0, z: 0 };
    return this.field3D.getZonePosition(
      slot.player || 0,
      slot.loc || 'mzone',
      slot.seq || 0
    );
  }
}
