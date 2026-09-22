/**
 * Three.js 3D Duel Field System
 * Complete 3D board rendering, card animations, raycasting, chain visualizer, and particle FX.
 */

import * as THREE from '../../libs/three.module.js';
import * as TWEEN from '../../libs/tween.esm.js';
import { settingsStore } from '../domain/settings.ts';
import { ChainVisualizer } from './chain_visualizer.ts';
import { LOC_NAMES } from '../domain/constants.ts';
import { CARD_WIDTH, CARD_HEIGHT, CARD_DEPTH, ZONE_COORDS } from './field3d_geometry.ts';
import { generateCardTexture as buildCardTexture } from './field3d_textures.ts';
import {
  animateDrawCard as runDrawCard,
  animateSummon as runSummon,
  animateSetCard as runSetCard,
  animateReposition as runReposition,
  animateAttack as runAttack,
  animateDestroy as runDestroy,
  createShockwave as runShockwave,
  createHitSparks as runHitSparks,
  cameraShake as runCameraShake,
} from './field3d_anim.ts';

// Re-exported for backward compatibility (was defined inline before P5-4).
export { CARD_WIDTH, CARD_HEIGHT, CARD_DEPTH, ZONE_COORDS };


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
  cardImageProvider: ((code: number) => Promise<{ url: string } | null>) | null = null;
  // Field spell art as the board background (the YGOPro field.png effect).
  fieldSpellPlane: any = null;
  fieldSpellTexture: any = null;
  fieldSpellCode = 0;      // code currently displayed (0 = plain board)
  fieldSpellPending = 0;   // code of the in-flight art load, 0 if none
  chainVisualizer: ChainVisualizer | null = null;
  // 原版战斗标记（textures/*.png 角标，drawing.cpp:443-562）：
  // attackMarks = 战阶可攻击怪兽头顶的剑形标记（上下浮动）；
  // actMarks = 可发动区域的旋转黄圈图标（每帧 +0.02 rad，同原版 act_rot）
  attackMarks: any[] = [];
  actMarks: any[] = [];
  attackTexture: any = null;
  actTexture: any = null;
  chainMarkTexture: any = null;
  numberTexture: any = null;
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
    this.negatedTexture = this.textureLoader.load('textures/negated.png');
    // 原版卡片状态角标贴图（drawing.cpp）：
    // attack.png 可攻击标记 :464、act.png 可发动旋转图标 :483、
    // chain.png + number.png 连锁序号角标 :541
    this.attackTexture = this.textureLoader.load('textures/attack.png');
    this.actTexture = this.textureLoader.load('textures/act.png');
    this.chainMarkTexture = this.textureLoader.load('textures/chain.png');
    this.numberTexture = this.textureLoader.load('textures/number.png');
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
    // MR2020 场地皮肤 field3.png（gframe/image_manager.cpp:42 rule>=4 时用
    // field3 / field-transparent3，与下方 MR4 ZONE_COORDS 的墓地/除外位一致）；
    // 有表侧场地魔法时换透明版露出场地大图
    const matTex = this.textureLoader.load('textures/field3.png', (tex: any) => {
      tex.colorSpace = THREE.SRGBColorSpace;
    });
    this.matTexture = matTex;
    this.transparentMatTexture = this.textureLoader.load('textures/field-transparent3.png', (tex: any) => {
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

  /** 某槽位当前渲染着的卡片 mesh（chain_visualizer 贴卡角标用） */
  getCardMesh(player: number, loc: string, seq: number): any | null {
    const stack = this.cardsOnField[player] && this.cardsOnField[player][loc];
    if (!stack) return null;
    return Array.isArray(stack) ? (stack[seq] || null) : null;
  }

  // ---- 可攻击标记（textures/attack.png，原版 drawing.cpp:464-472） ----
  // battleCmd 到来时在每只可攻击怪兽头顶放一柄剑，上下正弦浮动
  // （原版 atkdy 波动），响应战斗指令后由 duel_manager 调 clearAttackable。
  setAttackable(list: { c: number; s: number }[]): void {
    this.clearAttackable();
    for (const e of list || []) {
      const mesh = this.getCardMesh(e.c, 'mzone', e.s);
      const coord = mesh
        ? mesh.position
        : this.getZonePosition(e.c, 'mzone', e.s);
      const sprite: any = new (THREE as any).Sprite(new (THREE as any).SpriteMaterial({
        map: this.attackTexture, transparent: true, depthTest: false,
      }));
      sprite.scale.set(0.85, 0.85, 1);
      sprite.userData.baseY = coord.y + 1.7;
      sprite.userData.phase = Math.random() * Math.PI * 2;
      sprite.position.set(coord.x, sprite.userData.baseY, coord.z);
      sprite.renderOrder = 10;
      this.scene.add(sprite);
      this.attackMarks.push(sprite);
    }
  }

  clearAttackable(): void {
    for (const m of this.attackMarks) {
      this.scene.remove(m);
      m.material.dispose();
    }
    this.attackMarks = [];
  }

  // ---- 可发动标记（textures/act.png，原版 drawing.cpp:483-522） ----
  // idleCmd/battleCmd/chain 询问期间在可发动的卡/堆上放旋转黄圈
  // （原版 act_rot.Z 每帧 +0.02，见 animate()）。手牌无场上落点，跳过。
  setActivatable(list: { c: number; l: number; s?: number }[]): void {
    this.clearActivatable();
    for (const e of list || []) {
      const locName = LOC_NAMES[e.l];
      if (!locName || locName === 'hand') continue;
      const mesh = this.getCardMesh(e.c, locName, e.s || 0);
      const coord = mesh
        ? mesh.position
        : this.getZonePosition(e.c, locName, e.s || 0);
      const stack = this.cardsOnField[e.c] && this.cardsOnField[e.c][locName];
      const stackTop = Array.isArray(stack)
        ? (stack.length ? 0.02 + stack.length * 0.02 : 0.04)
        : 0.04;
      const sprite: any = new (THREE as any).Sprite(new (THREE as any).SpriteMaterial({
        map: this.actTexture, transparent: true, depthTest: false,
      }));
      sprite.scale.set(0.9, 0.9, 1);
      sprite.userData.isActMark = true;
      sprite.position.set(coord.x, coord.y + stackTop + 0.55, coord.z);
      sprite.renderOrder = 10;
      this.scene.add(sprite);
      this.actMarks.push(sprite);
    }
  }

  clearActivatable(): void {
    for (const m of this.actMarks) {
      this.scene.remove(m);
      m.material.dispose();
    }
    this.actMarks = [];
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
      map: this.generateCardTexture(cardCode),
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

  generateCardTexture(cardCode: number): any {
    return buildCardTexture(this, cardCode);
  }

  animateDrawCard(player: number, cardCode: number, cardInfo: any = null, onComplete: (() => void) | null = null): void {
    runDrawCard(this, player, cardCode, cardInfo, onComplete);
  }

  animateSummon(player: number, cardCode: number, slotIndex: number, position = 0x1, cardInfo: any = null, isSpecial = false): any {
    return runSummon(this, player, cardCode, slotIndex, position, cardInfo, isSpecial);
  }

  animateSetCard(player: number, cardCode: number, slotIndex: number, isMonster = false, cardInfo: any = null): any {
    return runSetCard(this, player, cardCode, slotIndex, isMonster, cardInfo);
  }

  animateReposition(player: number, locName: any = 'mzone', slotIndex?: any, newPos?: any): void {
    runReposition(this, player, locName, slotIndex, newPos);
  }

  animateAttack(attackerPlayer: number, attackerSlot: number, targetPlayer: number, targetSlot: number | undefined, isDirect = false): void {
    runAttack(this, attackerPlayer, attackerSlot, targetPlayer, targetSlot, isDirect);
  }

  animateDestroy(player: number, loc: string, slotIndex: number): void {
    runDestroy(this, player, loc, slotIndex);
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
  // CARD_QUESTION 玩家提示置位后，双方墓地中心上空常显 negated.png
  // （tNegated，image_manager.cpp:25）——原版是贴图，非自绘「?」。
  // 高度随墓地张数抬高，对应 grave.size()*0.01f+0.02f。
  graveLockSprites: any[] = [];
  negatedTexture: any = null;

  setCantCheckGrave(on: boolean): void {
    if (on && this.graveLockSprites.length === 0) {
      for (const p of [0, 1]) {
        const sprite = new (THREE as any).Sprite(new (THREE as any).SpriteMaterial({
          map: this.negatedTexture, transparent: true, depthTest: false,
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
    runShockwave(this, x, z, colorHex);
  }

  createHitSparks(x: number, z: number): void {
    runHitSparks(this, x, z);
  }

  cameraShake(intensity = 0.25, duration = 200): void {
    runCameraShake(this, intensity, duration);
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
    // 原版 attack.png 标记上下浮动（drawing.cpp atkdy 正弦）、
    // act.png 图标匀速自旋（原版 act_rot.Z 每帧 +0.02）
    const t = performance.now();
    for (const m of this.attackMarks) {
      m.position.y = m.userData.baseY + Math.sin(t / 320 + m.userData.phase) * 0.16;
    }
    for (const m of this.actMarks) {
      m.material.rotation += 0.02;
    }
    this.chainVisualizer?.tick();
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
      this.clearAttackable();
      this.clearActivatable();
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
