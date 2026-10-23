import './arena_smoke.jsx';
import * as THREE from '../libs/three.module.js';
import {WailsBridge,eventBus} from '../src/wails_bridge.ts';
const smoke=window.__handsSmoke={checks:{},ready:false};
const record=(name,ok)=>{smoke.checks[name]=!!ok;};
const pause=ms=>new Promise(resolve=>setTimeout(resolve,ms));
const wait=async fn=>{for(let i=0;i<400;i++){if(fn())return;await pause(20);}throw Error('hands wait timed out');};

async function run(){
 await wait(()=>window.__arenaSmoke?.ready);
 const field=window.__arena.stage.field3D;
 const originalClick=field.onCardClick;
 const originalInspect=field.onCardInspect;
 // This fixture varies the renderer's hand sizes without changing game state.
 // Inspection would legitimately resync the manager from its original snapshot.
 field.onCardInspect=()=>{};
 let clicked;
 field.onCardClick=(x,y,data)=>{clicked=data.slot;};
 const originalHands=field.handMeshes.map(hand=>hand.map(mesh=>mesh.userData.cardCode));
 const bounds=()=>field.container.getBoundingClientRect();
 const point=position=>{const p=position.clone().project(field.camera),r=bounds();return{x:r.left+(p.x+1)*r.width/2,y:r.top+(1-p.y)*r.height/2};};
 const findVisiblePoint=mesh=>{
  const r=bounds();field.scene.updateMatrixWorld(true);
  const corners=[[-.7,-1],[.7,-1],[.7,1],[-.7,1]].map(([x,z])=>point(new THREE.Vector3(x,0,z).applyMatrix4(mesh.matrixWorld)));
  const minX=Math.min(...corners.map(p=>p.x)),maxX=Math.max(...corners.map(p=>p.x));
  const minY=Math.min(...corners.map(p=>p.y)),maxY=Math.max(...corners.map(p=>p.y));
  for(let row=1;row<9;row++)for(let col=1;col<17;col++){
    const x=minX+(maxX-minX)*col/17,y=minY+(maxY-minY)*row/9;
    field.raycaster.setFromCamera(new THREE.Vector2((x-r.left)/r.width*2-1,1-(y-r.top)/r.height*2),field.camera);
    if(field.raycaster.intersectObjects(field.cardMeshes)[0]?.object===mesh)return{x,y};
  }
  return null;
 };
 const visibleFace=mesh=>{
  field.scene.updateMatrixWorld(true);const p=mesh.position.clone().project(field.camera);
  field.raycaster.setFromCamera(new THREE.Vector2(p.x,p.y),field.camera);
  return field.raycaster.intersectObject(mesh)[0]?.face.materialIndex;
 };
 try{
  for(const count of [1,5,12]){
   const codes=Array.from({length:count},(_,i)=>i%2?46986414:89631139);
   field.setHands([codes,Array(count).fill(0)]);await pause(80);
   const r=bounds(),near=field.handMeshes[0],far=field.handMeshes[1];
   record(`hand-${count}-anchors`,near.every(mesh=>point(mesh.position).y>r.top+r.height*.67)&&far.every(mesh=>point(mesh.position).y<r.top+r.height*.25));
   let accessible=true;
   for(const mesh of near){
    const p=findVisiblePoint(mesh);if(!p){accessible=false;break;}
    field.container.dispatchEvent(new MouseEvent('click',{bubbles:true,clientX:p.x,clientY:p.y}));
    if(clicked?.seq!==mesh.userData.slot.seq||clicked?.player!==0)accessible=false;
    field.container.dispatchEvent(new MouseEvent('mousemove',{bubbles:true,clientX:r.left+1,clientY:r.top+r.height/2}));await pause(130);
   }
   record(`hand-${count}-every-card-pickable`,accessible);
   record(`hand-${count}-opponent-stays-hidden`,far.every(mesh=>mesh.userData.cardCode===0));
   record(`hand-${count}-spell-trap-row-clear`,[0,1,2,3,4].every(seq=>{
    const p=field.getZonePosition(0,'szone',seq),ndc=new THREE.Vector3(p.x,p.y,p.z).project(field.camera);
    field.raycaster.setFromCamera(new THREE.Vector2(ndc.x,ndc.y),field.camera);
    return field.raycaster.intersectObjects(near).length===0;
   }));
  }
  const selected=field.handMeshes[0][6],home=point(selected.position),initialScale=selected.scale.x;
  record('visible-hand-faces-match-information',visibleFace(selected)===2&&visibleFace(field.handMeshes[1][6])===3);
  const p=findVisiblePoint(selected);
  field.container.dispatchEvent(new MouseEvent('mousemove',{bubbles:true,clientX:p.x,clientY:p.y}));await pause(150);
  record('hover-lifts-and-enlarges-card',point(selected.position).y<home.y&&selected.scale.x>initialScale*1.1);
  field.container.dispatchEvent(new MouseEvent('mousemove',{bubbles:true,clientX:p.x,clientY:p.y}));await pause(60);
  record('hover-remains-stable-at-original-pointer',field.hoveredCard===selected);
  const r=bounds();field.container.dispatchEvent(new MouseEvent('mousemove',{bubbles:true,clientX:r.left+1,clientY:r.top+r.height/2}));await pause(150);
  field.setViewSwapped(true);await pause(600);
  record('swap-moves-other-hand-to-bottom',point(field.handMeshes[1][6].position).y>r.top+r.height*.67&&point(field.handMeshes[0][6].position).y<r.top+r.height*.25);
  record('swap-keeps-hidden-information',field.handMeshes[1].every(mesh=>mesh.userData.cardCode===0));
  record('swap-still-renders-card-backs',visibleFace(field.handMeshes[1][6])===3);
  field.setViewSwapped(false);await pause(600);
  const before=point(field.handMeshes[0][6].position);
  field.shuffleHand(0);await pause(400);
  record('shuffle-restores-hand-anchor',Math.abs(point(field.handMeshes[0][6].position).x-before.x)<2);
  field.onCardClick=originalClick;field.onCardInspect=originalInspect;field.setHands(originalHands);
  const originalRespond=WailsBridge.respondIdleCmd,calls=[];
  WailsBridge.respondIdleCmd=(...args)=>{calls.push(args);};
  try{
   eventBus.emit('duel:select_idlecmd',{player:0,toBP:true,toEP:true,summon:[
    {code:89631139,c:0,l:2,s:0,idx:3},{code:89631139,c:0,l:2,s:2,idx:7},
   ],spsummon:[],repos:[],mset:[],sset:[],activate:[]});
   await pause(100);
   const target=findVisiblePoint(field.handMeshes[0][2]);
   field.container.dispatchEvent(new MouseEvent('click',{bubbles:true,clientX:target.x,clientY:target.y}));
   await wait(()=>document.querySelector('#action-popup .action-btn')?.textContent==='召唤');
   record('hand-click-opens-engine-actions',true);
   const popup=document.getElementById('action-popup').getBoundingClientRect();
   record('action-popup-follows-visible-hand',popup.left>bounds().left&&popup.top>window.innerHeight*.45);
   document.querySelector('#action-popup .action-btn').click();await pause(30);
   record('duplicate-card-sends-exact-sequence-command',calls.length===1&&calls[0][0]===7&&calls[0][1]===0);
  }finally{WailsBridge.respondIdleCmd=originalRespond;}
  record('no-fatal',true);
 }finally{field.onCardClick=originalClick;field.onCardInspect=originalInspect;field.setViewSwapped(false);field.setHands(originalHands);}
}
run().catch(error=>{smoke.checks.fatalMsg=String(error.stack||error);}).finally(()=>{smoke.ready=true;});
