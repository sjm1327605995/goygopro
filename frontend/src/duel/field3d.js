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
  // Player 0 Zones
  0: {
    mzone: [
      { x: -3.8, y: 0.02, z: 1.6 },
      { x: -1.9, y: 0.02, z: 1.6 },
      { x: 0.0,  y: 0.02, z: 1.6 },
      { x: 1.9,  y: 0.02, z: 1.6 },
      { x: 3.8,  y: 0.02, z: 1.6 }
    ],
    szone: [
      { x: -3.8, y: 0.02, z: 4.2 },
      { x: -1.9, y: 0.02, z: 4.2 },
      { x: 0.0,  y: 0.02, z: 4.2 },
      { x: 1.9,  y: 0.02, z: 4.2 },
      { x: 3.8,  y: 0.02, z: 4.2 }
    ],
    field:     { x: -5.8, y: 0.02, z: 1.6 },
    grave:     { x: 5.8,  y: 0.02, z: 1.6 },
    banish:    { x: 5.8,  y: 0.02, z: 2.9 },
    deck:      { x: 5.8,  y: 0.02, z: 4.2 },
    extra:     { x: -5.8, y: 0.02, z: 4.2 },
    emz_left:  { x: -1.9, y: 0.02, z: 0.0 },
    emz_right: { x: 1.9,  y: 0.02, z: 0.0 }
  },
  // Opponent 1 Zones (Rotated 180 deg)
  1: {
    mzone: [
      { x: 3.8,  y: 0.02, z: -1.6 },
      { x: 1.9,  y: 0.02, z: -1.6 },
      { x: 0.0,  y: 0.02, z: -1.6 },
      { x: -1.9, y: 0.02, z: -1.6 },
      { x: -3.8, y: 0.02, z: -1.6 }
    ],
    szone: [
      { x: 3.8,  y: 0.02, z: -4.2 },
      { x: 1.9,  y: 0.02, z: -4.2 },
      { x: 0.0,  y: 0.02, z: -4.2 },
      { x: -1.9, y: 0.02, z: -4.2 },
      { x: -3.8, y: 0.02, z: -4.2 }
    ],
    field:     { x: 5.8,  y: 0.02, z: -1.6 },
    grave:     { x: -5.8, y: 0.02, z: -1.6 },
    banish:    { x: -5.8, y: 0.02, z: -2.9 },
    deck:      { x: -5.8, y: 0.02, z: -4.2 },
    extra:     { x: 5.8,  y: 0.02, z: -4.2 }
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
    this.chainVisualizer = null;

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

    new TWEEN.Tween(cardMesh.position)
      .to({ x: targetPos.x, y: 4.5, z: (startCoord.z + targetPos.z) / 2 }, 350)
      .easing(TWEEN.Easing.Quadratic.Out)
      .chain(
        new TWEEN.Tween(cardMesh.position)
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

    new TWEEN.Tween(cardMesh.rotation)
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

    const isDef = (position & 0xa) !== 0;
    const isFaceDown = (position & 0x8) !== 0;

    const targetRot = {
      x: isFaceDown ? Math.PI : 0,
      y: player === 1 ? Math.PI : 0,
      z: isDef ? Math.PI / 2 : 0
    };

    new TWEEN.Tween(cardMesh.scale)
      .to({ x: 1, y: 1, z: 1 }, 450)
      .easing(TWEEN.Easing.Back.Out)
      .start();

    new TWEEN.Tween(cardMesh.position)
      .to({ x: targetCoord.x, y: targetCoord.y + 0.05, z: targetCoord.z }, 450)
      .easing(TWEEN.Easing.Bounce.Out)
      .onComplete(() => {
        this.createShockwave(targetCoord.x, targetCoord.z, isSpecial ? 0xf59e0b : 0x00d2ff);
      })
      .start();

    new TWEEN.Tween(cardMesh.rotation)
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
    cardMesh.rotation.set(Math.PI, 0, isMonster ? Math.PI / 2 : 0);
    this.scene.add(cardMesh);

    new TWEEN.Tween(cardMesh.position)
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

    const isDef = (newPos & 0xa) !== 0;
    const isFaceDown = (newPos & 0x8) !== 0;

    new TWEEN.Tween(cardMesh.position)
      .to({ y: 0.8 }, 150)
      .chain(
        new TWEEN.Tween(cardMesh.position)
          .to({ y: 0.05 }, 150)
      )
      .start();

    new TWEEN.Tween(cardMesh.rotation)
      .to({
        x: isFaceDown ? Math.PI : 0,
        z: isDef ? Math.PI / 2 : 0
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

    new TWEEN.Tween(attackerMesh.position)
      .to({ y: 1.6 }, 200)
      .easing(TWEEN.Easing.Quadratic.Out)
      .chain(
        new TWEEN.Tween(attackerMesh.position)
          .to({ x: targetPos.x, y: targetPos.y + 0.3, z: targetPos.z }, 180)
          .easing(TWEEN.Easing.Exponential.In)
          .onComplete(() => {
            soundManager.playDamage();
            this.createHitSparks(targetPos.x, targetPos.z);
            this.cameraShake(0.2, 200);
          })
          .chain(
            new TWEEN.Tween(attackerMesh.position)
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

    new TWEEN.Tween(cardMesh.position)
      .to({ y: 2.0 }, 150)
      .chain(
        new TWEEN.Tween(cardMesh.position)
          .to({ x: graveCoord.x, y: 0.2, z: graveCoord.z }, 350)
          .easing(TWEEN.Easing.Cubic.In)
          .onComplete(() => {
            this.scene.remove(cardMesh);
            const idx = this.cardMeshes.indexOf(cardMesh);
            if (idx > -1) this.cardMeshes.splice(idx, 1);
          })
      )
      .start();

    new TWEEN.Tween(cardMesh.rotation)
      .to({ x: 0, y: 0, z: Math.PI }, 350)
      .start();
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

    new TWEEN.Tween(ringMesh.scale)
      .to({ x: 6, y: 6, z: 6 }, 400)
      .easing(TWEEN.Easing.Quadratic.Out)
      .start();

    new TWEEN.Tween(ringMat)
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
    new TWEEN.Tween(progress)
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

    this.container.addEventListener('click', () => {
      if (this.hoveredCard && this.onCardClick) {
        this.onCardClick(this.hoveredCard, this.hoveredCard.userData);
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
          new TWEEN.Tween(this.hoveredCard.position)
            .to({ y: this.hoveredCard.userData.originalY || 0.05 }, 120)
            .start();
        }
        this.hoveredCard = mesh;
        new TWEEN.Tween(mesh.position)
          .to({ y: (mesh.userData.originalY || 0.05) + 0.35 }, 120)
          .start();

        if (this.onCardInspect && mesh.userData.cardCode) {
          this.onCardInspect(mesh.userData.cardCode, mesh.userData.cardInfo);
        }
      }
    } else if (this.hoveredCard) {
      new TWEEN.Tween(this.hoveredCard.position)
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
    requestAnimationFrame(() => this.animate());
    TWEEN.update();
    this.renderer.render(this.scene, this.camera);
  }
}
