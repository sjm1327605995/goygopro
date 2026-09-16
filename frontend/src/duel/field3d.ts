/**
 * Three.js 3D Duel Field System
 * Complete 3D board rendering, card animations, raycasting, chain visualizer, and particle FX.
 */

import * as THREE from '../../libs/three.module.js';
import * as TWEEN from '../../libs/tween.esm.js';
import { soundManager } from '../audio/sound_manager.ts';
import { settingsStore } from '../domain/settings.ts';
import { ChainVisualizer } from './chain_visualizer.ts';
import {
  TYPE_SPELL, TYPE_TRAP, TYPE_EFFECT, TYPE_FUSION,
  TYPE_SYNCHRO, TYPE_XYZ, TYPE_LINK,
  LOC_NAMES,
} from '../domain/constants.ts';

// Card dimensions in 3D world units
export const CARD_WIDTH = 1.4;
export const CARD_HEIGHT = 2.0;
export const CARD_DEPTH = 0.03;

// Field Zone coordinate definitions
// 区域坐标 = YGOPro 原版布局（source/ygopro/gframe/materials.cpp 的
// vField* 四边形，权威依据），换算 world = ((qx-4)*2, qy*2)：
// 原版地垫四边形是 x -1..9 / y -4..4 的 10×8，中心 (4,0) 平移到世界原点，
// 整体 ×2 使地垫铺满 20×16 的可视范围。卡片尺寸对应原版 vCardFront
// 0.7×1.0 ×2 = CARD_WIDTH×CARD_HEIGHT。mzone 7 槽（5 主怪区 + 两个共享
// 的额外怪区）、szone 8 槽（5 魔法陷阱 + 场地 + 两侧灵摆），与引擎
// MR2020 区域编号一致。
export const ZONE_COORDS: Record<number, Record<string, any>> = {
  // Player 0（z 正方向，靠近摄像机；对应原版画面下半边）
  0: {
    mzone: [
      { x: -4.5, y: 0.02, z: 2.8 },
      { x: -2.3, y: 0.02, z: 2.8 },
      { x: -0.1, y: 0.02, z: 2.8 },
      { x: 2.1,  y: 0.02, z: 2.8 },
      { x: 4.3,  y: 0.02, z: 2.8 },
      // seq 5/6 = 共享的两格额外怪区（原版 vFieldMzone[0][5]/[6]）
      { x: -2.3, y: 0.02, z: 0.0 },
      { x: 2.1,  y: 0.02, z: 0.0 }
    ],
    szone: [
      { x: -4.5, y: 0.02, z: 5.2 },
      { x: -2.3, y: 0.02, z: 5.2 },
      { x: -0.1, y: 0.02, z: 5.2 },
      { x: 2.1,  y: 0.02, z: 5.2 },
      { x: 4.3,  y: 0.02, z: 5.2 },
      // seq 5 = 场地魔法区；seq 6/7 = 两侧灵摆区（vFieldSzone[0][5..7]）
      { x: -6.8, y: 0.02, z: 1.4 },
      { x: -6.8, y: 0.02, z: 4.0 },
      { x: 6.6,  y: 0.02, z: 4.0 }
    ],
    field:     { x: -6.8, y: 0.02, z: 1.4 },
    grave:     { x: 6.6,  y: 0.02, z: 1.4 },
    banish:    { x: 8.6,  y: 0.02, z: 1.4 },
    deck:      { x: 6.6,  y: 0.02, z: 6.6 },
    extra:     { x: -6.8, y: 0.02, z: 6.6 },
    emz_left:  { x: -2.3, y: 0.02, z: 0.0 },
    emz_right: { x: 2.1,  y: 0.02, z: 0.0 }
  },
  // Player 1（关于原点中心对称，即原版画面上半边）。额外怪区两格由双方
  // 共享，seq 5/6 的左右顺序随玩家视角镜像。
  1: {
    mzone: [
      { x: 4.3,  y: 0.02, z: -2.8 },
      { x: 2.1,  y: 0.02, z: -2.8 },
      { x: -0.1, y: 0.02, z: -2.8 },
      { x: -2.3, y: 0.02, z: -2.8 },
      { x: -4.5, y: 0.02, z: -2.8 },
      { x: 2.1,  y: 0.02, z: 0.0 },
      { x: -2.3, y: 0.02, z: 0.0 }
    ],
    szone: [
      { x: 4.3,  y: 0.02, z: -5.2 },
      { x: 2.1,  y: 0.02, z: -5.2 },
      { x: -0.1, y: 0.02, z: -5.2 },
      { x: -2.3, y: 0.02, z: -5.2 },
      { x: -4.5, y: 0.02, z: -5.2 },
      // seq 5 = 场地魔法区；seq 6/7 = 两侧灵摆区
      // （vFieldSzone[1][5..7] 原值；原版左右两半有 0.1 的固有不对称）
      { x: 6.6,  y: 0.02, z: -1.4 },
      { x: 6.6,  y: 0.02, z: -4.0 },
      { x: -6.8, y: 0.02, z: -4.0 }
    ],
    field:     { x: 6.6,  y: 0.02, z: -1.4 },
    grave:     { x: -6.8, y: 0.02, z: -1.4 },
    banish:    { x: -8.8, y: 0.02, z: -1.4 },
    deck:      { x: -6.8, y: 0.02, z: -6.6 },
    extra:     { x: 6.6,  y: 0.02, z: -6.6 },
    emz_left:  { x: -2.3, y: 0.02, z: 0.0 },
    emz_right: { x: 2.1,  y: 0.02, z: 0.0 }
  }
};

export class DuelField3D {
  container: HTMLElement;
  onCardInspect: (code: number, info?: any) => void;
  onCardClick: (x: number, y: number, userData: any) => void;

  scene: any = null;
  camera: any = null;
  renderer: any = null;
  raycaster: any = new (THREE as any).Raycaster();
  mouse: any = new (THREE as any).Vector2();

  cardsOnField: Record<number, Record<string, any>> = {
    0: { mzone: [], szone: [], grave: [], banish: [], deck: [], extra: [] },
    1: { mzone: [], szone: [], grave: [], banish: [], deck: [], extra: [] }
  };
  cardMeshes: any[] = [];
  // 对手手牌的背面小卡（原版画面上缘一排微重叠的卡背；自己的手牌在 2D 手牌坞）
  opponentHandMeshes: any[] = [];
  hoveredCard: any = null;
  textureLoader: any = new (THREE as any).TextureLoader();
  cardBackTexture: any = null;
  cardTextureCache = new Map<number, any>();
  // Optional async (code) => Promise<dataURL|null> set by the host; when a
  // picture arrives the cached canvas texture is redrawn in place so every
  // mesh sharing it picks the art up on the next frame.
  cardImageProvider: ((code: number) => Promise<{ url: string; full: boolean } | null>) | null = null;
  // Field spell art as the board background (the YGOPro field.png effect).
  fieldSpellPlane: any = null;
  fieldSpellTexture: any = null;
  fieldSpellCode = 0;      // code currently displayed (0 = plain board)
  fieldSpellPending = 0;   // code of the in-flight art load, 0 if none
  chainVisualizer: ChainVisualizer | null = null;
  // 装备/目标关系线（duel:equip / card_target / cancel_target / unequip）
  relationLines: any[] = [];
  disposed = false;
  matTexture: any = null;
  transparentMatTexture: any = null;
  matMesh: any = null;
  _onResize: () => void = () => {};
  _onMouseMove: (e: MouseEvent) => void = () => {};
  _onClick: (e: MouseEvent) => void = () => {};
  // 最近一次鼠标的屏幕像素坐标（stTip 悬浮提示跟随用）
  _lastClientX = 0;
  _lastClientY = 0;
  // tween.js v25 no longer auto-registers `new Tween(obj)` with the global
  // group, so all card animations must live in an explicit group that
  // animate() drives. Without this the tweens are orphaned and cards stay
  // frozen at their animation start positions (floating above the board).
  tweenGroup = new TWEEN.Group();

  constructor(
    containerElement: HTMLElement,
    onCardInspect: (code: number, info?: any) => void,
    onCardClick: (x: number, y: number, userData: any) => void,
  ) {
    this.container = containerElement;
    this.onCardInspect = onCardInspect;
    this.onCardClick = onCardClick;

    this.initScene();
    this.initLights();
    this.initField();
    this.chainVisualizer = new ChainVisualizer(this.scene, this);
    this.initEvents();
    this.animate();
  }

  initScene(): void {
    this.scene = new (THREE as any).Scene();
    // YGOPro 的全屏背景图（gframe/game.cpp:1040 DrawBackImage(imgBackGround)）
    this.scene.background = this.textureLoader.load('textures/bg.jpg', (tex: any) => {
      tex.colorSpace = THREE.SRGBColorSpace;
    });
    // 原版无雾（gframe 不设 scene fog），雾会把远端场地压暗
    // this.scene.fog = new (THREE as any).FogExp2(0x0a0e17, 0.008);

    const width = this.container.clientWidth || window.innerWidth;
    const height = this.container.clientHeight || window.innerHeight;

    this.camera = new (THREE as any).PerspectiveCamera(45, width / height, 0.1, 100);
    this.camera.position.set(0, 14.5, 12.5);
    this.camera.lookAt(0, 0, 0.3);
    this.applyViewOffset();

    this.renderer = new (THREE as any).WebGLRenderer({ antialias: true, alpha: true });
    this.renderer.setSize(width, height);
    this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
    this.renderer.shadowMap.enabled = true;
    this.renderer.shadowMap.type = THREE.PCFSoftShadowMap;

    this.container.appendChild(this.renderer.domElement);
    this.cardBackTexture = this.textureLoader.load('textures/cover.jpg');
  }

  initLights(): void {
    // 原版 gframe 只有一盏全亮环境光（game.cpp:999
    // setAmbientLight(SColorf(1,1,1))），没有彩色点光/雾/辉光——那些是现代
    // 游戏的观感，不是 YGOPro 的。保留一盏弱白方向光只为投影服务。
    const ambientLight = new (THREE as any).AmbientLight(0xffffff, 1.0);
    this.scene.add(ambientLight);

    const dirLight = new (THREE as any).DirectionalLight(0xffffff, 0.7);
    dirLight.position.set(0, 20, 10);
    dirLight.castShadow = true;
    dirLight.shadow.mapSize.width = 2048;
    dirLight.shadow.mapSize.height = 2048;
    this.scene.add(dirLight);
  }

  initField(): void {
    // YGOPro 的场地盘面 = 一张贴图（gframe/image_manager.cpp:41-43 imgField，
    // drawing.cpp:183 以贴图四边形 vField 直接画 10×8 的 field2.png）。
    // 这里用同一个 field2.png 作为半透明 overlay 铺在 y=0，卡牌投其上；
    // 区域框线就是贴图自带的白描边，不再额外画槽位网格。
    const matTex = this.textureLoader.load('textures/field2.png', (tex: any) => {
      tex.colorSpace = THREE.SRGBColorSpace;
    });
    this.matTexture = matTex;
    this.transparentMatTexture = this.textureLoader.load('textures/field-transparent2.png', (tex: any) => {
      tex.colorSpace = THREE.SRGBColorSpace;
    });

    const matGeo = new (THREE as any).PlaneGeometry(20, 16);
    const matMat = new (THREE as any).MeshBasicMaterial({ map: matTex, transparent: true });
    this.matMesh = new (THREE as any).Mesh(matGeo, matMat);
    this.matMesh.rotation.x = -Math.PI / 2;
    // 贴图 y=0 边对应盘面远端（P1 侧），PlaneGeometry 顶边在 −z，方向一致
    this.matMesh.position.set(0, 0, 0);
    this.scene.add(this.matMesh);

    // 只接收阴影的透明地面：卡牌影子落在背景上，不遮挡盘面。
    // 原版没有影子，这里留一层很淡的接触阴影增加立体感。
    const shadowGeo = new (THREE as any).PlaneGeometry(60, 60);
    const shadowMat = new (THREE as any).ShadowMaterial({ opacity: 0.16 });
    const shadowCatcher = new (THREE as any).Mesh(shadowGeo, shadowMat);
    shadowCatcher.rotation.x = -Math.PI / 2;
    shadowCatcher.position.y = -0.02;
    shadowCatcher.receiveShadow = true;
    this.scene.add(shadowCatcher);
  }

  // Single source of truth for zone coordinates. External renderers (e.g. the
  // ChainVisualizer) call this instead of keeping their own copy of ZONE_COORDS
  // so the layout cannot drift out of sync.
  getZonePosition(player: number, loc: string, seq = 0): { x: number; y: number; z: number } {
    const zone = ZONE_COORDS[player] && ZONE_COORDS[player][loc];
    if (!zone) return { x: 0, y: 0.02, z: 0 };
    if (Array.isArray(zone)) {
      const pos = zone[seq];
      return pos ? { ...pos } : { x: 0, y: 0.02, z: 0 };
    }
    return { ...zone };
  }

  // Board-facing rotation for a card in `position` (ocgcore position bits:
  // POS_DEFENSE = 0xc, POS_FACEDOWN = 0xa). The face lies on +y so x=PI flips
  // it face-down; y spins the card within the board plane (a z-rotation would
  // tip it up on its edge). Match the original client_field.cpp: p0 ATK top
  // edge points -z (away), p1 ATK is spun PI, and defense turns the top edge
  // toward the owner's right hand (p0 → -PI/2, p1 → +PI/2), mirroring
  // gframe's Z=∓PI/2.
  cardRotationFor(player: number, position: number): { x: number; y: number; z: number } {
    const isDef = (position & 0xc) !== 0;
    const isFaceDown = (position & 0xa) !== 0;
    return {
      x: isFaceDown ? Math.PI : 0,
      y: isDef
        ? (player === 1 ? Math.PI / 2 : -Math.PI / 2)
        : (player === 1 ? Math.PI : 0),
      z: 0
    };
  }

  // Ocgecore position bits → symbolic state. Used both for the card's userData
  // (field-spell background tracks face-up cards in the field zone) and for
  // choosing the mesh rotation.
  positionStateFor(position: number): string {
    const isFaceDown = (position & 0xa) !== 0;
    const isDef = (position & 0xc) !== 0;
    return isFaceDown ? 'FACEDOWN' : (isDef ? 'DEF' : 'ATK');
  }

  // ---------------------------------------------------------------
  // Field spell background (YGOPro field.png effect): whenever a face-up
  // field spell rests in szone seq 5, its art becomes a translucent
  // backdrop for the whole board; when it leaves or is flipped down the
  // backdrop fades away. The applier reaches this too, since it drives the
  // same placeCard/moveCard entry points.
  // ---------------------------------------------------------------

  refreshFieldSpell(): void {
    if (!this.cardsOnField) return;
    // draw_field_spell=0（原版 system.conf 开关）时不显示场地魔法背景
    if (settingsStore.get('draw_field_spell') === 0) {
      if (this.fieldSpellCode !== 0) {
        this.fieldSpellCode = 0;
        this.hideFieldSpell();
      }
      return;
    }
    let code = 0;
    for (const player of [0, 1]) {
      const zone = this.cardsOnField[player] && this.cardsOnField[player].szone;
      const mesh = zone ? zone[5] : null;
      if (mesh && mesh.userData.cardCode && mesh.userData.positionState !== 'FACEDOWN') {
        code = mesh.userData.cardCode;
        break;
      }
    }
    if (code === this.fieldSpellCode) return;
    this.fieldSpellCode = code;
    if (code === 0) {
      this.hideFieldSpell();
    } else {
      this.showFieldSpell(code);
    }
  }

  showFieldSpell(code: number): void {
    this.fieldSpellPending = code;
    const wantsArt = code;
    if (this.cardImageProvider) {
      this.cardImageProvider(code).then((pic) => {
        // The board state may have moved on while the art was in flight.
        if (!pic || !pic.url || this.disposed || this.fieldSpellPending !== wantsArt) return;
        const img = new Image();
        img.onload = () => {
          if (this.disposed || this.fieldSpellPending !== wantsArt) return;
          const tex = new (THREE as any).CanvasTexture(img);
          tex.colorSpace = THREE.SRGBColorSpace;
          this.applyFieldSpellTexture(tex, wantsArt);
        };
        img.src = pic.url;
      }).catch(() => {});
    }
  }

  applyFieldSpellTexture(tex: any, code: number): void {
    if (this.fieldSpellTexture) this.fieldSpellTexture.dispose();
    this.fieldSpellTexture = tex;
    if (!this.fieldSpellPlane) {
      // vFieldSpell (materials.cpp) = x 1.2..6.7 / y −3.2..3.2 → ×2 = 11×12.8，
      // 中心 (−0.1, 0)（world = ((qx−4)×2, qy×2)）
      const geo = new (THREE as any).PlaneGeometry(11, 12.8);
      const mat = new (THREE as any).MeshBasicMaterial({
        map: tex,
        transparent: true,
        opacity: 0,
        depthWrite: false,
        side: THREE.DoubleSide
      });
      this.fieldSpellPlane = new (THREE as any).Mesh(geo, mat);
      this.fieldSpellPlane.rotation.x = -Math.PI / 2;
      this.fieldSpellPlane.position.set(-0.1, 0.01, 0);
      this.scene.add(this.fieldSpellPlane);
    } else {
      this.fieldSpellPlane.material.map = tex;
    }
    // 有表侧场地魔法时，原版把盘面贴图换成 field-transparent2.png
    // （drawing.cpp:183 drawField ? tFieldTransparent : tField）
    if (this.matMesh && this.transparentMatTexture) {
      this.matMesh.material.map = this.transparentMatTexture;
    }
    this.fieldSpellPlane.material.opacity = 0;
    this.makeTween(this.fieldSpellPlane.material)
      .to({ opacity: 0.55 }, 500)
      .easing(TWEEN.Easing.Quadratic.Out)
      .start();
  }

  hideFieldSpell(): void {
    this.fieldSpellPending = 0;
    if (this.matMesh && this.matTexture) {
      this.matMesh.material.map = this.matTexture;
    }
    const plane = this.fieldSpellPlane;
    if (!plane) return;
    this.makeTween(plane.material)
      .to({ opacity: 0 }, 400)
      .easing(TWEEN.Easing.Quadratic.In)
      .onComplete(() => {
        // A new field spell may have taken over while we faded out.
        if (this.fieldSpellPlane === plane && this.fieldSpellCode === 0) {
          this.scene.remove(plane);
          this.fieldSpellPlane = null;
        }
      })
      .start();
  }

  // Set spells turn face-up when activated (MSG_CHAINING anchors on them);
  // flips the szone mesh and re-evaluates the field-spell background.
  flipSzoneCard(player: number, seq: number): void {
    const zone = this.cardsOnField[player] && this.cardsOnField[player].szone;
    const mesh = zone ? zone[seq] : null;
    if (!mesh) return;
    mesh.userData.positionState = 'FACEUP';
    this.makeTween(mesh.rotation)
      .to({ x: 0 }, 200)
      .easing(TWEEN.Easing.Cubic.Out)
      .start();
    this.refreshFieldSpell();
  }

  // Creates a tween registered with this field's animation group. tween.js v25
  // requires explicit group membership; the global TWEEN.update() no longer
  // picks up `new Tween(obj)` created without a group.
  makeTween(obj: any): any {
    const tween = new TWEEN.Tween(obj);
    if (settingsStore.get('quick_animation')) {
      // 快速动画（原版 chkQuickAnimation）：时长减半（TimeScale 包装更精确，
      // 但 tween.js v25 的 group update 不支持逐组 timeScale，直接改 duration）
      const origTo = tween.to.bind(tween);
      tween.to = (props: any, duration?: number) =>
        origTo(props, duration !== undefined ? Math.max(50, Math.round(duration / 2)) : duration);
    }
    this.tweenGroup.add(tween);
    return tween;
  }

  createCardMesh(cardCode: number, cardInfo: any = null): any {
    const geo = new (THREE as any).BoxGeometry(CARD_WIDTH, CARD_DEPTH, CARD_HEIGHT);
    const borderMat = new (THREE as any).MeshStandardMaterial({ color: 0x111827 });
    const frontMat = new (THREE as any).MeshStandardMaterial({
      map: this.generateCardTexture(cardCode, cardInfo),
      roughness: 0.4,
      metalness: 0.1
    });
    const backMat = new (THREE as any).MeshStandardMaterial({
      map: this.cardBackTexture,
      roughness: 0.4,
      metalness: 0.1
    });

    const materials = [borderMat, borderMat, frontMat, backMat, borderMat, borderMat];
    const cardMesh = new (THREE as any).Mesh(geo, materials);
    cardMesh.castShadow = true;
    cardMesh.receiveShadow = true;
    cardMesh.userData = {
      cardCode,
      cardInfo,
      originalY: 0.05,
      positionState: 'ATK',
      slot: null
    };

    this.cardMeshes.push(cardMesh);
    return cardMesh;
  }

  generateCardTexture(cardCode: number, cardInfo: any): any {
    if (this.cardTextureCache.has(cardCode)) {
      return this.cardTextureCache.get(cardCode);
    }

    const canvas = document.createElement('canvas');
    canvas.width = 512;
    canvas.height = 744;
    const ctx = canvas.getContext('2d')!;

    let frameColor = '#c28544';
    if (cardInfo) {
      // 卡框配色按卡片类型位（TYPE_*，domain/constants.ts）
      if (cardInfo.type & TYPE_SPELL) frameColor = '#1d9e74';
      else if (cardInfo.type & TYPE_TRAP) frameColor = '#bc1c6c';
      else if (cardInfo.type & TYPE_EFFECT) frameColor = '#a05c28';
      else if (cardInfo.type & TYPE_FUSION) frameColor = '#732c86';
      else if (cardInfo.type & TYPE_SYNCHRO) frameColor = '#e5e7eb';
      else if (cardInfo.type & TYPE_XYZ) frameColor = '#1e1e1e';
      else if (cardInfo.type & TYPE_LINK) frameColor = '#0284c7';
    }

    ctx.fillStyle = frameColor;
    ctx.fillRect(0, 0, canvas.width, canvas.height);

    ctx.strokeStyle = '#000000';
    ctx.lineWidth = 12;
    ctx.strokeRect(6, 6, canvas.width - 12, canvas.height - 12);

    ctx.fillStyle = '#0f172a';
    ctx.fillRect(40, 80, canvas.width - 80, 360);
    ctx.strokeStyle = '#334155';
    ctx.lineWidth = 4;
    ctx.strokeRect(40, 80, canvas.width - 80, 360);

    ctx.fillStyle = '#38bdf8';
    ctx.font = 'bold 32px sans-serif';
    ctx.textAlign = 'center';
    ctx.fillText(cardInfo ? cardInfo.name.substring(0, 16) : `Card #${cardCode}`, 256, 260);

    ctx.fillStyle = '#ffffff';
    ctx.font = 'bold 24px sans-serif';
    ctx.textAlign = 'left';
    ctx.fillText(cardInfo ? cardInfo.name : `Card ${cardCode}`, 44, 52);

    if (cardInfo && (cardInfo.attack !== undefined || cardInfo.level)) {
      ctx.fillStyle = '#1e293b';
      ctx.fillRect(40, 480, canvas.width - 80, 220);
      ctx.strokeStyle = '#475569';
      ctx.strokeRect(40, 480, canvas.width - 80, 220);

      if (cardInfo.level) {
        ctx.fillStyle = '#f59e0b';
        ctx.font = 'bold 20px sans-serif';
        let starStr = '★ '.repeat(Math.min(cardInfo.level, 12));
        ctx.fillText(starStr, 50, 515);
      }

      ctx.fillStyle = '#ffffff';
      ctx.font = 'bold 24px sans-serif';
      ctx.fillText(`ATK / ${cardInfo.attack}  DEF / ${cardInfo.defense}`, 50, 680);

      if (cardInfo.desc) {
        ctx.fillStyle = '#94a3b8';
        ctx.font = '16px sans-serif';
        const words = cardInfo.desc.substring(0, 100) + '...';
        ctx.fillText(words.substring(0, 38), 50, 555);
        ctx.fillText(words.substring(38, 76), 50, 585);
      }
    }

    const texture = new (THREE as any).CanvasTexture(canvas);
    this.cardTextureCache.set(cardCode, texture);
    // Real card art (if available) is layered over the procedural drawing
    // asynchronously — the card is playable immediately either way. The
    // provider resolves to { url, full }: a "full" picture is a complete card
    // face (CDN) drawn across the whole canvas; raw local art (pics/<code>) is
    // framed inside the art box of the procedural face.
    if (this.cardImageProvider) {
      this.cardImageProvider(cardCode).then((pic) => {
        if (!pic || !pic.url) return;
        const img = new Image();
        img.onload = () => {
          const tex = this.cardTextureCache.get(cardCode);
          if (!tex || tex.image !== canvas) return;
          const ctx = canvas.getContext('2d')!;
          if (pic.full) {
            // Complete card face: cover-fit the whole canvas (aspect matches
            // a real card, 512x744 ≈ 59x86mm, so distortion is negligible).
            ctx.fillStyle = '#000000';
            ctx.fillRect(0, 0, canvas.width, canvas.height);
            const scale = Math.max(canvas.width / img.width, canvas.height / img.height);
            const dw = img.width * scale, dh = img.height * scale;
            ctx.drawImage(img, (canvas.width - dw) / 2, (canvas.height - dh) / 2, dw, dh);
          } else {
            const ax = 40, ay = 80, aw = canvas.width - 80, ah = 360;
            ctx.fillStyle = '#0b1120';
            ctx.fillRect(ax, ay, aw, ah);
            const scale = Math.min(aw / img.width, ah / img.height);
            const dw = img.width * scale, dh = img.height * scale;
            ctx.drawImage(img, ax + (aw - dw) / 2, ay + (ah - dh) / 2, dw, dh);
          }
          tex.needsUpdate = true;
        };
        img.src = pic.url;
      }).catch(() => {});
    }
    return texture;
  }

  animateDrawCard(player: number, cardCode: number, cardInfo: any = null, onComplete: (() => void) | null = null): void {
    soundManager.playDraw();
    const cardMesh = this.createCardMesh(cardCode, cardInfo);
    const startCoord = ZONE_COORDS[player].deck;

    cardMesh.position.set(startCoord.x, 0.4, startCoord.z);
    cardMesh.rotation.set(Math.PI, 0, 0);
    this.scene.add(cardMesh);

    const targetPos = player === 0 ? { x: 0, y: 3, z: 8.5 } : { x: 0, y: 3, z: -8.5 };

    this.makeTween(cardMesh.position)
      .to({ x: targetPos.x, y: 4.5, z: (startCoord.z + targetPos.z) / 2 }, 350)
      .easing(TWEEN.Easing.Quadratic.Out)
      .chain(
        this.makeTween(cardMesh.position)
          .to({ x: targetPos.x, y: targetPos.y, z: targetPos.z }, 300)
          .easing(TWEEN.Easing.Quadratic.In)
          .onComplete(() => {
            this.scene.remove(cardMesh);
            const idx = this.cardMeshes.indexOf(cardMesh);
            if (idx > -1) this.cardMeshes.splice(idx, 1);
            if (onComplete) onComplete();
          })
      )
      .start();

    this.makeTween(cardMesh.rotation)
      .to({ x: player === 0 ? 0 : Math.PI, y: 0, z: 0 }, 650)
      .easing(TWEEN.Easing.Cubic.Out)
      .start();
  }

  animateSummon(player: number, cardCode: number, slotIndex: number, position = 0x1, cardInfo: any = null, isSpecial = false): any {
    if (isSpecial) soundManager.playSpecialSummon();
    else soundManager.playSummon();

    const targetCoord = ZONE_COORDS[player].mzone[slotIndex];
    if (!targetCoord) return;

    // The engine often moves the card into the slot (e.g. grave → mzone for a
    // special summon) before announcing the summoning; recycle that mesh
    // instead of stacking a second card in the same slot. The same applies to
    // flip summons, where the set card already occupies the slot.
    const existing = this.cardsOnField[player].mzone[slotIndex];
    if (existing) this.retireMesh(existing, true);

    const cardMesh = this.createCardMesh(cardCode, cardInfo);

    cardMesh.position.set(targetCoord.x, 5.0, targetCoord.z + (player === 0 ? 2 : -2));
    cardMesh.scale.set(0.2, 0.2, 0.2);
    this.scene.add(cardMesh);

    // A card on the board is a thin box lying flat (thickness along y). Turning
    // it sideways for defense means rotating about y (the face normal), NOT z:
    // a z-rotation tips the card up onto its edge, perpendicular to the board.
    const targetRot = this.cardRotationFor(player, position);

    this.makeTween(cardMesh.scale)
      .to({ x: 1, y: 1, z: 1 }, 450)
      .easing(TWEEN.Easing.Back.Out)
      .start();

    this.makeTween(cardMesh.position)
      .to({ x: targetCoord.x, y: targetCoord.y + 0.05, z: targetCoord.z }, 450)
      .easing(TWEEN.Easing.Bounce.Out)
      .onComplete(() => {
        this.createShockwave(targetCoord.x, targetCoord.z, isSpecial ? 0xf59e0b : 0x00d2ff);
      })
      .start();

    this.makeTween(cardMesh.rotation)
      .to(targetRot, 450)
      .easing(TWEEN.Easing.Cubic.Out)
      .start();

    cardMesh.userData.slot = { player, loc: 'mzone', seq: slotIndex };
    cardMesh.userData.positionState = this.positionStateFor(position);
    this.cardsOnField[player].mzone[slotIndex] = cardMesh;
    return cardMesh;
  }

  animateSetCard(player: number, cardCode: number, slotIndex: number, isMonster = false, cardInfo: any = null): any {
    soundManager.playDraw();
    const targetCoord = isMonster
      ? ZONE_COORDS[player].mzone[slotIndex]
      : ZONE_COORDS[player].szone[slotIndex];
    if (!targetCoord) return;

    // Validate the slot before creating anything, and recycle any mesh the
    // preceding MSG_MOVE already placed there.
    const locName = isMonster ? 'mzone' : 'szone';
    const existing = this.cardsOnField[player][locName][slotIndex];
    if (existing) this.retireMesh(existing, true);

    const cardMesh = this.createCardMesh(cardCode, cardInfo);

    cardMesh.position.set(targetCoord.x, 3.5, targetCoord.z);
    // Face-down sets lie with the back up (x=PI); set monsters are set in
    // defense, so their top edge follows the same owner's-right-hand rule as
    // cardRotationFor. Set spells keep their top edge toward the owner.
    cardMesh.rotation.set(
      Math.PI,
      isMonster ? (player === 1 ? Math.PI / 2 : -Math.PI / 2) : 0,
      0
    );
    this.scene.add(cardMesh);

    this.makeTween(cardMesh.position)
      .to({ x: targetCoord.x, y: targetCoord.y + 0.05, z: targetCoord.z }, 350)
      .easing(TWEEN.Easing.Quadratic.Out)
      .start();

    cardMesh.userData.slot = { player, loc: locName, seq: slotIndex };
    cardMesh.userData.positionState = 'FACEDOWN';
    this.cardsOnField[player][locName][slotIndex] = cardMesh;
    this.refreshFieldSpell();
    return cardMesh;
  }

  animateReposition(player: number, locName: any = 'mzone', slotIndex?: any, newPos?: any): void {
    soundManager.playDraw();
    // MSG_POS_CHANGE carries the zone; szone flips matter here (a set field
    // spell activating turns face-up). Older callers may pass the slot as the
    // 2nd argument with a numeric locName-less form — fall back to mzone.
    if (typeof locName !== 'string') {
      slotIndex = locName;
      locName = 'mzone';
    }
    const zone = this.cardsOnField[player] && this.cardsOnField[player][locName];
    const cardMesh = zone ? zone[slotIndex] : this.cardsOnField[player].mzone[slotIndex];
    if (!cardMesh) return;

    cardMesh.userData.positionState = this.positionStateFor(newPos);

    this.makeTween(cardMesh.position)
      .to({ y: 0.8 }, 150)
      .chain(
        this.makeTween(cardMesh.position)
          .to({ y: 0.05 }, 150)
      )
      .start();

    const rot = this.cardRotationFor(player, newPos);
    this.makeTween(cardMesh.rotation)
      .to({ x: rot.x, y: rot.y, z: rot.z }, 300)
      .easing(TWEEN.Easing.Cubic.InOut)
      .start();

    this.refreshFieldSpell();
  }

  animateAttack(attackerPlayer: number, attackerSlot: number, targetPlayer: number, targetSlot: number | undefined, isDirect = false): void {
    soundManager.playAttack();
    const attackerMesh = this.cardsOnField[attackerPlayer].mzone[attackerSlot];
    if (!attackerMesh) return;

    const startPos = { ...attackerMesh.position };
    let targetPos;

    if (isDirect || targetSlot === undefined || targetSlot < 0) {
      targetPos = { x: 0, y: 1.5, z: targetPlayer === 1 ? -6.0 : 6.0 };
    } else {
      const targetCoord = ZONE_COORDS[targetPlayer].mzone[targetSlot];
      targetPos = { x: targetCoord.x, y: 0.5, z: targetCoord.z };
    }

    this.makeTween(attackerMesh.position)
      .to({ y: 1.6 }, 200)
      .easing(TWEEN.Easing.Quadratic.Out)
      .chain(
        this.makeTween(attackerMesh.position)
          .to({ x: targetPos.x, y: targetPos.y + 0.3, z: targetPos.z }, 180)
          .easing(TWEEN.Easing.Exponential.In)
          .onComplete(() => {
            soundManager.playDamage();
            this.createHitSparks(targetPos.x, targetPos.z);
            this.cameraShake(0.2, 200);
          })
          .chain(
            this.makeTween(attackerMesh.position)
              .to(startPos, 300)
              .easing(TWEEN.Easing.Cubic.Out)
          )
      )
      .start();
  }

  animateDestroy(player: number, loc: string, slotIndex: number): void {
    soundManager.playDamage();
    const cardMesh = this.cardsOnField[player][loc][slotIndex];
    if (!cardMesh) return;

    this.cardsOnField[player][loc][slotIndex] = null;
    const graveCoord = ZONE_COORDS[player].grave;

    this.makeTween(cardMesh.position)
      .to({ y: 2.0 }, 150)
      .chain(
        this.makeTween(cardMesh.position)
          .to({ x: graveCoord.x, y: 0.2, z: graveCoord.z }, 350)
          .easing(TWEEN.Easing.Cubic.In)
          .onComplete(() => {
            this.scene.remove(cardMesh);
            const idx = this.cardMeshes.indexOf(cardMesh);
            if (idx > -1) this.cardMeshes.splice(idx, 1);
          })
      )
      .start();

    this.makeTween(cardMesh.rotation)
      .to({ x: 0, y: 0, z: Math.PI }, 350)
      .start();

    this.refreshFieldSpell();
  }

  // Removes a card mesh from its registered slot WITHOUT removing it from the
  // scene; returns the mesh (or null) so callers can move it elsewhere.
  removeFromSlot(player: number, locName: string, seq: number): any {
    const zone = this.cardsOnField[player] && this.cardsOnField[player][locName];
    if (!zone || !zone[seq]) return null;
    const mesh = zone[seq];
    zone[seq] = null;
    this.refreshFieldSpell();
    return mesh;
  }

  // Instant (non-animated) placement used by the replay applier when it
  // rebuilds the board for a seek/step-back. Supports every zone a card can
  // rest in: mzone/szone in `position`, grave/banish stacked, and piles.
  placeCard(player: number, locName: string, seq: number, cardCode: number, cardInfo: any, position = 0x1): any {
    if (locName === 'mzone' || locName === 'szone') {
      const coord = this.getZonePosition(player, locName, seq);
      if (!coord) return null;
      const mesh = this.createCardMesh(cardCode, cardInfo);
      mesh.position.set(coord.x, coord.y + 0.05, coord.z);
      mesh.rotation.set(0, 0, 0);
      const rot = this.cardRotationFor(player, position);
      mesh.rotation.set(rot.x, rot.y, rot.z);
      mesh.userData.slot = { player, loc: locName, seq };
      mesh.userData.positionState = this.positionStateFor(position);
      this.scene.add(mesh);
      this.cardsOnField[player][locName][seq] = mesh;
      this.refreshFieldSpell();
      return mesh;
    }
    if (locName === 'grave' || locName === 'banish') {
      const stack = this.cardsOnField[player][locName];
      const coord = this.getZonePosition(player, locName, 0);
      const mesh = this.createCardMesh(cardCode, cardInfo);
      mesh.position.set(coord.x, coord.y + 0.05 + Math.min(stack.length, 20) * 0.02, coord.z);
      mesh.rotation.set(0, player === 1 ? Math.PI : 0, 0);
      mesh.userData.slot = { player, loc: locName, seq: stack.length };
      this.scene.add(mesh);
      stack.push(mesh);
      if (locName === 'grave' && this.graveLockSprites.length) this.repositionGraveLockSprites();
      return mesh;
    }
    return null; // deck/extra/hand/overlay have no resting mesh on the board
  }

  // Moves an existing mesh to a destination zone. `instant` snaps instead of
  // tweening (replay seek). Deck/extra/overlay/hand destinations take the
  // mesh off the board — those zones are represented by stacks (deck/extra)
  // or the 2D hand dock, not individual meshes.
  moveCard(mesh: any, toPlayer: number, locName: string, seq: number, position = 0x1, { instant = false }: { instant?: boolean } = {}): void {
    if (!mesh) return;

    if (locName === 'mzone' || locName === 'szone') {
      const coord = this.getZonePosition(toPlayer, locName, seq);
      if (!coord) return;
      mesh.userData.slot = { player: toPlayer, loc: locName, seq };
      mesh.userData.positionState = this.positionStateFor(position);
      this.cardsOnField[toPlayer][locName][seq] = mesh;
      const rot = this.cardRotationFor(toPlayer, position);
      const target = { x: coord.x, y: coord.y + 0.05, z: coord.z };
      if (instant) {
        mesh.position.set(target.x, target.y, target.z);
        mesh.rotation.set(rot.x, rot.y, rot.z);
      } else {
        this.makeTween(mesh.position).to(target, 300).easing(TWEEN.Easing.Cubic.InOut).start();
        this.makeTween(mesh.rotation).to({ x: rot.x, y: rot.y, z: rot.z }, 300).easing(TWEEN.Easing.Cubic.InOut).start();
      }
      this.refreshFieldSpell();
      return;
    }

    if (locName === 'grave' || locName === 'banish') {
      const stack = this.cardsOnField[toPlayer][locName];
      const coord = this.getZonePosition(toPlayer, locName, 0);
      const idx = stack.length;
      const target = {
        x: coord.x,
        y: coord.y + 0.05 + Math.min(idx, 20) * 0.02,
        z: coord.z
      };
      mesh.userData.slot = { player: toPlayer, loc: locName, seq: idx };
      stack.push(mesh);
      if (instant) {
        mesh.position.set(target.x, target.y, target.z);
        mesh.rotation.set(0, toPlayer === 1 ? Math.PI : 0, 0);
      } else {
        this.makeTween(mesh.position).to(target, 320).easing(TWEEN.Easing.Cubic.In).start();
        this.makeTween(mesh.rotation)
          .to({ x: 0, y: toPlayer === 1 ? Math.PI : 0, z: 0 }, 320)
          .easing(TWEEN.Easing.Cubic.In)
          .start();
      }
      return;
    }

    // deck / extra / overlay / hand: the card leaves the 3D board.
    this.retireMesh(mesh, instant);
  }

  // Removes a mesh from the scene and tracking (shrinks it away when animated).
  retireMesh(mesh: any, instant = false): void {
    const finish = () => {
      this.scene.remove(mesh);
      const idx = this.cardMeshes.indexOf(mesh);
      if (idx > -1) this.cardMeshes.splice(idx, 1);
    };
    if (instant) {
      finish();
    } else {
      this.makeTween(mesh.scale)
        .to({ x: 0.05, y: 0.05, z: 0.05 }, 250)
        .easing(TWEEN.Easing.Quadratic.In)
        .onComplete(finish)
        .start();
    }
    this.refreshFieldSpell();
  }

  // Clears every card, chain badge, and in-flight tween off the board; the
  // replay applier calls this before rebuilding state for a seek.
  clearBoard(): void {
    this.hoveredCard = null;
    this.cardMeshes.slice().forEach(mesh => {
      this.scene.remove(mesh);
    });
    this.cardMeshes = [];
    this.opponentHandMeshes = [];
    this.cardsOnField = {
      0: { mzone: [], szone: [], grave: [], banish: [], deck: [], extra: [] },
      1: { mzone: [], szone: [], grave: [], banish: [], deck: [], extra: [] }
    };
    if (this.chainVisualizer) this.chainVisualizer.clearChain();
    this.clearRelationLines();
    this.tweenGroup.removeAll();
    // Force the field-spell background to re-evaluate (and fade out).
    this.fieldSpellCode = -1;
    this.refreshFieldSpell();
  }

  /**
   * 对手手牌：上缘一排背面朝上的卡（原版 opponent hand 的呈现方式）。
   * count 增减时直接落位（无飞入动画——抽牌动画已由 animateDrawCard 负责，
   * 落位只是把持久表示同步成最新数量）。
   */
  setOpponentHandCount(count: number): void {
    const target = Math.max(0, Math.min(count, 10));
    while (this.opponentHandMeshes.length > target) {
      const mesh = this.opponentHandMeshes.pop();
      this.scene.remove(mesh);
    }
    while (this.opponentHandMeshes.length < target) {
      const mesh = this.createCardMesh(0, null);
      mesh.userData.slot = { player: 1, loc: 'hand', seq: this.opponentHandMeshes.length };
      this.opponentHandMeshes.push(mesh);
      this.scene.add(mesh);
    }
    // 以对手视角朝向我们：卡背朝上（x=PI），排在对方后场之后居中微重叠
    const spacing = CARD_WIDTH * 0.55;
    this.opponentHandMeshes.forEach((mesh, i) => {
      const offset = i - (this.opponentHandMeshes.length - 1) / 2;
      mesh.position.set(-offset * spacing, 0.35, -8.8);
      mesh.rotation.set(Math.PI, 0, 0);
    });
  }

  // ---- 墓地禁查（drawing.cpp:564-575 的 tNegated 贴图）----
  // CARD_QUESTION 玩家提示置位后，双方墓地中心上空常显一张「?」圆牌
  // （高度随墓地张数抬高，对应 grave.size()*0.01f+0.02f）。
  graveLockSprites: any[] = [];

  setCantCheckGrave(on: boolean): void {
    if (on && this.graveLockSprites.length === 0) {
      for (const p of [0, 1]) {
        const canvas = document.createElement('canvas');
        canvas.width = 64;
        canvas.height = 64;
        const ctx = canvas.getContext('2d')!;
        ctx.fillStyle = 'rgba(15,23,42,0.85)';
        ctx.beginPath();
        ctx.arc(32, 32, 27, 0, Math.PI * 2);
        ctx.fill();
        ctx.strokeStyle = '#ef4444';
        ctx.lineWidth = 4;
        ctx.stroke();
        ctx.fillStyle = '#fecaca';
        ctx.font = 'bold 34px sans-serif';
        ctx.textAlign = 'center';
        ctx.textBaseline = 'middle';
        ctx.fillText('?', 32, 34);
        const tex = new (THREE as any).CanvasTexture(canvas);
        const sprite = new (THREE as any).Sprite(new (THREE as any).SpriteMaterial({
          map: tex, transparent: true, depthTest: false,
        }));
        sprite.userData.gravePlayer = p;
        this.graveLockSprites.push(sprite);
        this.scene.add(sprite);
      }
    }
    this.graveLockSprites.forEach((sprite) => {
      sprite.visible = on;
    });
    this.repositionGraveLockSprites();
  }

  /** 禁查图标随墓地叠高重新落位（与堆区 0.02/张的叠放一致） */
  repositionGraveLockSprites(): void {
    this.graveLockSprites.forEach((sprite) => {
      const p: number = sprite.userData.gravePlayer;
      const c = ZONE_COORDS[p].grave;
      const stack = (this.cardsOnField[p] && this.cardsOnField[p].grave) || [];
      sprite.position.set(c.x, 0.07 + Math.min(stack.length, 20) * 0.02 + 0.15, c.z);
      sprite.scale.set(1.1, 1.1, 1);
    });
  }

  // ---- 波 B 渲染原语：指示物 / 装备线 / 目标线 / 翻开确认 ----

  // 按引擎 LOCATION 码取一张场卡 mesh（hand/deck/extra 无 mesh 返回 null；
  // 堆区按序号取栈顶近似）
  meshAt(player: number, loc: number, seq: number): any {
    const locName = LOC_NAMES[loc];
    if (!locName || !this.cardsOnField[player]) return null;
    const zone = this.cardsOnField[player][locName];
    if (!zone) return null;
    if (locName === 'grave' || locName === 'banish') {
      return zone[Math.min(seq, zone.length - 1)] || null;
    }
    return zone[seq] || null;
  }

  // MSG_ADD/REMOVE_COUNTER：角标是卡 mesh 的子精灵，随卡移除自动消失
  applyCounterDelta(player: number, loc: number, seq: number, counterType: number, delta: number): void {
    const mesh = this.meshAt(player, loc, seq);
    if (!mesh) return;
    const counters = mesh.userData.counters || (mesh.userData.counters = {});
    const count = (counters[counterType] || 0) + delta;
    if (count > 0) counters[counterType] = count;
    else delete counters[counterType];
    this.renderCounterBadge(mesh);
  }

  renderCounterBadge(mesh: any): void {
    if (mesh.userData.counterSprite) {
      const sprite = mesh.userData.counterSprite;
      mesh.remove(sprite);
      if (sprite.material.map) sprite.material.map.dispose();
      sprite.material.dispose();
      mesh.userData.counterSprite = null;
    }
    const counters = mesh.userData.counters || {};
    const values = Object.values(counters) as number[];
    if (!values.length) return;
    const text = values.length === 1 ? String(values[0]) : values.join('+');

    const canvas = document.createElement('canvas');
    canvas.width = 64;
    canvas.height = 64;
    const ctx = canvas.getContext('2d')!;
    ctx.fillStyle = 'rgba(15,23,42,0.85)';
    ctx.beginPath();
    ctx.arc(32, 32, 27, 0, Math.PI * 2);
    ctx.fill();
    ctx.strokeStyle = '#fbbf24';
    ctx.lineWidth = 4;
    ctx.stroke();
    ctx.fillStyle = '#fde68a';
    ctx.font = 'bold 30px sans-serif';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText(text, 32, 34);

    const tex = new (THREE as any).CanvasTexture(canvas);
    const sprite = new (THREE as any).Sprite(new (THREE as any).SpriteMaterial({
      map: tex, transparent: true, depthTest: false,
    }));
    sprite.scale.set(0.7, 0.7, 1);
    sprite.position.set(CARD_WIDTH / 2 + 0.25, 0.1, 0);
    sprite.renderOrder = 10;
    mesh.add(sprite);
    mesh.userData.counterSprite = sprite;
  }

  // MSG_EQUIP / MSG_CARD_TARGET：两张场卡之间连一条关系线
  // （equip 橙色 / target 青色；同一条线不重复添加）
  addRelationLine(
    from: { player: number; loc: number; seq: number },
    to: { player: number; loc: number; seq: number },
    kind: 'equip' | 'target',
  ): void {
    const fromMesh = this.meshAt(from.player, from.loc, from.seq);
    const toMesh = this.meshAt(to.player, to.loc, to.seq);
    if (!fromMesh || !toMesh) return;
    const dup = this.relationLines.some((r) => r.kind === kind
      && r.fromMesh === fromMesh && r.toMesh === toMesh);
    if (dup) return;
    const pts = [
      new (THREE as any).Vector3().copy(fromMesh.position).setY(0.25),
      new (THREE as any).Vector3().copy(toMesh.position).setY(0.25),
    ];
    const geo = new (THREE as any).BufferGeometry().setFromPoints(pts);
    const line = new (THREE as any).Line(geo, new (THREE as any).LineBasicMaterial({
      color: kind === 'equip' ? 0xf97316 : 0x22d3ee,
      transparent: true,
      opacity: 0.85,
    }));
    this.scene.add(line);
    this.relationLines.push({ line, fromMesh, toMesh, kind });
  }

  // MSG_UNEQUIP / MSG_CANCEL_TARGET：解除关系线。to/kind 缺省时按起点
  // 宽松匹配（unequip 只带装备卡）。
  removeRelationLine(
    from: { player: number; loc: number; seq: number },
    to?: { player: number; loc: number; seq: number },
    kind?: 'equip' | 'target',
  ): void {
    const fromMesh = this.meshAt(from.player, from.loc, from.seq);
    if (!fromMesh) return;
    const toMesh = to ? this.meshAt(to.player, to.loc, to.seq) : null;
    this.relationLines = this.relationLines.filter((r) => {
      if (r.fromMesh !== fromMesh) return true;
      if (kind && r.kind !== kind) return true;
      if (toMesh && r.toMesh !== toMesh) return true;
      this.scene.remove(r.line);
      r.line.geometry.dispose();
      r.line.material.dispose();
      return false;
    });
  }

  // MSG_CONFIRM_CARDS：卡面黄色闪光（原版确认动画的简化版）
  flashCards(cards: { c: number; l: number; s: number }[]): void {
    for (const e of cards || []) {
      const mesh = this.meshAt(e.c, e.l, e.s);
      if (!mesh) continue;
      const mat = Array.isArray(mesh.material) ? mesh.material[2] : mesh.material;
      if (!mat || !mat.emissive) continue;
      mat.emissive = new (THREE as any).Color(0xfde047);
      mat.emissiveIntensity = 0;
      const proxy = { v: 0 };
      this.makeTween(proxy)
        .to({ v: 1 }, 180)
        .yoyo(true)
        .repeat(1)
        .easing(TWEEN.Easing.Quadratic.Out)
        .onUpdate(() => { mat.emissiveIntensity = proxy.v; })
        .onComplete(() => { mat.emissiveIntensity = 0; })
        .start();
    }
  }

  clearRelationLines(): void {
    this.relationLines.forEach((r) => {
      this.scene.remove(r.line);
      r.line.geometry.dispose();
      r.line.material.dispose();
    });
    this.relationLines = [];
  }

  createShockwave(x: number, z: number, colorHex = 0x00d2ff): void {
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
    this.scene.add(ringMesh);

    this.makeTween(ringMesh.scale)
      .to({ x: 6, y: 6, z: 6 }, 400)
      .easing(TWEEN.Easing.Quadratic.Out)
      .start();

    this.makeTween(ringMat)
      .to({ opacity: 0 }, 400)
      .easing(TWEEN.Easing.Quadratic.Out)
      .onComplete(() => this.scene.remove(ringMesh))
      .start();
  }

  createHitSparks(x: number, z: number): void {
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
    this.scene.add(pMesh);

    let progress = { t: 0 };
    this.makeTween(progress)
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
      .onComplete(() => this.scene.remove(pMesh))
      .start();
  }

  cameraShake(intensity = 0.25, duration = 200): void {
    const startCamPos = { x: 0, y: 13.5, z: 12.0 };
    const startTime = performance.now();

    const shakeInterval = setInterval(() => {
      const elapsed = performance.now() - startTime;
      if (elapsed >= duration) {
        clearInterval(shakeInterval);
        this.camera.position.set(startCamPos.x, startCamPos.y, startCamPos.z);
        return;
      }
      const damping = 1 - elapsed / duration;
      this.camera.position.x = startCamPos.x + (Math.random() - 0.5) * intensity * damping;
      this.camera.position.y = startCamPos.y + (Math.random() - 0.5) * intensity * damping;
    }, 16);
  }

  initEvents(): void {
    // Handlers are kept as bound references so dispose() can remove exactly
    // what was registered (an anonymous resize arrow can never be removed and
    // would stack one listener per duel screen mount).
    this._onResize = () => this.onWindowResize();
    this._onMouseMove = (e: MouseEvent) => {
      const rect = this.container.getBoundingClientRect();
      this.mouse.x = ((e.clientX - rect.left) / rect.width) * 2 - 1;
      this.mouse.y = -((e.clientY - rect.top) / rect.height) * 2 + 1;
      this._lastClientX = e.clientX;
      this._lastClientY = e.clientY;
      this.handleHover();
    };
    this._onClick = (e: MouseEvent) => {
      if (this.hoveredCard && this.onCardClick) {
        // Screen coordinates let the caller position an action popup (e.g.
        // the battle-phase attack menu) right where the player clicked.
        this.onCardClick(e.clientX, e.clientY, this.hoveredCard.userData);
      }
    };

    window.addEventListener('resize', this._onResize);
    this.container.addEventListener('mousemove', this._onMouseMove);
    this.container.addEventListener('click', this._onClick);
  }

  handleHover(): void {
    this.raycaster.setFromCamera(this.mouse, this.camera);
    const intersects = this.raycaster.intersectObjects(this.cardMeshes);

    if (intersects.length > 0) {
      const mesh = intersects[0].object;
      if (this.hoveredCard !== mesh) {
        if (this.hoveredCard) {
          this.makeTween(this.hoveredCard.position)
            .to({ y: this.hoveredCard.userData.originalY || 0.05 }, 120)
            .start();
        }
        this.hoveredCard = mesh;
        this.makeTween(mesh.position)
          .to({ y: (mesh.userData.originalY || 0.05) + 0.35 }, 120)
          .start();

        if (this.onCardInspect && mesh.userData.cardCode) {
          this.onCardInspect(mesh.userData.cardCode, mesh.userData.cardInfo);
        }
        // stTip 悬浮提示（原版 game.cpp:952 stTip）：由 CardTooltip 组件消费。
        this.container.dispatchEvent(new CustomEvent('ygo:cardhover', {
          detail: {
            code: mesh.userData.cardCode,
            info: mesh.userData.cardInfo,
            x: this._lastClientX, y: this._lastClientY,
          },
        }));
      }
    } else if (this.hoveredCard) {
      this.makeTween(this.hoveredCard.position)
        .to({ y: this.hoveredCard.userData.originalY || 0.05 }, 120)
        .start();
      this.hoveredCard = null;
      this.container.dispatchEvent(new CustomEvent('ygo:cardhoverend'));
    }
  }

  // 原版投影视锥左右不对称（game.cpp:994 BuildProjectionMatrix 左 -0.90 /
  // 右 0.45，中心偏左约 16.7%）：等效于把盘面往右推，给左侧 HUD 面板留位。
  // 用 three 的 setViewOffset 复刻；窄屏（面板隐藏）时取消偏移。
  applyViewOffset(): void {
    const width = this.container.clientWidth || window.innerWidth;
    const height = this.container.clientHeight || window.innerHeight;
    if (width > 820) {
      this.camera.setViewOffset(width, height, -width * 0.15, 0, width, height);
    } else {
      this.camera.clearViewOffset();
    }
  }

  onWindowResize(): void {
    const width = this.container.clientWidth || window.innerWidth;
    const height = this.container.clientHeight || window.innerHeight;
    this.camera.aspect = width / height;
    this.camera.updateProjectionMatrix();
    this.applyViewOffset();
    this.renderer.setSize(width, height);
  }

  animate(): void {
    if (this.disposed) return;
    requestAnimationFrame(() => this.animate());
    this.tweenGroup.update();
    this.renderer.render(this.scene, this.camera);
  }

  // dispose tears down the WebGL context and listeners when the component
  // hosting this field unmounts, so re-entering the duel screen does not leak
  // renderers, listeners, or GPU resources.
  dispose(): void {
    this.disposed = true;
    window.removeEventListener('resize', this._onResize);
    if (this.container) {
      this.container.removeEventListener('mousemove', this._onMouseMove);
      this.container.removeEventListener('click', this._onClick);
    }
    if (this.scene) {
      this.cardMeshes.slice().forEach((mesh) => {
        this.scene.remove(mesh);
        this.disposeMeshResources(mesh);
      });
      this.cardMeshes = [];
      this.graveLockSprites.forEach((sprite) => {
        this.scene.remove(sprite);
        if (sprite.material.map) sprite.material.map.dispose();
        sprite.material.dispose();
      });
      this.graveLockSprites = [];
    }
    this.cardTextureCache.forEach((tex) => tex.dispose());
    this.cardTextureCache.clear();
    if (this.cardBackTexture) this.cardBackTexture.dispose();
    if (this.matMesh) {
      this.scene.remove(this.matMesh);
      this.matMesh.geometry.dispose();
      this.matMesh.material.dispose();
      this.matMesh = null;
    }
    if (this.matTexture) {
      this.matTexture.dispose();
      this.matTexture = null;
    }
    if (this.transparentMatTexture) {
      this.transparentMatTexture.dispose();
      this.transparentMatTexture = null;
    }
    if (this.scene && this.scene.background && this.scene.background.dispose) {
      this.scene.background.dispose();
    }
    if (this.fieldSpellPlane) {
      this.scene.remove(this.fieldSpellPlane);
      this.fieldSpellPlane.geometry.dispose();
      this.fieldSpellPlane.material.dispose();
      this.fieldSpellPlane = null;
    }
    if (this.fieldSpellTexture) {
      this.fieldSpellTexture.dispose();
      this.fieldSpellTexture = null;
    }
    this.tweenGroup.removeAll();
    this.clearRelationLines();
    if (this.renderer) {
      this.renderer.dispose();
      if (this.renderer.domElement && this.renderer.domElement.parentNode) {
        this.renderer.domElement.parentNode.removeChild(this.renderer.domElement);
      }
    }
  }

  disposeMeshResources(mesh: any): void {
    if (mesh.geometry) mesh.geometry.dispose();
    const materials = Array.isArray(mesh.material) ? mesh.material : [mesh.material];
    materials.forEach((mat: any) => {
      if (mat) mat.dispose();
    });
  }
}
