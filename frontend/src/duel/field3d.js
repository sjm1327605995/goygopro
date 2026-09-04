/**
 * Three.js 3D Duel Field System
 * Complete 3D board rendering, card animations, raycasting, chain visualizer, and particle FX.
 */

import * as THREE from '../../libs/three.module.js';
import * as TWEEN from '../../libs/tween.esm.js';
import { soundManager } from '../audio/sound_manager.js';
import { ChainVisualizer } from './chain_visualizer.js';

// Card dimensions in 3D world units
export const CARD_WIDTH = 1.6;
export const CARD_HEIGHT = 2.3;
export const CARD_DEPTH = 0.03;

// Field Zone coordinate definitions
export const ZONE_COORDS = {
  // Player 0 Zones (z positive, near camera). Rows are pitched 2.6 apart in z
  // so a 2.3-unit card (2.45-unit slot frame) clears its neighbours: this keeps
  // the EMZ (z=0) from overlapping the monster row.
  0: {
    mzone: [
      { x: -3.8, y: 0.02, z: 2.6 },
      { x: -1.9, y: 0.02, z: 2.6 },
      { x: 0.0,  y: 0.02, z: 2.6 },
      { x: 1.9,  y: 0.02, z: 2.6 },
      { x: 3.8,  y: 0.02, z: 2.6 }
    ],
    szone: [
      { x: -3.8, y: 0.02, z: 5.2 },
      { x: -1.9, y: 0.02, z: 5.2 },
      { x: 0.0,  y: 0.02, z: 5.2 },
      { x: 1.9,  y: 0.02, z: 5.2 },
      { x: 3.8,  y: 0.02, z: 5.2 }
    ],
    field:     { x: -5.8, y: 0.02, z: 2.6 },
    grave:     { x: 5.8,  y: 0.02, z: 2.6 },
    banish:    { x: 5.8,  y: 0.02, z: 5.2 },
    deck:      { x: 5.8,  y: 0.02, z: 7.8 },
    extra:     { x: -5.8, y: 0.02, z: 7.8 },
    emz_left:  { x: -1.9, y: 0.02, z: 0.0 },
    emz_right: { x: 1.9,  y: 0.02, z: 0.0 }
  },
  // Opponent 1 Zones (Rotated 180 deg)
  1: {
    mzone: [
      { x: 3.8,  y: 0.02, z: -2.6 },
      { x: 1.9,  y: 0.02, z: -2.6 },
      { x: 0.0,  y: 0.02, z: -2.6 },
      { x: -1.9, y: 0.02, z: -2.6 },
      { x: -3.8, y: 0.02, z: -2.6 }
    ],
    szone: [
      { x: 3.8,  y: 0.02, z: -5.2 },
      { x: 1.9,  y: 0.02, z: -5.2 },
      { x: 0.0,  y: 0.02, z: -5.2 },
      { x: -1.9, y: 0.02, z: -5.2 },
      { x: -3.8, y: 0.02, z: -5.2 }
    ],
    field:     { x: 5.8,  y: 0.02, z: -2.6 },
    grave:     { x: -5.8, y: 0.02, z: -2.6 },
    banish:    { x: -5.8, y: 0.02, z: -5.2 },
    deck:      { x: -5.8, y: 0.02, z: -7.8 },
    extra:     { x: 5.8,  y: 0.02, z: -7.8 }
  }
};

export class DuelField3D {
  constructor(containerElement, onCardInspect, onCardClick) {
    this.container = containerElement;
    this.onCardInspect = onCardInspect;
    this.onCardClick = onCardClick;

    this.scene = null;
    this.camera = null;
    this.renderer = null;
    this.raycaster = new THREE.Raycaster();
    this.mouse = new THREE.Vector2();

    this.cardsOnField = {
      0: { mzone: [], szone: [], grave: [], banish: [], deck: [], extra: [] },
      1: { mzone: [], szone: [], grave: [], banish: [], deck: [], extra: [] }
    };
    this.cardMeshes = [];
    this.hoveredCard = null;
    this.textureLoader = new THREE.TextureLoader();
    this.cardBackTexture = null;
    this.cardTextureCache = new Map();
    // Optional async (code) => Promise<dataURL|null> set by the host; when a
    // picture arrives the cached canvas texture is redrawn in place so every
    // mesh sharing it picks the art up on the next frame.
    this.cardImageProvider = null;
    this.chainVisualizer = null;
    this.disposed = false;
    // tween.js v25 no longer auto-registers `new Tween(obj)` with the global
    // group, so all card animations must live in an explicit group that
    // animate() drives. Without this the tweens are orphaned and cards stay
    // frozen at their animation start positions (floating above the board).
    this.tweenGroup = new TWEEN.Group();

    this.initScene();
    this.initLights();
    this.initField();
    this.initDeckStacks();
    this.chainVisualizer = new ChainVisualizer(this.scene, this);
    this.initEvents();
    this.animate();
  }

  initScene() {
    this.scene = new THREE.Scene();
    this.scene.background = new THREE.Color(0x0a0e17);
    this.scene.fog = new THREE.FogExp2(0x0a0e17, 0.025);

    const width = this.container.clientWidth || window.innerWidth;
    const height = this.container.clientHeight || window.innerHeight;

    this.camera = new THREE.PerspectiveCamera(45, width / height, 0.1, 100);
    this.camera.position.set(0, 13.5, 12.0);
    this.camera.lookAt(0, -0.5, 0.5);

    this.renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true });
    this.renderer.setSize(width, height);
    this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
    this.renderer.shadowMap.enabled = true;
    this.renderer.shadowMap.type = THREE.PCFSoftShadowMap;

    this.container.appendChild(this.renderer.domElement);
    this.cardBackTexture = this.textureLoader.load('textures/cover.jpg');
  }

  initLights() {
    const ambientLight = new THREE.AmbientLight(0xffffff, 0.9);
    this.scene.add(ambientLight);

    const dirLight = new THREE.DirectionalLight(0xe2e8f0, 1.2);
    dirLight.position.set(0, 20, 10);
    dirLight.castShadow = true;
    dirLight.shadow.mapSize.width = 2048;
    dirLight.shadow.mapSize.height = 2048;
    this.scene.add(dirLight);

    const cyanLight = new THREE.PointLight(0x00d2ff, 1.5, 25);
    cyanLight.position.set(-8, 3, 0);
    this.scene.add(cyanLight);

    const purpleLight = new THREE.PointLight(0x8b5cf6, 1.5, 25);
    purpleLight.position.set(8, 3, 0);
    this.scene.add(purpleLight);
  }

  initField() {
    const arenaGeo = new THREE.PlaneGeometry(24, 18);
    const arenaMat = new THREE.MeshStandardMaterial({
      color: 0x0d1527,
      roughness: 0.8,
      metalness: 0.2
    });
    const arenaMesh = new THREE.Mesh(arenaGeo, arenaMat);
    arenaMesh.rotation.x = -Math.PI / 2;
    arenaMesh.receiveShadow = true;
    this.scene.add(arenaMesh);

    const gridHelper = new THREE.GridHelper(22, 22, 0x00d2ff, 0x1e293b);
    gridHelper.position.y = 0.01;
    this.scene.add(gridHelper);

    this.buildZoneSlots();
  }

  buildZoneSlots() {
    const slotGeo = new THREE.PlaneGeometry(CARD_WIDTH + 0.15, CARD_HEIGHT + 0.15);
    const createSlotMesh = (pos, color = 0x00d2ff) => {
      const edges = new THREE.EdgesGeometry(slotGeo);
      const line = new THREE.LineSegments(
        edges,
        new THREE.LineBasicMaterial({ color, transparent: true, opacity: 0.5, linewidth: 2 })
      );
      line.rotation.x = -Math.PI / 2;
      line.position.set(pos.x, pos.y, pos.z);
      this.scene.add(line);

      const glowMat = new THREE.MeshBasicMaterial({
        color,
        transparent: true,
        opacity: 0.08,
        side: THREE.DoubleSide
      });
      const glowMesh = new THREE.Mesh(slotGeo, glowMat);
      glowMesh.rotation.x = -Math.PI / 2;
      glowMesh.position.set(pos.x, pos.y, pos.z);
      this.scene.add(glowMesh);
    };

    ZONE_COORDS[0].mzone.forEach(p => createSlotMesh(p, 0x00d2ff));
    ZONE_COORDS[0].szone.forEach(p => createSlotMesh(p, 0x10b981));
    createSlotMesh(ZONE_COORDS[0].field, 0x8b5cf6);
    createSlotMesh(ZONE_COORDS[0].grave, 0x64748b);
    createSlotMesh(ZONE_COORDS[0].banish, 0x475569);
    createSlotMesh(ZONE_COORDS[0].deck, 0xf59e0b);
    createSlotMesh(ZONE_COORDS[0].extra, 0xec4899);
    createSlotMesh(ZONE_COORDS[0].emz_left, 0x06b6d4);
    createSlotMesh(ZONE_COORDS[0].emz_right, 0x06b6d4);

    ZONE_COORDS[1].mzone.forEach(p => createSlotMesh(p, 0xef4444));
    ZONE_COORDS[1].szone.forEach(p => createSlotMesh(p, 0xf97316));
    createSlotMesh(ZONE_COORDS[1].field, 0x8b5cf6);
    createSlotMesh(ZONE_COORDS[1].grave, 0x64748b);
    createSlotMesh(ZONE_COORDS[1].banish, 0x475569);
    createSlotMesh(ZONE_COORDS[1].deck, 0xf59e0b);
    createSlotMesh(ZONE_COORDS[1].extra, 0xec4899);
  }

  initDeckStacks() {
    const createDeckStack = (pos) => {
      const stackGeo = new THREE.BoxGeometry(CARD_WIDTH, 0.4, CARD_HEIGHT);
      const stackMat = new THREE.MeshStandardMaterial({
        color: 0x1e293b,
        roughness: 0.6,
        metalness: 0.3
      });
      const stackMesh = new THREE.Mesh(stackGeo, stackMat);
      stackMesh.position.set(pos.x, 0.2, pos.z);
      stackMesh.castShadow = true;
      stackMesh.receiveShadow = true;
      this.scene.add(stackMesh);
    };

    createDeckStack(ZONE_COORDS[0].deck);
    createDeckStack(ZONE_COORDS[0].extra);
    createDeckStack(ZONE_COORDS[1].deck);
    createDeckStack(ZONE_COORDS[1].extra);
  }

  // Single source of truth for zone coordinates. External renderers (e.g. the
  // ChainVisualizer) call this instead of keeping their own copy of ZONE_COORDS
  // so the layout cannot drift out of sync.
  getZonePosition(player, loc, seq = 0) {
    const zone = ZONE_COORDS[player] && ZONE_COORDS[player][loc];
    if (!zone) return { x: 0, y: 0.02, z: 0 };
    if (Array.isArray(zone)) {
      const pos = zone[seq];
      return pos ? { ...pos } : { x: 0, y: 0.02, z: 0 };
    }
    return { ...zone };
  }

  // Board-facing rotation for a card in `position` (ocgcore position bits:
  // POS_DEFENSE = 0xc, POS_FACEDOWN = 0xa). Defense turns the card sideways
  // about y (the face normal) — a z-rotation would tip it vertical.
  cardRotationFor(player, position) {
    const isDef = (position & 0xc) !== 0;
    const isFaceDown = (position & 0xa) !== 0;
    return {
      x: isFaceDown ? Math.PI : 0,
      y: (player === 1 ? Math.PI : 0) + (isDef ? Math.PI / 2 : 0),
      z: 0
    };
  }

  // Creates a tween registered with this field's animation group. tween.js v25
  // requires explicit group membership; the global TWEEN.update() no longer
  // picks up `new Tween(obj)` created without a group.
  makeTween(obj) {
    const tween = new TWEEN.Tween(obj);
    this.tweenGroup.add(tween);
    return tween;
  }

  createCardMesh(cardCode, cardInfo = null) {
    const geo = new THREE.BoxGeometry(CARD_WIDTH, CARD_DEPTH, CARD_HEIGHT);
    const borderMat = new THREE.MeshStandardMaterial({ color: 0x111827 });
    const frontMat = new THREE.MeshStandardMaterial({
      map: this.generateCardTexture(cardCode, cardInfo),
      roughness: 0.4,
      metalness: 0.1
    });
    const backMat = new THREE.MeshStandardMaterial({
      map: this.cardBackTexture,
      roughness: 0.4,
      metalness: 0.1
    });

    const materials = [borderMat, borderMat, frontMat, backMat, borderMat, borderMat];
    const cardMesh = new THREE.Mesh(geo, materials);
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

  generateCardTexture(cardCode, cardInfo) {
    if (this.cardTextureCache.has(cardCode)) {
      return this.cardTextureCache.get(cardCode);
    }

    const canvas = document.createElement('canvas');
    canvas.width = 512;
    canvas.height = 744;
    const ctx = canvas.getContext('2d');

    let frameColor = '#c28544';
    if (cardInfo) {
      if (cardInfo.type & 0x2) frameColor = '#1d9e74';
      else if (cardInfo.type & 0x4) frameColor = '#bc1c6c';
      else if (cardInfo.type & 0x20) frameColor = '#a05c28';
      else if (cardInfo.type & 0x40) frameColor = '#732c86';
      else if (cardInfo.type & 0x2000) frameColor = '#e5e7eb';
      else if (cardInfo.type & 0x800000) frameColor = '#1e1e1e';
      else if (cardInfo.type & 0x4000000) frameColor = '#0284c7';
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

    const texture = new THREE.CanvasTexture(canvas);
    this.cardTextureCache.set(cardCode, texture);
    // Real card art (if available) is layered over the procedural drawing
    // asynchronously — the card is playable immediately either way.
    if (this.cardImageProvider) {
      this.cardImageProvider(cardCode).then((url) => {
        if (!url) return;
        const img = new Image();
        img.onload = () => {
          const tex = this.cardTextureCache.get(cardCode);
          if (!tex || tex.image !== canvas) return;
          const ctx = canvas.getContext('2d');
          const ax = 40, ay = 80, aw = canvas.width - 80, ah = 360;
          const scale = Math.min(aw / img.width, ah / img.height);
          const dw = img.width * scale, dh = img.height * scale;
          ctx.drawImage(img, ax + (aw - dw) / 2, ay + (ah - dh) / 2, dw, dh);
          tex.needsUpdate = true;
        };
        img.src = url;
      }).catch(() => {});
    }
    return texture;
  }

  animateDrawCard(player, cardCode, cardInfo = null, onComplete = null) {
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

  animateSummon(player, cardCode, slotIndex, position = 0x1, cardInfo = null, isSpecial = false) {
    if (isSpecial) soundManager.playSpecialSummon();
    else soundManager.playSummon();

    const cardMesh = this.createCardMesh(cardCode, cardInfo);
    const targetCoord = ZONE_COORDS[player].mzone[slotIndex];
    if (!targetCoord) return;

    cardMesh.position.set(targetCoord.x, 5.0, targetCoord.z + (player === 0 ? 2 : -2));
    cardMesh.scale.set(0.2, 0.2, 0.2);
    this.scene.add(cardMesh);

    // ocgcore position bits: POS_DEFENSE = 0xc, POS_FACEDOWN = 0xa.
    const isDef = (position & 0xc) !== 0;
    const isFaceDown = (position & 0xa) !== 0;

    // A card on the board is a thin box lying flat (thickness along y). Turning
    // it sideways for defense means rotating about y (the face normal), NOT z:
    // a z-rotation tips the card up onto its edge, perpendicular to the board.
    const targetRot = {
      x: isFaceDown ? Math.PI : 0,
      y: (player === 1 ? Math.PI : 0) + (isDef ? Math.PI / 2 : 0),
      z: 0
    };

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
    this.cardsOnField[player].mzone[slotIndex] = cardMesh;
    return cardMesh;
  }

  animateSetCard(player, cardCode, slotIndex, isMonster = false, cardInfo = null) {
    soundManager.playDraw();
    const cardMesh = this.createCardMesh(cardCode, cardInfo);
    const targetCoord = isMonster
      ? ZONE_COORDS[player].mzone[slotIndex]
      : ZONE_COORDS[player].szone[slotIndex];
    if (!targetCoord) return;

    cardMesh.position.set(targetCoord.x, 3.5, targetCoord.z);
    // Face-down monster sets are set in defense (horizontal): rotate about y,
    // not z, so the card stays flat on the board.
    cardMesh.rotation.set(
      Math.PI,
      (player === 1 ? Math.PI : 0) + (isMonster ? Math.PI / 2 : 0),
      0
    );
    this.scene.add(cardMesh);

    this.makeTween(cardMesh.position)
      .to({ x: targetCoord.x, y: targetCoord.y + 0.05, z: targetCoord.z }, 350)
      .easing(TWEEN.Easing.Quadratic.Out)
      .start();

    const locName = isMonster ? 'mzone' : 'szone';
    cardMesh.userData.slot = { player, loc: locName, seq: slotIndex };
    this.cardsOnField[player][locName][slotIndex] = cardMesh;
    return cardMesh;
  }

  animateReposition(player, slotIndex, newPos) {
    soundManager.playDraw();
    const cardMesh = this.cardsOnField[player].mzone[slotIndex];
    if (!cardMesh) return;

    // ocgcore position bits: POS_DEFENSE = 0xc, POS_FACEDOWN = 0xa.
    const isDef = (newPos & 0xc) !== 0;
    const isFaceDown = (newPos & 0xa) !== 0;

    this.makeTween(cardMesh.position)
      .to({ y: 0.8 }, 150)
      .chain(
        this.makeTween(cardMesh.position)
          .to({ y: 0.05 }, 150)
      )
      .start();

    this.makeTween(cardMesh.rotation)
      .to({
        x: isFaceDown ? Math.PI : 0,
        y: (player === 1 ? Math.PI : 0) + (isDef ? Math.PI / 2 : 0),
        z: 0
      }, 300)
      .easing(TWEEN.Easing.Cubic.InOut)
      .start();
  }

  animateAttack(attackerPlayer, attackerSlot, targetPlayer, targetSlot, isDirect = false) {
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

  animateDestroy(player, loc, slotIndex) {
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
  }

  // Removes a card mesh from its registered slot WITHOUT removing it from the
  // scene; returns the mesh (or null) so callers can move it elsewhere.
  removeFromSlot(player, locName, seq) {
    const zone = this.cardsOnField[player] && this.cardsOnField[player][locName];
    if (!zone || !zone[seq]) return null;
    const mesh = zone[seq];
    zone[seq] = null;
    return mesh;
  }

  // Instant (non-animated) placement used by the replay applier when it
  // rebuilds the board for a seek/step-back. Supports every zone a card can
  // rest in: mzone/szone in `position`, grave/banish stacked, and piles.
  placeCard(player, locName, seq, cardCode, cardInfo, position = 0x1) {
    if (locName === 'mzone' || locName === 'szone') {
      const coord = this.getZonePosition(player, locName, seq);
      if (!coord) return null;
      const mesh = this.createCardMesh(cardCode, cardInfo);
      mesh.position.set(coord.x, coord.y + 0.05, coord.z);
      mesh.rotation.set(0, 0, 0);
      const rot = this.cardRotationFor(player, position);
      mesh.rotation.set(rot.x, rot.y, rot.z);
      mesh.userData.slot = { player, loc: locName, seq };
      this.scene.add(mesh);
      this.cardsOnField[player][locName][seq] = mesh;
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
      return mesh;
    }
    return null; // deck/extra/hand/overlay have no resting mesh on the board
  }

  // Moves an existing mesh to a destination zone. `instant` snaps instead of
  // tweening (replay seek). Deck/extra/overlay/hand destinations take the
  // mesh off the board — those zones are represented by stacks (deck/extra)
  // or the 2D hand dock, not individual meshes.
  moveCard(mesh, toPlayer, locName, seq, position = 0x1, { instant = false } = {}) {
    if (!mesh) return;

    if (locName === 'mzone' || locName === 'szone') {
      const coord = this.getZonePosition(toPlayer, locName, seq);
      if (!coord) return;
      mesh.userData.slot = { player: toPlayer, loc: locName, seq };
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
  retireMesh(mesh, instant = false) {
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
  }

  // Clears every card, chain badge, and in-flight tween off the board; the
  // replay applier calls this before rebuilding state for a seek.
  clearBoard() {
    this.hoveredCard = null;
    this.cardMeshes.slice().forEach(mesh => {
      this.scene.remove(mesh);
    });
    this.cardMeshes = [];
    this.cardsOnField = {
      0: { mzone: [], szone: [], grave: [], banish: [], deck: [], extra: [] },
      1: { mzone: [], szone: [], grave: [], banish: [], deck: [], extra: [] }
    };
    if (this.chainVisualizer) this.chainVisualizer.clearChain();
    this.tweenGroup.removeAll();
  }

  createShockwave(x, z, colorHex = 0x00d2ff) {
    const ringGeo = new THREE.RingGeometry(0.1, 0.4, 32);
    const ringMat = new THREE.MeshBasicMaterial({
      color: colorHex,
      transparent: true,
      opacity: 0.9,
      side: THREE.DoubleSide
    });
    const ringMesh = new THREE.Mesh(ringGeo, ringMat);
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

  createHitSparks(x, z) {
    const sparkCount = 20;
    const pGeo = new THREE.BufferGeometry();
    const positions = new Float32Array(sparkCount * 3);
    const velocities = [];

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

    pGeo.setAttribute('position', new THREE.BufferAttribute(positions, 3));
    const pMat = new THREE.PointsMaterial({
      color: 0xf59e0b,
      size: 0.25,
      transparent: true,
      opacity: 1
    });

    const pMesh = new THREE.Points(pGeo, pMat);
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

  cameraShake(intensity = 0.25, duration = 200) {
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

  initEvents() {
    window.addEventListener('resize', () => this.onWindowResize());

    this.container.addEventListener('mousemove', (e) => {
      const rect = this.container.getBoundingClientRect();
      this.mouse.x = ((e.clientX - rect.left) / rect.width) * 2 - 1;
      this.mouse.y = -((e.clientY - rect.top) / rect.height) * 2 + 1;
      this.handleHover();
    });

    this.container.addEventListener('click', (e) => {
      if (this.hoveredCard && this.onCardClick) {
        // Screen coordinates let the caller position an action popup (e.g.
        // the battle-phase attack menu) right where the player clicked.
        this.onCardClick(e.clientX, e.clientY, this.hoveredCard.userData);
      }
    });
  }

  handleHover() {
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
      }
    } else if (this.hoveredCard) {
      this.makeTween(this.hoveredCard.position)
        .to({ y: this.hoveredCard.userData.originalY || 0.05 }, 120)
        .start();
      this.hoveredCard = null;
    }
  }

  onWindowResize() {
    const width = this.container.clientWidth || window.innerWidth;
    const height = this.container.clientHeight || window.innerHeight;
    this.camera.aspect = width / height;
    this.camera.updateProjectionMatrix();
    this.renderer.setSize(width, height);
  }

  animate() {
    if (this.disposed) return;
    requestAnimationFrame(() => this.animate());
    this.tweenGroup.update();
    this.renderer.render(this.scene, this.camera);
  }

  // dispose tears down the WebGL context and listeners when the component
  // hosting this field unmounts, so re-entering the duel screen does not leak
  // renderers or stack animation loops.
  dispose() {
    this.disposed = true;
    window.removeEventListener('resize', this.onWindowResize);
    if (this.renderer) {
      this.renderer.dispose();
      if (this.renderer.domElement && this.renderer.domElement.parentNode) {
        this.renderer.domElement.parentNode.removeChild(this.renderer.domElement);
      }
    }
  }
}
