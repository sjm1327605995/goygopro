// Real scene fixture: checks canvas fitting, pointer access and resize independently
// of the classic pixel-parity fixture. Card art is local so QA works offline.
import React from 'react';
import {createRoot} from 'react-dom/client';
import '@mantine/core/styles.css';
import '../css/style.css';
import '../css/gframe-window.css';
import '../css/duel-original.css';
import '../css/duel-neos.css';
import '../css/ui-kit.css';
import UiRoot from '../src/ui/UiRoot.tsx';
import DuelStage from '../src/components/DuelStage.tsx';
import {eventBus, WailsBridge} from '../src/wails_bridge.ts';
import {duelStore} from '../src/duel/store.ts';
import * as THREE from '../libs/three.module.js';
import blueEyes from './assets/blue-eyes.jpg';
import magician from './assets/dark-magician.jpg';

WailsBridge.getCardImage = async code => ({url: code === 46986414 ? magician : blueEyes, full: true});
const smoke = window.__arenaSmoke = {checks: {}, ready: false};
const wait = ms => new Promise(resolve => setTimeout(resolve, ms));
const record = (name, pass) => { smoke.checks[name] = !!pass; };
let stage;
const root = createRoot(document.getElementById('root'));
root.render(<UiRoot><div style={{width:'100vw',height:'100vh'}}><DuelStage layout="arena" onReady={handle => { stage = handle; }} /></div></UiRoot>);

async function run() {
  while (!stage) await wait(25);
  const field = stage.field3D;
  window.__arena = {stage, eventBus, duelStore};
  eventBus.emit('duel:start', {playerType:0, duelRule:5, lp0:8000, lp1:6200, deck0:35,extra0:15,deck1:35,extra1:15});
  eventBus.emit('stoc:player_enter', {pos:0,name:'决斗者 / DUELIST'});
  eventBus.emit('stoc:player_enter', {pos:1,name:'挑战者 / CHALLENGER'});
  eventBus.emit('duel:draw', {player:0,count:5,cards:[89631139,46986414,89631139,46986414,89631139]});
  eventBus.emit('duel:draw', {player:1,count:5,cards:[0,0,0,0,0]});
  for (const [player,seq,code,position] of [[0,1,89631139,1],[0,3,46986414,1],[1,2,46986414,1],[1,0,89631139,4]]) {
    eventBus.emit('duel:move',{code,pc:player,pl:0x01,ps:0,pp:0,cc:player,cl:0x04,cs:seq,cp:position,reason:0});
  }
  eventBus.emit('duel:new_turn',{player:0});
  eventBus.emit('duel:new_phase',{phase:4});
  eventBus.emit('duel:select_idlecmd',{player:0,toBP:true,toEP:true,shuffle:true,summon:[],spsummon:[],reposition:[],mset:[],sset:[],activate:[]});
  duelStore.inspect(89631139);
  await wait(1800);
  const rect = field.container.getBoundingClientRect();
  const project = position => {
    const p = new THREE.Vector3(position.x,position.y,position.z).project(field.camera);
    return {x:rect.left+(p.x+1)*rect.width/2,y:rect.top+(1-p.y)*rect.height/2,ndc:p};
  };
  field.scene.updateMatrixWorld(true);
  field.camera.updateMatrixWorld(true);
  const zones = [0,1].flatMap(player => ['deck','extra','grave','banish'].map(loc => field.getZonePosition(player,loc)));
  const pilesFit = () => zones.every(pos => { const {ndc}=project(pos); return Math.abs(ndc.x)<.96 && Math.abs(ndc.y)<.96; });
  record('all-piles-inside-canvas', pilesFit());
  record('field-clear-of-inspector', rect.left >= document.querySelector('.card-preview-panel').getBoundingClientRect().right);
  record('phase-controls-below-canvas', document.querySelector('#phase-strip').getBoundingClientRect().top >= rect.bottom);
  record('hand-visible', field.handMeshes[0].filter(Boolean).length === 5);
  let picked = null;
  const original = field.onPlaceZoneClick;
  const target = {player:0,loc:4,seq:4};
  field.setPlaceSelectZones([target],new Set());
  field.onPlaceZoneClick = zone => { picked = zone; };
  const point = project(field.getZonePosition(0,'mzone',4));
  const hit = document.elementFromPoint(point.x,point.y);
  record('board-pointer-not-blocked', hit === field.renderer.domElement);
  hit?.dispatchEvent(new MouseEvent('click',{bubbles:true,clientX:point.x,clientY:point.y}));
  record('zone-click-matches-projection', picked?.seq===4 && picked?.player===0);
  field.onPlaceZoneClick = original;
  field.clearPlaceSelectZones();
  const before = field.renderer.domElement.width;
  field.container.style.right = '10%';
  await wait(150);
  record('container-resize-updates-renderer', field.renderer.domElement.width < before);
  field.container.style.right = '';
  await wait(150);
  const expectedSide = field.camera.position.z;
  duelStore.toggleViewSwap();
  await wait(600);
  record('swapped-camera-fits', Math.sign(field.camera.position.z) !== Math.sign(expectedSide) && pilesFit());
  duelStore.toggleViewSwap();
  await wait(600);
  record('swap-restores-bottom-seat', Math.sign(field.camera.position.z)===Math.sign(expectedSide));
  const hand = field.handMeshes[0][2];
  const handPoint = project(hand.position);
  field.container.dispatchEvent(new MouseEvent('mousemove',{bubbles:true,clientX:handPoint.x,clientY:handPoint.y}));
  record('hand-hover-matches-projection', field.hoveredCard === hand);
  field.container.dispatchEvent(new MouseEvent('mousemove',{bubbles:true,clientX:rect.left+1,clientY:rect.top+1}));
  record('no-fatal',true);
}
run().catch(error => { smoke.checks.fatalMsg = String(error.stack || error); }).finally(() => { smoke.ready = true; });
