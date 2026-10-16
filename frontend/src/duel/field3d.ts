/**
 * Three.js 3D Duel Field System
 * Complete 3D board rendering, card animations, raycasting, chain visualizer, and particle FX.
 */

import * as THREE from '../../libs/three.module.js';
import * as TWEEN from '../../libs/tween.esm.js';
import { settingsStore } from '../domain/settings.ts';
import { ChainVisualizer } from './chain_visualizer.ts';
import {
  LOC_NAMES, TYPE_LINK, TYPE_MONSTER, TYPE_XYZ, TYPE_TUNER,
  LINK_MARKER_BOTTOM_LEFT, LINK_MARKER_BOTTOM, LINK_MARKER_BOTTOM_RIGHT,
  LINK_MARKER_LEFT, LINK_MARKER_RIGHT,
  LINK_MARKER_TOP_LEFT, LINK_MARKER_TOP, LINK_MARKER_TOP_RIGHT,
} from '../domain/constants.ts';
import { CARD_WIDTH, CARD_HEIGHT, CARD_DEPTH, ZONE_COORDS } from './field3d_geometry.ts';
import { fieldZonePosition, handPose } from './original_geometry.ts';
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
  handMeshes: [any[], any[]] = [[], []];
  overlayMeshes: any[][][] = Array.from({length:2},()=>Array.from({length:7},()=>[]));
  duelRule = 5;
  localSeat = 0;
  hoveredCard: any = null;
  textureLoader: any = new (THREE as any).TextureLoader();
  cardBackTexture: any = null;
  cardBackTexture2: any = null;
  cardTextureCache = new Map<number, any>();
  // Optional async (code) => Promise<dataURL|null> set by the host; when a
  // picture arrives the cached canvas texture is redrawn in place so every
  // mesh sharing it picks the art up on the next frame.
  cardImageProvider: ((code: number) => Promise<{ url: string } | null>) | null = null;
  fieldImageProvider: ((code:number)=>Promise<{url:string}|null>) | null = null;
  fieldSpellPlanes: any[] = [];
  fieldSpellSignature = '';
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
  // ---- 悬停关系图标（drawing.cpp:446-457 + client_field.cpp SetShowMark）----
  // 悬停某卡时它的装备/对象/连锁对象关联卡上画 equip.png/target.png/
  // chaintarget.png 平铺图标；关联数据由 DuelManager 维护，悬停时经
  // onHoverChange 重算（见 setRelationMarks）。原版无事件驱动关系连线。
  relationMarks: { icon: any; mesh: any }[] = [];
  equipTexture: any = null;
  targetTexture: any = null;
  chainTargetTexture: any = null;
  onHoverChange: ((mesh: any | null) => void) | null = null;
  disposed = false;
  // ---- MSG_SELECT_PLACE 落点选择（原版 drawing.cpp:187-209 的虚线高亮）----
  // placeSelectMarks = 可选格的虚线描边面片（点击射线检测目标）；
  // disabledMarks = MSG_FIELD_DISABLED 禁用格的白叉（drawing.cpp:210-241）。
  placeSelectMarks: { mesh: any; zone: { player: number; loc: number; seq: number } }[] = [];
  placeSelectTextures: { on: any; off: any } | null = null;
  disabledMarks: any[] = [];
  // ---- select_card / select_unselect 场上点选（原版 drawing.cpp:431-436
  // 可选卡画黄色虚线框 DrawSelectionLine，直接点卡完成选择）----
  // cardSelectMarks = 可选场卡上的黄框面片；点击射线直接打卡 mesh。
  cardSelectMarks: { mesh: any; idx: number; cardMesh: any }[] = [];
  cardSelectTextures: { on: any; off: any } | null = null;
  // 视角交换（原版 SwapField/ReplaySwap）：true 时相机转到场地另一侧，
  // 3D 座标不动——从对侧看等效于场地旋转 180°（卡牌朝向随观看侧自然正确）
  viewSwapped = false;
  // HINT_ZONE 区域高亮（duelclient.cpp:1168：与 select_place 同位域，
  // 样式区分——绿色实线+浅填充；瞬态展示，duel_manager 定时清除）
  hintZoneMarks: any[] = [];
  hintZoneTexture: any = null;
  // ---- 连接怪互连区域高亮（原版 drawing.cpp:278-359 DrawLinkedZones +
  // CheckMutual）：悬停场上连接怪时把它箭头指向的格画成半透明色块，
  // 单向链接 = 原版 0xff0261a2 蓝、互连（目标格上的连接怪有指回来的
  // 标记）= 0xff009900 绿 ----
  linkZoneMarks: { mesh: any; mutual: boolean }[] = [];
  linkZoneTextures: { link: any; mutual: any } | null = null;
  // ---- 怪兽攻守状态浮签（原版 drawing.cpp:771-786 DrawStatus）----
  // statusLabels = 每只可见怪兽的两行文本 sprite（等级串 + atk/def），
  // scene 直挂（不做 mesh 子节点，避免随守备表示旋转），逐帧跟随 mesh 位置。
  statusLabels: { sprite: any; mesh: any; seat: number }[] = [];
  // ---- 牌堆计数浮签（原版 drawing.cpp:724-769 DrawMisc 的黄色数字）----
  // 键 `seat:pile`（引擎 seat；deck/extra/grave/banish），>0 才存在。
  pileCountSprites: Record<string, any> = {};
  // 点选/右键回调由 DuelStage 挂到 manager 方法上（构造时还没有 manager）
  onPlaceZoneClick: ((zone: { player: number; loc: number; seq: number }) => void) | null = null;
  onCardSelectPick: ((idx: number) => void) | null = null;
  onBoardRightClick: ((x: number, y: number) => void) | null = null;
  onPileRightClick: ((player: number, pile: string) => void) | null = null;
  /** 左键点堆区（含空堆）= 打开列表查看（原版 MSG_WAITING 点堆区的 COMMAND_LIST） */
  onPileLeftClick: ((player: number, pile: string) => void) | null = null;
  matTexture: any = null;
  transparentMatTexture: any = null;
  matMesh: any = null;
  _onResize: () => void = () => {};
  _onMouseMove: (e: MouseEvent) => void = () => {};
  _onClick: (e: MouseEvent) => void = () => {};
  _onContextMenu: (e: MouseEvent) => void = () => {};
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
    this.camera.position.set(0.4, 15.6, 16);
    this.camera.lookAt(0.4, 0, 0);
    this.applyViewOffset();

    this.renderer = new (THREE as any).WebGLRenderer({ antialias: true, alpha: true });
    this.renderer.setSize(width, height);
    this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
    this.renderer.shadowMap.enabled = false;
    this.renderer.shadowMap.type = THREE.PCFSoftShadowMap;

    this.container.appendChild(this.renderer.domElement);
    this.cardBackTexture = this.textureLoader.load('textures/cover.jpg');
    // 原版卡背按控制方区分（drawing.cpp:425 tCover[pcard->controler]）：
    // 0 号位 cover.jpg、1 号位 cover2.jpg
    this.cardBackTexture2 = this.textureLoader.load('textures/cover2.jpg');
    this.negatedTexture = this.textureLoader.load('textures/negated.png');
    // 原版卡片状态角标贴图（drawing.cpp）：
    // attack.png 可攻击标记 :464、act.png 可发动旋转图标 :483、
    // chain.png + number.png 连锁序号角标 :541
    this.attackTexture = this.textureLoader.load('textures/attack.png');
    this.actTexture = this.textureLoader.load('textures/act.png');
    this.chainMarkTexture = this.textureLoader.load('textures/chain.png');
    this.numberTexture = this.textureLoader.load('textures/number.png');
    this.equipTexture = this.textureLoader.load('textures/equip.png');
    this.targetTexture = this.textureLoader.load('textures/target.png');
    this.chainTargetTexture = this.textureLoader.load('textures/chaintarget.png');
  }

  initLights(): void {
    // 原版 gframe 只有一盏全亮环境光（game.cpp:999
    // setAmbientLight(SColorf(1,1,1))），没有彩色点光/雾/辉光——那些是现代
    // 游戏的观感，不是 YGOPro 的。保留一盏弱白方向光只为投影服务。
    const ambientLight = new (THREE as any).AmbientLight(0xffffff, 1.0);
    this.scene.add(ambientLight);

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
    return fieldZonePosition(player, loc, seq, this.duelRule) as any;
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

  // ---- MSG_SELECT_PLACE / SELECT_DISFIELD 落点选择 ----
  // 原版 drawing.cpp:187-209：selectable_field 的每格画蓝色虚线选框
  // （DrawSelectionLine，alpha 在 255↔25 间呼吸），selected_field 的格
  // 反转虚线相位——这里译为未选=蓝色虚线呼吸、已选=琥珀实线+浅填充。
  // zones 的 player 是引擎 seat（与 ZONE_COORDS 一致），loc 是引擎 LOCATION。

  /** 虚线/实线描边的共享贴图（懒生成，全实例共享两种状态图） */
  ensurePlaceSelectTextures(): { on: any; off: any } {
    if (this.placeSelectTextures) return this.placeSelectTextures;
    const build = (selected: boolean): any => {
      const canvas = document.createElement('canvas');
      canvas.width = 128;
      canvas.height = 184;
      const ctx = canvas.getContext('2d')!;
      ctx.clearRect(0, 0, 128, 184);
      if (selected) {
        ctx.fillStyle = 'rgba(251, 191, 36, 0.16)';
        ctx.fillRect(0, 0, 128, 184);
        ctx.strokeStyle = '#fbbf24';
        ctx.lineWidth = 7;
        ctx.strokeRect(4, 4, 120, 176);
      } else {
        ctx.strokeStyle = '#3b82f6';
        ctx.lineWidth = 6;
        ctx.setLineDash([16, 10]);
        ctx.strokeRect(4, 4, 120, 176);
      }
      const tex = new (THREE as any).CanvasTexture(canvas);
      return tex;
    };
    this.placeSelectTextures = { on: build(true), off: build(false) };
    return this.placeSelectTextures;
  }

  /**
   * 布设可选落点高亮。selectedKeys 是 "player:loc:seq" 集合
   * （duel_manager 维护选择状态）；每次选择变化整体重建（格数 ≤ 16，廉价）。
   */
  setPlaceSelectZones(
    zones: { player: number; loc: number; seq: number }[],
    selectedKeys: Set<string>,
  ): void {
    this.clearPlaceSelectZones();
    const tex = this.ensurePlaceSelectTextures();
    for (const zone of zones || []) {
      const coord = this.getZonePosition(zone.player, LOC_NAMES[zone.loc] || 'mzone', zone.seq);
      const selected = selectedKeys.has(`${zone.player}:${zone.loc}:${zone.seq}`);
      const geo = new (THREE as any).PlaneGeometry(CARD_WIDTH + 0.16, CARD_HEIGHT + 0.16);
      const mat = new (THREE as any).MeshBasicMaterial({
        map: selected ? tex.on : tex.off,
        transparent: true,
        depthWrite: false,
        opacity: selected ? 0.95 : 0.8,
      });
      const mesh = new (THREE as any).Mesh(geo, mat);
      mesh.rotation.x = -Math.PI / 2;
      mesh.position.set(coord.x, 0.045, coord.z);
      mesh.renderOrder = 5;
      mesh.userData.placeZone = zone;
      mesh.userData.selected = selected;
      this.scene.add(mesh);
      this.placeSelectMarks.push({ mesh, zone });
    }
  }

  clearPlaceSelectZones(): void {
    for (const m of this.placeSelectMarks) {
      this.scene.remove(m.mesh);
      m.mesh.geometry.dispose();
      m.mesh.material.dispose();
    }
    this.placeSelectMarks = [];
  }

  // ---- select_card / select_unselect 场上可选卡（黄色选框）----
  // 原版 drawing.cpp:431-436：is_selectable 的场卡画黄色虚线框、已选反相；
  // 这里译为未选=黄色虚线呼吸、已选=黄色实线+浅填充。点击命中直接打
  // 卡 mesh（pickCardSelect），不依赖面片。

  ensureCardSelectTextures(): { on: any; off: any } {
    if (this.cardSelectTextures) return this.cardSelectTextures;
    const build = (selected: boolean): any => {
      const canvas = document.createElement('canvas');
      canvas.width = 128;
      canvas.height = 184;
      const ctx = canvas.getContext('2d')!;
      ctx.clearRect(0, 0, 128, 184);
      if (selected) {
        ctx.fillStyle = 'rgba(250, 204, 21, 0.18)';
        ctx.fillRect(0, 0, 128, 184);
        ctx.strokeStyle = '#facc15';
        ctx.lineWidth = 7;
        ctx.strokeRect(4, 4, 120, 176);
      } else {
        ctx.strokeStyle = '#facc15';
        ctx.lineWidth = 6;
        ctx.setLineDash([16, 10]);
        ctx.strokeRect(4, 4, 120, 176);
      }
      return new (THREE as any).CanvasTexture(canvas);
    };
    this.cardSelectTextures = { on: build(true), off: build(false) };
    return this.cardSelectTextures;
  }

  /**
   * 布设场上可选卡高亮。entries 的 c/l/s 是引擎座标（与 cardsOnField 一致），
   * selectedIdx 是已选应答下标集合；每次选择变化整体重建（候选数小，廉价）。
   */
  setCardSelectMarks(
    entries: { idx: number; c: number; l: number; s: number }[],
    selectedIdx: Set<number>,
  ): void {
    this.clearCardSelectMarks();
    const tex = this.ensureCardSelectTextures();
    for (const entry of entries || []) {
      const cardMesh = this.meshAt(entry.c, entry.l, entry.s);
      if (!cardMesh) continue;
      const selected = selectedIdx.has(entry.idx);
      const geo = new (THREE as any).PlaneGeometry(CARD_WIDTH + 0.16, CARD_HEIGHT + 0.16);
      const mat = new (THREE as any).MeshBasicMaterial({
        map: selected ? tex.on : tex.off,
        transparent: true,
        depthWrite: false,
        opacity: selected ? 0.95 : 0.8,
      });
      const mesh = new (THREE as any).Mesh(geo, mat);
      mesh.rotation.x = -Math.PI / 2;
      mesh.position.set(cardMesh.position.x, cardMesh.position.y + 0.06, cardMesh.position.z);
      mesh.quaternion.copy(cardMesh.quaternion);
      mesh.rotateX(-Math.PI/2);
      mesh.renderOrder = 6;
      mesh.userData.selected = selected;
      this.scene.add(mesh);
      this.cardSelectMarks.push({ mesh, idx: entry.idx, cardMesh });
    }
  }

  clearCardSelectMarks(): void {
    for (const m of this.cardSelectMarks) {
      this.scene.remove(m.mesh);
      m.mesh.geometry.dispose();
      m.mesh.material.dispose();
    }
    this.cardSelectMarks = [];
  }

  /** 鼠标下的可选场卡（命中卡 mesh 本身）；无则 null */
  pickCardSelect(): { idx: number; cardMesh: any } | null {
    if (!this.cardSelectMarks.length) return null;
    this.raycaster.setFromCamera(this.mouse, this.camera);
    const hits = this.raycaster.intersectObjects(this.cardSelectMarks.map((m) => m.cardMesh));
    if (!hits.length) return null;
    const mark = this.cardSelectMarks.find((m) => m.cardMesh === hits[0].object);
    return mark ? { idx: mark.idx, cardMesh: mark.cardMesh } : null;
  }

  // ---- 视角交换（观战 btnSpectatorSwap / 回放 btnReplaySwap）----
  // 相机绕到场地另一侧（等效场地旋转 180°）：卡牌朝向/守备方向随观看侧
  // 自然正确，3D 座标与 ZONE_COORDS 不变。2D HUD 的座位显示由 store 的
  // viewSwapped 负责（localSeat 翻转），这里只搬相机。
  setViewSwapped(on: boolean): void {
    if (this.viewSwapped === on) return;
    this.viewSwapped = on;
    const targetZ = on ? -12.5 : 12.5;
    const lookZ = on ? -0.3 : 0.3;
    this.makeTween(this.camera.position)
      .to({ x:targetZ>0?0.4:-0.6,z:targetZ },500)
      .easing(TWEEN.Easing.Quadratic.InOut)
      .onUpdate(() => this.camera.lookAt(targetZ>0?0.4:-0.6,0,0))
      .onComplete(() => this.camera.lookAt(targetZ>0?0.4:-0.6,0,0))
      .start();
  }

  // ---- HINT_ZONE 区域高亮（绿色实线+浅填充，与 select_place 蓝虚线区分）----

  ensureHintZoneTexture(): any {
    if (this.hintZoneTexture) return this.hintZoneTexture;
    const canvas = document.createElement('canvas');
    canvas.width = 128;
    canvas.height = 184;
    const ctx = canvas.getContext('2d')!;
    ctx.clearRect(0, 0, 128, 184);
    ctx.fillStyle = 'rgba(74, 222, 128, 0.18)';
    ctx.fillRect(0, 0, 128, 184);
    ctx.strokeStyle = '#4ade80';
    ctx.lineWidth = 7;
    ctx.strokeRect(4, 4, 120, 176);
    this.hintZoneTexture = new (THREE as any).CanvasTexture(canvas);
    return this.hintZoneTexture;
  }

  /** 布设 HINT_ZONE 高亮（zones 的 player 是引擎 seat，同 setPlaceSelectZones） */
  setHintZones(zones: { player: number; loc: number; seq: number }[]): void {
    this.clearHintZones();
    const tex = this.ensureHintZoneTexture();
    for (const zone of zones || []) {
      const coord = this.getZonePosition(zone.player, LOC_NAMES[zone.loc] || 'mzone', zone.seq);
      const geo = new (THREE as any).PlaneGeometry(CARD_WIDTH + 0.16, CARD_HEIGHT + 0.16);
      const mat = new (THREE as any).MeshBasicMaterial({
        map: tex,
        transparent: true,
        depthWrite: false,
        opacity: 0.9,
      });
      const mesh = new (THREE as any).Mesh(geo, mat);
      mesh.rotation.x = -Math.PI / 2;
      mesh.position.set(coord.x, 0.05, coord.z);
      mesh.renderOrder = 6;
      mesh.userData.hintZone = zone;
      this.scene.add(mesh);
      this.hintZoneMarks.push(mesh);
    }
  }

  clearHintZones(): void {
    for (const m of this.hintZoneMarks) {
      this.scene.remove(m);
      m.geometry.dispose();
      m.material.dispose();
    }
    this.hintZoneMarks = [];
  }

  // ---- 连接怪互连区域高亮（DrawLinkedZones / CheckMutual）----
  // 区域计算全在引擎座标做：cardsOnField 的下标即引擎 seat/seq，
  // ZONE_COORDS 已把双方镜像布局烤进坐标（p1 的 seq 0-4 与额外怪区 5/6
  // 共享格的编号都按引擎序），视角交换只搬相机，这里无需任何换算。
  // 原版 dInfo.duel_rule>=4 的分支恒取——本前端场地固定 MR2020 布局。

  /** 单向=原版 0xff0261a2 蓝、互连=0xff009900 绿（CheckMutual 的两种着色） */
  ensureLinkZoneTextures(): { link: any; mutual: any } {
    if (this.linkZoneTextures) return this.linkZoneTextures;
    const build = (fill: string, stroke: string): any => {
      const canvas = document.createElement('canvas');
      canvas.width = 128;
      canvas.height = 184;
      const ctx = canvas.getContext('2d')!;
      ctx.clearRect(0, 0, 128, 184);
      ctx.fillStyle = fill;
      ctx.fillRect(0, 0, 128, 184);
      ctx.strokeStyle = stroke;
      ctx.lineWidth = 7;
      ctx.strokeRect(4, 4, 120, 176);
      return new (THREE as any).CanvasTexture(canvas);
    };
    this.linkZoneTextures = {
      link: build('rgba(2, 97, 162, 0.30)', '#2a9bf0'),
      mutual: build('rgba(0, 153, 0, 0.30)', '#00cc44'),
    };
    return this.linkZoneTextures;
  }

  /** mesh 是带 arrow 回指标记的连接怪？（CheckMutual 的 pcard2 判定） */
  isMutualLink(mesh: any, arrow: number): boolean {
    return !!mesh
      && (((mesh.userData.cardType || 0) & TYPE_LINK) !== 0)
      && (((mesh.userData.linkMarker || 0) & arrow) !== 0);
  }

  /**
   * 原版 drawing.cpp:278-350 DrawLinkedZones 的逐行移植（引擎座标）。
   * 返回该卡 link_marker 指向的 mzone 格及互连标记；mark=0 或主怪区
   * 边界外的方向按原版规则跳过。
   */
  computeLinkedZones(
    controler: number, seq: number, mark: number,
  ): { player: number; seq: number; mutual: boolean }[] {
    const at = (c: number, s: number): any =>
      (this.cardsOnField[c] && this.cardsOnField[c].mzone[s]) || null;
    const out: { player: number; seq: number; mutual: boolean }[] = [];
    // other：共享额外怪区的另一方编号（己方 5/6 与对方 6/5 是同物理格），
    // 主查格为空时用于互连判定（原版 pcard2 的落空替换）
    const push = (c: number, s: number, arrow: number,
      other?: { c: number; s: number; arrow: number }): void => {
      const target = at(c, s);
      const mutual = this.isMutualLink(target, arrow)
        || (!target && !!other && this.isMutualLink(at(other.c, other.s), other.arrow));
      out.push({ player: c, seq: s, mutual });
    };
    if (seq < 5) {
      if (mark & LINK_MARKER_LEFT && seq > 0) push(controler, seq - 1, LINK_MARKER_RIGHT);
      if (mark & LINK_MARKER_RIGHT && seq < 4) push(controler, seq + 1, LINK_MARKER_LEFT);
      if ((mark & LINK_MARKER_TOP_LEFT && seq === 2)
        || (mark & LINK_MARKER_TOP && seq === 1)
        || (mark & LINK_MARKER_TOP_RIGHT && seq === 0)) {
        const arrow = seq === 2 ? LINK_MARKER_BOTTOM_RIGHT : seq === 1 ? LINK_MARKER_BOTTOM : LINK_MARKER_BOTTOM_LEFT;
        const otherArrow = seq === 2 ? LINK_MARKER_TOP_LEFT : seq === 1 ? LINK_MARKER_TOP : LINK_MARKER_TOP_RIGHT;
        push(controler, 5, arrow, { c: 1 - controler, s: 6, arrow: otherArrow });
      }
      if ((mark & LINK_MARKER_TOP_LEFT && seq === 4)
        || (mark & LINK_MARKER_TOP && seq === 3)
        || (mark & LINK_MARKER_TOP_RIGHT && seq === 2)) {
        const arrow = seq === 4 ? LINK_MARKER_BOTTOM_RIGHT : seq === 3 ? LINK_MARKER_BOTTOM : LINK_MARKER_BOTTOM_LEFT;
        const otherArrow = seq === 4 ? LINK_MARKER_TOP_LEFT : seq === 3 ? LINK_MARKER_TOP : LINK_MARKER_TOP_RIGHT;
        push(controler, 6, arrow, { c: 1 - controler, s: 5, arrow: otherArrow });
      }
    } else {
      const swap = seq === 5 ? 0 : 2;
      if (mark & LINK_MARKER_BOTTOM_LEFT) push(controler, 0 + swap, LINK_MARKER_TOP_RIGHT);
      if (mark & LINK_MARKER_BOTTOM) push(controler, 1 + swap, LINK_MARKER_TOP);
      if (mark & LINK_MARKER_BOTTOM_RIGHT) push(controler, 2 + swap, LINK_MARKER_TOP_LEFT);
      if (mark & LINK_MARKER_TOP_LEFT) push(1 - controler, 4 - swap, LINK_MARKER_TOP_LEFT);
      if (mark & LINK_MARKER_TOP) push(1 - controler, 3 - swap, LINK_MARKER_TOP);
      if (mark & LINK_MARKER_TOP_RIGHT) push(1 - controler, 2 - swap, LINK_MARKER_TOP_RIGHT);
    }
    return out;
  }

  /**
   * 按当前悬停卡重算互连高亮（悬停切换/离场/update_data 后都要调）。
   * 原版触发点：悬停在连接怪上（drawing.cpp:252-254，type & TYPE_LINK）。
   */
  updateLinkedZones(): void {
    this.clearLinkedZones();
    const mesh = this.hoveredCard;
    const slot = mesh && mesh.userData.slot;
    if (!mesh || !slot || slot.loc !== 'mzone') return;
    if (!((mesh.userData.cardType || 0) & TYPE_LINK)) return;
    const mark = mesh.userData.linkMarker || 0;
    if (!mark) return;
    // 悬停引用可能已离场（mesh 退休时 hoveredCard 未清），以登记槽位为准
    const zone = this.cardsOnField[slot.player] && this.cardsOnField[slot.player].mzone;
    if (!zone || zone[slot.seq] !== mesh) return;
    const tex = this.ensureLinkZoneTextures();
    for (const z of this.computeLinkedZones(slot.player, slot.seq, mark)) {
      const coord = this.getZonePosition(z.player, 'mzone', z.seq);
      const geo = new (THREE as any).PlaneGeometry(CARD_WIDTH + 0.16, CARD_HEIGHT + 0.16);
      const mat = new (THREE as any).MeshBasicMaterial({
        map: z.mutual ? tex.mutual : tex.link,
        transparent: true,
        depthWrite: false,
        opacity: 0.9,
      });
      const m = new (THREE as any).Mesh(geo, mat);
      m.rotation.x = -Math.PI / 2;
      m.position.set(coord.x, 0.055, coord.z);
      m.renderOrder = 6;
      m.userData.linkZone = { player: z.player, seq: z.seq, mutual: z.mutual };
      this.scene.add(m);
      this.linkZoneMarks.push({ mesh: m, mutual: z.mutual });
    }
  }

  clearLinkedZones(): void {
    for (const m of this.linkZoneMarks) {
      this.scene.remove(m.mesh);
      m.mesh.geometry.dispose();
      m.mesh.material.dispose();
    }
    this.linkZoneMarks = [];
  }

  // ---- MSG_FIELD_DISABLED 禁用格（drawing.cpp:210-241：格上画白色对角叉线）----
  // flag 位域与 decorateSelectPlace 同构：低 16 位 = 操作方 mzone(0-6)/
  // szone(8-13+灵摆 14/15)，高 16 位 = 对手同构区域。
  setFieldDisabled(flag: number): void {
    for (const m of this.disabledMarks) {
      this.scene.remove(m);
      m.geometry.dispose();
      m.material.dispose();
    }
    this.disabledMarks = [];
    const zones: { player: number; loc: number; seq: number }[] = [];
    const scan = (player: number, loc: number, base: number, count: number) => {
      for (let seq = 0; seq < count; seq++) {
        if (flag & (base << seq)) zones.push({ player, loc, seq });
      }
    };
    scan(0, 0x04, 0x1, 7);
    scan(0, 0x08, 0x100, 8);
    scan(1, 0x04, 0x10000, 7);
    scan(1, 0x08, 0x1000000, 8);
    for (const zone of zones) {
      const coord = this.getZonePosition(zone.player, LOC_NAMES[zone.loc] || 'mzone', zone.seq);
      const hw = CARD_WIDTH / 2 + 0.06;
      const hd = CARD_HEIGHT / 2 + 0.06;
      const pts = [
        new (THREE as any).Vector3(coord.x - hw, 0.06, coord.z - hd),
        new (THREE as any).Vector3(coord.x + hw, 0.06, coord.z + hd),
      ];
      const pts2 = [
        new (THREE as any).Vector3(coord.x - hw, 0.06, coord.z + hd),
        new (THREE as any).Vector3(coord.x + hw, 0.06, coord.z - hd),
      ];
      for (const linePts of [pts, pts2]) {
        const geo = new (THREE as any).BufferGeometry().setFromPoints(linePts);
        const line = new (THREE as any).Line(geo, new (THREE as any).LineBasicMaterial({
          color: 0xffffff,
          transparent: true,
          opacity: 0.9,
        }));
        line.renderOrder = 5;
        this.scene.add(line);
        this.disabledMarks.push(line);
      }
    }
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
    const codes=[0,1].map(seat=>{
      const m=this.cardsOnField[seat]?.szone[5];
      return settingsStore.get('draw_field_spell')===0||!m||m.userData.positionState==='FACEDOWN'?0:m.userData.cardCode||0;
    });
    const signature=codes.join(':');
    if(signature===this.fieldSpellSignature)return;
    this.fieldSpellSignature=signature;
    this.fieldSpellCode=codes[0]||codes[1];
    for(const plane of this.fieldSpellPlanes){this.scene.remove(plane);plane.geometry.dispose();plane.material.map?.dispose();plane.material.dispose();}
    this.fieldSpellPlanes=[];this.fieldSpellPlane=null;
    if(this.matMesh)this.matMesh.material.map=this.matTexture;
    if(!this.fieldSpellCode||!this.fieldImageProvider)return;
    const split=!!codes[0]&&!!codes[1]&&codes[0]!==codes[1];
    const entries=split?[{code:codes[0],seat:0},{code:codes[1],seat:1}]:[{code:this.fieldSpellCode,seat:-1}];
    for(const entry of entries){
      this.fieldImageProvider(entry.code).then(pic=>{
        if(!pic?.url||this.disposed||this.fieldSpellSignature!==signature)return;
        const tex=this.textureLoader.load(pic.url,()=>{
          if(this.disposed||this.fieldSpellSignature!==signature){tex.dispose();return;}
          tex.colorSpace=THREE.SRGBColorSpace;
          // vFieldSpell1/2 UV ranges from materials.cpp, including the mirrored far half.
          if(split){if(entry.seat===0){tex.repeat.set(1,.43636);tex.offset.set(0,.36364);}else{tex.repeat.set(-1,-.43636);tex.offset.set(1,.8);}}
          const geo=new (THREE as any).PlaneGeometry(11,split?4.8:12.8);
          const mat=new (THREE as any).MeshBasicMaterial({map:tex,depthWrite:false});
          const plane=new (THREE as any).Mesh(geo,mat);plane.rotation.x=-Math.PI/2;
          plane.position.set(-.1,.01,split?(entry.seat===0?4:-4):0);
          this.scene.add(plane);this.fieldSpellPlanes.push(plane);this.fieldSpellPlane=this.fieldSpellPlanes[0];
          this.matMesh.material.map=this.transparentMatTexture;
        });
      }).catch(()=>{});
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
    const quick = settingsStore.get('quick_animation');
    const speed = Number(settingsStore.get('animation_speed')) || 100;
    if (quick || speed !== 100) {
      // 动画速度（neos-ts animation 设置，animation_speed 百分比）：按
      // 100/speed 缩放时长；快速动画（原版 chkQuickAnimation）再封顶
      // 200ms（TimeScale 包装更精确，但 tween.js v25 的 group update
      // 不支持逐组 timeScale，直接改 duration）
      const origTo = tween.to.bind(tween);
      tween.to = (props: any, duration?: number) => {
        let d = duration;
        if (d !== undefined) {
          d = d * 100 / speed;
          if (quick && d >= 200) d = 200;
        }
        return origTo(props, d);
      };
    }
    this.tweenGroup.add(tween);
    return tween;
  }

  // 回放动画驱动：场上是否还有未播完的 tween。ReplayTheater 用它等一个
  // 事件触发的动画自然结束再推进下一步（代替固定时钟，避免动作一闪而过）。
  hasActiveTweens(): boolean {
    return this.tweenGroup.getAll().some((t) => t.isPlaying());
  }

  createCardMesh(cardCode: number, cardInfo: any = null, player: number | null = null): any {
    const geo = new (THREE as any).BoxGeometry(CARD_WIDTH, CARD_DEPTH, CARD_HEIGHT);
    const borderMat = new (THREE as any).MeshBasicMaterial({ color: 0x000000 });
    const frontMat = new (THREE as any).MeshBasicMaterial({
      map: this.generateCardTexture(cardCode),
    });
    // 里侧卡背按控制方取原版双色（drawing.cpp:425 tCover[controler]）
    const backMat = new (THREE as any).MeshBasicMaterial({
      map: player === 1 ? this.cardBackTexture2 : this.cardBackTexture,
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

  animateDestroy(mesh: any): void {
    runDestroy(this, mesh);
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
      // 槽位已有 mesh（如 MSG_MOVE 先行落位、SUMMONING/SET 再到）先静默清掉，
      // 防止 scene/cardMeshes 泄漏双份
      const existing = this.cardsOnField[player][locName][seq];
      if (existing) this.retireMesh(existing, true);
      const mesh = this.createCardMesh(cardCode, cardInfo, player);
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
      const mesh = this.createCardMesh(cardCode, cardInfo, player);
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

  // 原版 FadeCard（client_field.cpp:1107-1111）：凭空出现的卡（token 出场等
  // MSG_MOVE pl==0 路径）20 帧淡入。
  fadeInMesh(mesh: any, instant = false): void {
    if (!mesh || instant) return;
    const mats: any[] = Array.isArray(mesh.material) ? mesh.material : [mesh.material];
    mats.forEach((m) => { m.transparent = true; m.opacity = 0; });
    const state = { t: 0 };
    this.makeTween(state)
      .to({ t: 1 }, 20 * 1000 / 60)
      .onUpdate(() => { mats.forEach((m) => { m.opacity = state.t; }); })
      .onComplete(() => {
        mats.forEach((m) => { m.opacity = 1; m.transparent = false; m.needsUpdate = true; });
      })
      .start();
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
    this.handMeshes = [[], []];
    this.overlayMeshes = Array.from({length:2},()=>Array.from({length:7},()=>[]));
    this.cardsOnField = {
      0: { mzone: [], szone: [], grave: [], banish: [], deck: [], extra: [] },
      1: { mzone: [], szone: [], grave: [], banish: [], deck: [], extra: [] }
    };
    if (this.chainVisualizer) this.chainVisualizer.clearChain();
    this.clearRelationMarks();
    this.clearPlaceSelectZones();
    this.clearCardSelectMarks();
    this.clearHintZones();
    this.clearLinkedZones();
    this.clearStatusLabels();
    this.clearPileCounts();
    this.tweenGroup.removeAll();
    // Force the field-spell background to re-evaluate (and fade out).
    this.fieldSpellCode = -1;
    this.fieldSpellSignature = '!';
    this.refreshFieldSpell();
  }

  // 双方手牌的 3D 直排（client_field.cpp GetCardLocation LOCATION_HAND：
  // 同旋转、≤6 张固定间距、更多压缩；零码 = 卡背）。无飞入动画——
  // 抽牌动画由 animateDrawCard 负责，这里只把持久表示同步成最新张数。
  setHands(codes: [number[], number[]]): void {
    for (const seat of [0,1] as const) {
      const meshes = this.handMeshes[seat];
      while (meshes.length > codes[seat].length) this.retireMesh(meshes.pop(),true);
      codes[seat].forEach((code,seq) => {
        let mesh = meshes[seq];
        if (!mesh || mesh.userData.cardCode !== code) {
          if (mesh) this.retireMesh(mesh,true);
          mesh = this.createCardMesh(code, null, seat);
          meshes[seq] = mesh;
          mesh.userData.slot = {player:seat,loc:'hand',seq};
          this.scene.add(mesh);
        }
        const pose = handPose(seat,seq,codes[seat].length,mesh === this.hoveredCard);
        mesh.userData.handCount = codes[seat].length;
        mesh.userData.originalY = pose.y;
        if (mesh.userData.drawing) return;
        mesh.position.set(pose.x,pose.y,pose.z);
        mesh.rotation.set(seat===0?0.798056:-0.798056,0,0);
        if (!code) mesh.rotation.x += Math.PI;
      });
    }
  }

  syncOverlays(codes:number[][][]):void {
    for(const seat of [0,1])for(let host=0;host<7;host++){
      const meshes=this.overlayMeshes[seat][host],materials=codes[seat]?.[host]||[];
      while(meshes.length>materials.length)this.retireMesh(meshes.pop(),true);
      materials.forEach((code,index)=>{
        let mesh=meshes[index];
        if(!mesh||mesh.userData.cardCode!==code){
          if(mesh)this.retireMesh(mesh,true);
          mesh=this.createCardMesh(code);meshes[index]=mesh;this.scene.add(mesh);
        }
        mesh.userData.slot={player:seat,loc:'overlay',seq:host,material:index};
      });
    }
    this.layoutOverlays();
  }

  layoutOverlays():void {
    for(const seat of [0,1])for(let host=0;host<7;host++){
      const parent=this.getCardMesh(seat,'mzone',host);
      this.overlayMeshes[seat][host].forEach((mesh,index)=>{
        mesh.visible=!!parent&&index<6;
        if(!parent)return;
        const position=this.getZonePosition(seat,'mzone',host),sign=seat===0?1:-1,layer=Math.min(index,5);
        mesh.position.set(position.x+sign*(-.24+.12*layer),layer*.006,position.z+sign*.1);
        mesh.rotation.set(0,seat===0?0:Math.PI,0);
      });
    }
  }

  setHandHover(mesh: any, on: boolean): void {
    const slot = mesh.userData.slot;
    const pose = handPose(slot.player,slot.seq,mesh.userData.handCount,on);
    this.makeTween(mesh.position).to({x:pose.x,y:pose.y,z:pose.z},5*1000/60).start();
  }

  shuffleHand(player: number): void {
    const meshes = this.handMeshes[player as 0|1];
    for (const mesh of meshes) {
      const pose = handPose(player,mesh.userData.slot.seq,meshes.length);
      mesh.userData.shuffling = true;
      this.makeTween(mesh.position).to({x:-0.2},5*1000/60).chain(
        this.makeTween(mesh.position).to({x:pose.x},5*1000/60).delay(6*1000/60)
          .onComplete(()=>{mesh.userData.shuffling=false;})
      ).start();
    }
  }

  setDuelRule(rule: number): void {
    if (this.duelRule === rule) return;
    this.duelRule = rule;
    const suffix = rule >= 4 ? '3' : '2';
    this.matTexture?.dispose();
    this.transparentMatTexture?.dispose();
    this.matTexture = this.textureLoader.load('textures/field'+suffix+'.png');
    this.transparentMatTexture = this.textureLoader.load('textures/field-transparent'+suffix+'.png');
    this.matTexture.colorSpace = THREE.SRGBColorSpace;
    this.transparentMatTexture.colorSpace = THREE.SRGBColorSpace;
    this.matMesh.material.map = this.fieldSpellPlanes.length > 0 ? this.transparentMatTexture : this.matTexture;
    for (const seat of [0,1]) for (const loc of ['mzone','szone','grave','banish']) {
      this.cardsOnField[seat][loc].forEach((mesh:any,seq:number) => {
        if (!mesh) return;
        const p = this.getZonePosition(seat,loc,seq);
        mesh.visible = !!p;
        if (p) mesh.position.set(p.x,p.y+0.02,p.z);
      });
    }
    this.repositionGraveLockSprites();
  }

  setBottomSeat(seat: number, instant = false): void {
    const swap = seat === 1;
    this.localSeat = seat;
    if (instant) {
      this.camera.position.set(swap?-0.6:0.4,15.6,swap?-16:16);
      this.camera.lookAt(swap?-0.6:0.4,0,0);
      this.camera.updateMatrixWorld(true);
    }
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
      const c = this.getZonePosition(p,'grave');
      const stack = (this.cardsOnField[p] && this.cardsOnField[p].grave) || [];
      sprite.position.set(c.x, 0.07 + Math.min(stack.length, 20) * 0.02 + 0.15, c.z);
      sprite.scale.set(1.1, 1.1, 1);
    });
  }

  // ---- 场上卡无效化徽章（drawing.cpp:458-462）----
  // STATUS_DISABLED|STATUS_FORBIDDEN 且在场、表侧的卡的 mesh 上挂 negated
  // 小图标（Sprite 作为 mesh 子节点，随卡移动/销毁自动清理）。
  // 由 DuelManager 在 update_data/update_card 后按 store.board 的 status 同步。
  syncNegatedBadges(seat: number, locName: 'mzone' | 'szone', cards: ({ status?: number; pos: number } | null)[]): void {
    const STATUS_DISABLED = 0x0001;
    const STATUS_FORBIDDEN = 0x4000000;
    const zone = (this.cardsOnField[seat] || {})[locName] || [];
    const max = Math.max(zone.length, cards.length);
    for (let i = 0; i < max; i++) {
      const mesh = zone[i];
      if (!mesh) continue;
      const card = cards[i] || null;
      // pos & 0x5 = 表侧（POS_FACEUP_ATTACK|POS_FACEUP_DEFENSE）
      const show = !!card
        && (((card.status ?? 0) & (STATUS_DISABLED | STATUS_FORBIDDEN)) !== 0)
        && ((card.pos & 0x5) !== 0);
      const badge = mesh.userData.negatedBadge;
      if (show && !badge) {
        const sprite = new (THREE as any).Sprite(new (THREE as any).SpriteMaterial({
          map: this.negatedTexture, transparent: true, depthTest: false,
        }));
        // BoxGeometry 的 y 是卡面法线方向（CARD_DEPTH 厚度），微抬避免 z-fighting
        sprite.position.set(0, CARD_DEPTH, 0);
        sprite.scale.set(0.45, 0.45, 1);
        mesh.add(sprite);
        mesh.userData.negatedBadge = sprite;
      } else if (!show && badge) {
        mesh.remove(badge);
        badge.material.dispose();
        delete mesh.userData.negatedBadge;
      }
    }
  }

  // ---- 怪兽攻守状态浮签（drawing.cpp:771-786 DrawStatus）----
  // 原版是 2D 屏幕文本：本方 code!=0 即画、对方仅表侧；内容第一行等级串
  // （XYZ 阶级 #ff80ff / 调整 #ffff00 / 其他白；连接怪无等级）、第二行
  // atk / def（连接怪 atk / LINK-n，LINK 串 #99ffff）。攻/防相对原本值
  // 升高画 #ffff00、降低画 #ff2090、相等画白。这里译为 canvas→sprite，
  // 数据由 DuelManager.syncLinkData 写进 mesh.userData。

  /** DrawStatus 一卡的文本与配色（null = 不画） */
  statusContentFor(ud: any): {
    line1: string; line1Color: string;
    atkText: string; atkColor: string; defText: string; defColor: string;
  } | null {
    if (((ud.cardType || 0) & TYPE_MONSTER) === 0) return null;
    if (ud.attack === undefined) return null; // 数据未到（未 query 过攻守）不画
    const isLink = (ud.cardType & TYPE_LINK) !== 0;
    const colorOf = (cur: number, base: number | undefined): string =>
      base === undefined ? '#ffffff'
        : cur > base ? '#ffff00' : cur < base ? '#ff2090' : '#ffffff';
    const atkText = ud.attack < 0 ? '?' : String(ud.attack);
    const atkColor = ud.attack < 0 ? '#ffffff' : colorOf(ud.attack, ud.baseAttack);
    let line1 = '';
    let line1Color = '#ffffff';
    let defText: string;
    let defColor: string;
    if (isLink) {
      defText = `LINK-${ud.link || 0}`;
      defColor = '#99ffff';
    } else {
      const isXyz = (ud.cardType & TYPE_XYZ) !== 0;
      line1 = isXyz ? `R${ud.rank || 0}` : `☆${ud.level || 0}`;
      line1Color = isXyz ? '#ff80ff' : (ud.cardType & TYPE_TUNER) ? '#ffff00' : '#ffffff';
      const def = ud.defense ?? 0;
      defText = def < 0 ? '?' : String(def);
      defColor = def < 0 ? '#ffffff' : colorOf(def, ud.baseDefense);
    }
    return { line1, line1Color, atkText, atkColor, defText, defColor };
  }

  /** canvas → status sprite（仿 renderCounterBadge；影子偏移复刻 DrawShadowText） */
  buildStatusSprite(content: NonNullable<ReturnType<DuelField3D['statusContentFor']>>): any {
    const canvas = document.createElement('canvas');
    canvas.width = 256;
    canvas.height = 96;
    const ctx = canvas.getContext('2d')!;
    ctx.clearRect(0, 0, 256, 96);
    ctx.textBaseline = 'middle';
    const drawShadowed = (text: string, x: number, y: number, color: string, align: CanvasTextAlign) => {
      ctx.textAlign = align;
      ctx.fillStyle = '#000000';
      ctx.fillText(text, x + 2, y + 2, 240);
      ctx.fillStyle = color;
      ctx.fillText(text, x, y, 240);
    };
    if (content.line1) {
      ctx.font = 'bold 30px sans-serif';
      drawShadowed(content.line1, 128, 24, content.line1Color, 'center');
    }
    // 第二行 atk / def 三段异色：量宽后整体居中
    ctx.font = 'bold 32px sans-serif';
    const segs: [string, string][] = [
      [content.atkText, content.atkColor], [' / ', '#ffffff'], [content.defText, content.defColor],
    ];
    const widths = segs.map(([t]) => ctx.measureText(t).width);
    let x = (256 - widths.reduce((a, b) => a + b, 0)) / 2;
    segs.forEach(([t, color], i) => {
      drawShadowed(t, x, 66, color, 'left');
      x += widths[i];
    });
    const tex = new (THREE as any).CanvasTexture(canvas);
    const sprite = new (THREE as any).Sprite(new (THREE as any).SpriteMaterial({
      map: tex, transparent: true, depthTest: false,
    }));
    sprite.scale.set(2.3, 0.86, 1);
    sprite.renderOrder = 10;
    // 元数据供冒烟断言（颜色规则与文本内容不必解析像素）
    sprite.userData.isStatusLabel = true;
    sprite.userData.statusText = { line1: content.line1, line2: `${content.atkText} / ${content.defText}` };
    sprite.userData.statusColors = {
      line1: content.line1Color, atk: content.atkColor, def: content.defColor,
    };
    sprite.userData.statusFingerprint = JSON.stringify(sprite.userData.statusText)
      + JSON.stringify(sprite.userData.statusColors);
    return sprite;
  }

  disposeTextSprite(sprite: any): void {
    this.scene.remove(sprite);
    if (sprite.material.map) sprite.material.map.dispose();
    sprite.material.dispose();
  }

  /**
   * 按 mesh.userData（syncLinkData 写入）维护全场怪兽的状态浮签；幂等——
   * 指纹未变的 sprite 不重画。ownSeat = 本方引擎 seat（原版 controler 0
   * 的判定：本方 code!=0 即画，对方仅表侧 pos&0x5）。
   */
  syncStatusLabels(ownSeat = 0): void {
    const wanted = new Set<any>();
    for (const seat of [0, 1]) {
      const zone = (this.cardsOnField[seat] || {}).mzone || [];
      for (const mesh of zone) {
        if (!mesh) continue;
        const ud = mesh.userData;
        const content = this.statusContentFor(ud);
        if (!content) continue;
        const faceUp = ud.cardPos !== undefined
          ? (ud.cardPos & 0x5) !== 0
          : ud.positionState !== 'FACEDOWN';
        if (seat === ownSeat ? !ud.cardCode : !faceUp) continue;
        wanted.add(mesh);
        const existing = this.statusLabels.find((e) => e.mesh === mesh);
        const fingerprint = JSON.stringify({ line1: content.line1, line2: `${content.atkText} / ${content.defText}` })
          + JSON.stringify({ line1: content.line1Color, atk: content.atkColor, def: content.defColor });
        if (existing && existing.sprite.userData.statusFingerprint === fingerprint) continue;
        if (existing) {
          this.disposeTextSprite(existing.sprite);
          this.statusLabels.splice(this.statusLabels.indexOf(existing), 1);
        }
        const sprite = this.buildStatusSprite(content);
        sprite.position.set(mesh.position.x, mesh.position.y + 0.35,
          mesh.position.z + (seat === 0 ? 0.55 : -0.55));
        this.scene.add(sprite);
        this.statusLabels.push({ sprite, mesh, seat });
      }
    }
    // 离场/变里侧/变非怪的卡摘除浮签
    this.statusLabels = this.statusLabels.filter((e) => {
      if (wanted.has(e.mesh)) return true;
      this.disposeTextSprite(e.sprite);
      return false;
    });
  }

  clearStatusLabels(): void {
    for (const e of this.statusLabels) this.disposeTextSprite(e.sprite);
    this.statusLabels = [];
  }

  // ---- 牌堆计数浮签（drawing.cpp:724-769 DrawMisc：四堆旁黄色数字）----
  // counts 按引擎 seat 索引（换算在 DuelManager 做）；>0 才画，幂等。

  buildPileCountSprite(text: string): any {
    const canvas = document.createElement('canvas');
    canvas.width = 96;
    canvas.height = 64;
    const ctx = canvas.getContext('2d')!;
    ctx.clearRect(0, 0, 96, 64);
    ctx.font = 'bold 44px sans-serif';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillStyle = '#000000';
    ctx.fillText(text, 50, 34);
    ctx.fillStyle = '#ffff00';
    ctx.fillText(text, 48, 32);
    const tex = new (THREE as any).CanvasTexture(canvas);
    const sprite = new (THREE as any).Sprite(new (THREE as any).SpriteMaterial({
      map: tex, transparent: true, depthTest: false,
    }));
    sprite.scale.set(1.05, 0.7, 1);
    sprite.renderOrder = 10;
    sprite.userData.isPileCount = true;
    sprite.userData.pileText = text;
    return sprite;
  }

  syncPileCounts(counts: ({ deck: number; grave: number; banish: number; extra: number; extraP?: number })[]): void {
    for (const seat of [0, 1]) {
      for (const pile of ['deck', 'extra', 'grave', 'banish'] as const) {
        // 原版额外堆双数字（drawing.cpp:726-727,749-750）：总数 + 表侧数
        const n = (counts[seat] && counts[seat][pile]) || 0;
        const text = pile === 'extra' && counts[seat] && counts[seat].extraP
          ? `${n}(${counts[seat].extraP})` : String(n);
        const key = `${seat}:${pile}`;
        const existing = this.pileCountSprites[key];
        if (n <= 0) {
          if (existing) {
            this.disposeTextSprite(existing);
            delete this.pileCountSprites[key];
          }
          continue;
        }
        if (existing && existing.userData.pileText === text) continue;
        if (existing) this.disposeTextSprite(existing);
        const sprite = this.buildPileCountSprite(text);
        sprite.userData.pilePlayer = seat;
        sprite.userData.pile = pile;
        const c = this.getZonePosition(seat, pile, 0);
        sprite.position.set(c.x, c.y + 0.35, c.z);
        this.scene.add(sprite);
        this.pileCountSprites[key] = sprite;
      }
    }
  }

  // 原版 MSG_SHUFFLE_DECK 的卡组抖动（duelclient.cpp:2263-2301 共 5 轮）：
  // 卡组堆无实体卡 mesh，抖动落在计数浮签上（正弦往返 5 次）。
  shakeDeckPile(seat: number): void {
    const sprite = this.pileCountSprites[`${seat}:deck`];
    if (!sprite) return;
    const baseX = sprite.position.x;
    const state = { t: 0 };
    this.makeTween(state)
      .to({ t: 1 }, 5 * 2 * 2000 / 60)
      .onUpdate(() => {
        sprite.position.x = baseX + Math.sin(state.t * Math.PI * 2 * 5) * 0.12;
      })
      .onComplete(() => { sprite.position.x = baseX; })
      .start();
  }

  clearPileCounts(): void {
    for (const key of Object.keys(this.pileCountSprites)) {
      this.disposeTextSprite(this.pileCountSprites[key]);
    }
    this.pileCountSprites = {};
  }

  /** 逐帧跟随：状态浮签贴卡（z 向持卡方屏幕侧偏移），计数浮签随堆高抬升 */
  repositionTextSprites(): void {
    for (const e of this.statusLabels) {
      e.sprite.position.set(e.mesh.position.x, e.mesh.position.y + 0.35,
        e.mesh.position.z + (e.seat === 0 ? 0.55 : -0.55));
    }
    for (const key of Object.keys(this.pileCountSprites)) {
      const sprite = this.pileCountSprites[key];
      const seat = sprite.userData.pilePlayer;
      const pile = sprite.userData.pile;
      const c = this.getZonePosition(seat,pile);
      if (!c) continue;
      const stack = (this.cardsOnField[seat] && this.cardsOnField[seat][pile]) || [];
      sprite.position.set(c.x, c.y + 0.35 + Math.min(stack.length, 20) * 0.02, c.z);
    }
  }

  // ---- 波 B 渲染原语：指示物 / 装备线 / 目标线 / 翻开确认 ----

  // 按引擎 LOCATION 码取一张场卡 mesh（hand/deck/extra 无 mesh 返回 null；
  // 堆区按序号取栈顶近似）
  meshAt(player: number, loc: number, seq: number): any {
    const locName = LOC_NAMES[loc];
    if (locName === 'hand') return this.handMeshes[player as 0|1][seq] || null;
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

  setRelationMarks(marks: { mesh: any; kind: 'equip' | 'target' | 'chaintarget' }[]): void {
    this.clearRelationMarks();
    for (const mark of marks || []) {
      const tex = mark.kind === 'equip' ? this.equipTexture
        : mark.kind === 'target' ? this.targetTexture : this.chainTargetTexture;
      // 原版 vSymbol：0.7×0.7 平铺在卡面中心（materials.cpp:19）
      const geo = new (THREE as any).PlaneGeometry(0.7, 0.7);
      const mat = new (THREE as any).MeshBasicMaterial({
        map: tex, transparent: true, depthWrite: false,
      });
      const icon = new (THREE as any).Mesh(geo, mat);
      icon.rotation.x = -Math.PI / 2;
      icon.position.set(mark.mesh.position.x, mark.mesh.position.y + 0.06, mark.mesh.position.z);
      icon.renderOrder = 6;
      icon.userData.relationMark = mark.kind;
      this.scene.add(icon);
      this.relationMarks.push({ icon, mesh: mark.mesh });
      // 原版 else-if 链（equip > target > chaintarget > negated）：
      // 关系图标显示期间该卡的无效化徽章不画
      const badge = mark.mesh.userData.negatedBadge;
      if (badge) badge.visible = false;
    }
  }

  clearRelationMarks(): void {
    for (const m of this.relationMarks) {
      this.scene.remove(m.icon);
      m.icon.geometry.dispose();
      m.icon.material.dispose();
      const badge = m.mesh.userData.negatedBadge;
      if (badge) badge.visible = true;
    }
    this.relationMarks = [];
  }

  // MSG_CONFIRM_CARDS：揭示序列（原版 duelclient.cpp:2161-2260 的简化版）——
  // 盖卡先做一次翻开抖动（绕长边/短边的小角度旋转），再叠黄色高亮闪光
  flashCards(cards: { c: number; l: number; s: number }[]): void {
    for (const e of cards || []) {
      const mesh = this.meshAt(e.c, e.l, e.s);
      if (!mesh) continue;
      // 翻开抖动：表侧卡只抬升轻晃；里侧卡（set 状态）绕边翻转半周回弹，
      // 近似原版 pcard->dRot 的 36° 旋转揭示
      const facedown = mesh.userData.positionState === 'FACEDOWN';
      const lift = { y: mesh.position.y + 0.35 };
      this.makeTween(mesh.position)
        .to(lift, 140)
        .yoyo(true)
        .repeat(1)
        .easing(TWEEN.Easing.Quadratic.Out)
        .start();
      if (facedown) {
        const axis = (mesh.userData.slot && (mesh.userData.slot.loc === 'mzone' || mesh.userData.slot.loc === 'szone'))
          && mesh.rotation.y !== 0 ? 'x' : 'y';
        const rot = { ...mesh.rotation };
        this.makeTween(mesh.rotation)
          .to({ [axis]: (rot[axis] || 0) + Math.PI / 5 } as any, 160)
          .yoyo(true)
          .repeat(1)
          .easing(TWEEN.Easing.Quadratic.InOut)
          .onComplete(() => mesh.rotation.set(rot.x, rot.y, rot.z))
          .start();
      }
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

  // MSG_SUMMONED/SPSUMMONED/FLIPSUMMONED：召唤落定的顿点表现——怪兽槽位
  // 小冲击波 + 卡的轻微压实弹跳（原版只有日志事件串，这里是轻量增强）
  settleCard(_player: number, _slot: number): void {}

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
      this.updateMouseFromEvent(e);
      this._lastClientX = e.clientX;
      this._lastClientY = e.clientY;
      this.handleHover();
    };
    this._onClick = (e: MouseEvent) => {
      this.updateMouseFromEvent(e);
      this.handleHover();
      // 落点选择模式优先：点到高亮格 → 交给 manager 累计选位；
      // 点空处吞掉点击（原版选位期间不响应其他场上点击）
      if (this.placeSelectMarks.length) {
        const hit = this.pickPlaceZone();
        if (hit && this.onPlaceZoneClick) this.onPlaceZoneClick(hit.zone);
        return;
      }
      // select_card 场上点选模式：点可选卡 → 交给 manager 累计/应答；
      // 点空处同样吞掉（原版选择期间不响应其他场上动作）
      if (this.cardSelectMarks.length) {
        const hit = this.pickCardSelect();
        if (hit && this.onCardSelectPick) this.onCardSelectPick(hit.idx);
        return;
      }
      if (this.hoveredCard && this.onCardClick) {
        e.stopPropagation();
        // Screen coordinates let the caller position an action popup (e.g.
        // the battle-phase attack menu) right where the player clicked.
        this.onCardClick(e.clientX, e.clientY, this.hoveredCard.userData);
      } else if (this.onPileLeftClick) {
        // 左键点堆区（空堆或堆旁）：打开列表查看（原版 MSG_WAITING 点堆区
        // ShowMenu(COMMAND_LIST)；堆顶卡的点击走上面 hoveredCard 分支）
        const pile = this.pickPileZone();
        if (pile) this.onPileLeftClick(pile.player, pile.pile);
      }
    };
    this._onContextMenu = (e: MouseEvent) => {
      // 原版右键语义（event_handler.cpp RMOUSE_LEFT_UP）：关闭菜单/取消
      // 当前选择；点在墓地/除外/额外/卡组堆区上 = 快速查看列表
      // （gframe F1-F8 同源的 wCardDisplay）。
      e.preventDefault();
      this.updateMouseFromEvent(e);
      const pile = this.pickPileZone();
      if (pile && this.onPileRightClick) {
        this.onPileRightClick(pile.player, pile.pile);
      } else if (this.onBoardRightClick) {
        this.onBoardRightClick(e.clientX, e.clientY);
      }
    };

    window.addEventListener('resize', this._onResize);
    this.container.addEventListener('mousemove', this._onMouseMove);
    this.container.addEventListener('click', this._onClick);
    this.container.addEventListener('contextmenu', this._onContextMenu);
  }

  updateMouseFromEvent(e: MouseEvent): void {
    const rect = this.container.getBoundingClientRect();
    this.mouse.x = ((e.clientX - rect.left) / rect.width) * 2 - 1;
    this.mouse.y = -((e.clientY - rect.top) / rect.height) * 2 + 1;
  }

  /** 当前鼠标下的可选落点高亮（无则 null） */
  pickPlaceZone(): { mesh: any; zone: { player: number; loc: number; seq: number } } | null {
    if (!this.placeSelectMarks.length) return null;
    this.raycaster.setFromCamera(this.mouse, this.camera);
    const hits = this.raycaster.intersectObjects(this.placeSelectMarks.map((m) => m.mesh));
    return hits.length ? this.placeSelectMarks.find((m) => m.mesh === hits[0].object) || null : null;
  }

  /**
   * 鼠标命中的堆区（grave/banish/extra/deck，双方）。堆区没有 mesh，
   * 用盘面交点与 ZONE_COORDS 的堆区中心做矩形粗匹配（含卡组位）。
   */
  pickPileZone(): { player: number; pile: string } | null {
    this.raycaster.setFromCamera(this.mouse, this.camera);
    const targets = [this.matMesh, ...this.cardMeshes].filter(Boolean);
    const hits = this.raycaster.intersectObjects(targets);
    if (!hits.length) return null;
    const p = hits[0].point;
    for (const player of [0, 1]) {
      for (const pile of ['grave', 'banish', 'extra', 'deck']) {
        const c = this.getZonePosition(player,pile);
        if (!c) continue;
        if (Math.abs(p.x - c.x) <= 0.95 && Math.abs(p.z - c.z) <= 1.25) {
          return { player, pile };
        }
      }
    }
    return null;
  }

  handleHover(): void {
    this.scene.updateMatrixWorld(true);
    this.camera.updateMatrixWorld(true);
    // 原版 MOUSE_MOVED 与选择状态无关（event_handler.cpp:1507-1704）：落点/
    // 选卡点选期间悬停照常更新卡片详情与 stTip，仅光标体现可选态
    let selectCursor: string | null = null;
    if (this.placeSelectMarks.length) {
      selectCursor = this.pickPlaceZone() ? 'pointer' : 'default';
    } else if (this.cardSelectMarks.length) {
      selectCursor = this.pickCardSelect() ? 'pointer' : 'default';
    }
    this.container.style.cursor = selectCursor ?? 'default';
    this.raycaster.setFromCamera(this.mouse, this.camera);
    const intersects = this.raycaster.intersectObjects(this.cardMeshes);

    if (intersects.length > 0) {
      const mesh = intersects[0].object;
      if (this.hoveredCard !== mesh) {
        if (this.hoveredCard) {
          if (this.hoveredCard.userData.slot?.loc === 'hand') this.setHandHover(this.hoveredCard,false);
        }
        this.hoveredCard = mesh;
        // 悬停关系图标重算（client_field.cpp SetShowMark 的触发点）
        this.onHoverChange?.(mesh);
        if (mesh.userData.slot?.loc === 'hand') this.setHandHover(mesh,true);

        if (this.onCardInspect && mesh.userData.cardCode) {
          this.onCardInspect(mesh.userData.cardCode, mesh.userData.cardInfo);
        }
        // 悬停连接怪 → 互连区域高亮。
        this.updateLinkedZones();
      }
        // stTip tracks movement even while the same card remains hovered.
        this.container.dispatchEvent(new CustomEvent('ygo:cardhover', {
          detail: {
            code: mesh.userData.cardCode,
            info: mesh.userData.cardInfo,
            // 场上实时数值（syncLinkData 写入，DrawStatus 同源）：tooltip 的
            // 攻守/星阶优先用它；手牌/堆区等无实时值的区域为 undefined
            live: {
              attack: mesh.userData.attack, defense: mesh.userData.defense,
              level: mesh.userData.level, rank: mesh.userData.rank,
              link: mesh.userData.link,
            },
            x: this._lastClientX, y: this._lastClientY,
            location:mesh.userData.slot?.loc,
          },
        }));
    } else if (this.hoveredCard) {
      if (this.hoveredCard.userData.slot?.loc === 'hand') this.setHandHover(this.hoveredCard,false);
      this.hoveredCard = null;
      this.onHoverChange?.(null);
      this.container.dispatchEvent(new CustomEvent('ygo:cardhoverend'));
      this.clearLinkedZones();
    }
  }

  // 原版投影视锥左右不对称（game.cpp:994 BuildProjectionMatrix 左 -0.90 /
  // 右 0.45，中心偏左约 16.7%）：等效于把盘面往右推，给左侧 HUD 面板留位。
  // 用 three 的 setViewOffset 复刻；窄屏（面板隐藏）时取消偏移。
  applyViewOffset(): void {
    // game.cpp's fixed asymmetric projection; window resizing stretches it.
    this.camera.projectionMatrix.makePerspective(-1.8,0.9,0.84,-0.84,2,200);
    this.camera.projectionMatrixInverse.copy(this.camera.projectionMatrix).invert();
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
    this.layoutOverlays();
    // 原版 attack.png 标记上下浮动（drawing.cpp atkdy 正弦）、
    // act.png 图标匀速自旋（原版 act_rot.Z 每帧 +0.02）
    const t = performance.now();
    for (const m of this.attackMarks) {
      m.position.y = m.userData.baseY + Math.sin(t / 320 + m.userData.phase) * 0.16;
    }
    for (const m of this.actMarks) {
      m.material.rotation += 0.02;
    }
    // 落点高亮呼吸（drawing.cpp:144-145 selFieldAlpha 255↔25 往返）
    for (const m of this.placeSelectMarks) {
      if (!m.mesh.userData.selected) {
        m.mesh.material.opacity = 0.45 + 0.35 * (0.5 + 0.5 * Math.sin(t / 240));
      }
    }
    this.repositionTextSprites();
    // 可选卡黄框同样呼吸（与落点高亮同节律）
    for (const m of this.cardSelectMarks) {
      if (!m.mesh.userData.selected) {
        m.mesh.material.opacity = 0.45 + 0.35 * (0.5 + 0.5 * Math.sin(t / 240));
      }
    }
    this.chainVisualizer?.tick();
    for (const mark of this.cardSelectMarks) {
      mark.mesh.position.copy(mark.cardMesh.position);
      mark.mesh.position.y += 0.04;
      mark.mesh.quaternion.copy(mark.cardMesh.quaternion);
      mark.mesh.rotateX(-Math.PI/2);
    }
    // 关系图标逐帧跟随卡 mesh（悬停中卡可能移动/被抬升）
    for (const m of this.relationMarks) {
      m.icon.position.set(m.mesh.position.x, m.mesh.position.y + 0.06, m.mesh.position.z);
    }
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
      this.container.removeEventListener('contextmenu', this._onContextMenu);
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
      this.clearPlaceSelectZones();
      this.clearCardSelectMarks();
      this.clearHintZones();
      this.clearLinkedZones();
      this.clearStatusLabels();
      this.clearPileCounts();
      this.setFieldDisabled(0);
    }
    this.cardTextureCache.forEach((tex) => tex.dispose());
    this.cardTextureCache.clear();
    if (this.cardBackTexture) this.cardBackTexture.dispose();
    if (this.cardBackTexture2) this.cardBackTexture2.dispose();
    for (const t of [this.equipTexture, this.targetTexture, this.chainTargetTexture]) {
      if (t) t.dispose();
    }
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
    this.clearRelationMarks();
    if (this.placeSelectTextures) {
      this.placeSelectTextures.on.dispose();
      this.placeSelectTextures.off.dispose();
      this.placeSelectTextures = null;
    }
    if (this.cardSelectTextures) {
      this.cardSelectTextures.on.dispose();
      this.cardSelectTextures.off.dispose();
      this.cardSelectTextures = null;
    }
    if (this.linkZoneTextures) {
      this.linkZoneTextures.link.dispose();
      this.linkZoneTextures.mutual.dispose();
      this.linkZoneTextures = null;
    }
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
