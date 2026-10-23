import * as THREE from '../../libs/three.module.js';

// Original, deterministic stonework built in Three.js. Card/zone coordinates
// remain owned by field3d_geometry; these meshes are visual scenery only.
export function createStoneSurface(): any {
  const canvas = document.createElement('canvas'); canvas.width = 1024; canvas.height = 820;
  const ctx = canvas.getContext('2d')!;
  let seed = 73;
  const random = () => { seed = (Math.imul(seed,1664525)+1013904223)|0; return (seed>>>0)/4294967296; };
  ctx.fillStyle = '#202d2d'; ctx.fillRect(0,0,1024,820);
  for (let row=0;row<14;row++) for (let col=-1;col<18;col++) {
    const x=col*64+(row%2)*32, y=row*64;
    const tone=Math.floor(random()*12);
    ctx.fillStyle=y<410?`rgb(${58+tone},${66+tone},${59+tone})`:`rgb(${47+tone},${64+tone},${65+tone})`;
    ctx.fillRect(x+2,y+2,60,60);
    ctx.strokeStyle='#c1c6a51b';ctx.strokeRect(x+3,y+3,58,58);
    if(random()>.58){
      ctx.strokeStyle='#0d1c2488';ctx.lineWidth=.8;ctx.beginPath();ctx.moveTo(x+8,y+5);
      ctx.lineTo(x+18+random()*16,y+24);ctx.lineTo(x+13+random()*30,y+45);ctx.lineTo(x+40,y+61);ctx.stroke();
    }
  }
  for(let i=0;i<42000;i++) {
    const light=random()>.5;
    ctx.fillStyle=light?'#e2dec30a':'#020c1717';
    ctx.fillRect(random()*1024,random()*820,random()*3+.5,random()*2+.5);
  }
  // Weathered moss grows towards the rim, leaving the playable surface clear.
  for(let i=0;i<450;i++) {
    const x=random()*1024,y=random()*820;
    if(x>100&&x<924&&y>75&&y<745)continue;
    ctx.fillStyle=`rgba(74,94,54,${.08+random()*.16})`;
    ctx.beginPath();ctx.ellipse(x,y,random()*21+3,random()*12+2,random()*3,0,Math.PI*2);ctx.fill();
  }
  ctx.strokeStyle='#b5a47165';ctx.lineWidth=2;
  ctx.strokeRect(18,18,988,784);ctx.strokeRect(28,28,968,764);
  ctx.beginPath();ctx.moveTo(36,410);ctx.lineTo(988,410);ctx.stroke();
  ctx.save();ctx.translate(512,410);ctx.rotate(Math.PI/4);
  ctx.strokeStyle='#bca77850';ctx.strokeRect(-69,-69,138,138);ctx.strokeRect(-57,-57,114,114);ctx.restore();
  const vignette=ctx.createRadialGradient(512,410,130,512,410,640);
  vignette.addColorStop(0,'#00000000');vignette.addColorStop(1,'#020b16a0');ctx.fillStyle=vignette;ctx.fillRect(0,0,1024,820);
  const texture=new (THREE as any).CanvasTexture(canvas);texture.colorSpace=THREE.SRGBColorSpace;
  return texture;
}

export function bevelledSlab(width:number,height:number,depth:number,material:any): any {
  const c=Math.min(width,height)*.14;
  const shape=new (THREE as any).Shape();
  const points=[[-width/2+c,-height/2],[width/2-c,-height/2],[width/2,-height/2+c],[width/2,height/2-c],[width/2-c,height/2],[-width/2+c,height/2],[-width/2,height/2-c],[-width/2,-height/2+c]];
  shape.moveTo(...points[0]);points.slice(1).forEach(p=>shape.lineTo(...p));shape.closePath();
  const mesh=new (THREE as any).Mesh(new (THREE as any).ExtrudeGeometry(shape,{depth,bevelEnabled:true,bevelThickness:.035,bevelSize:.035,bevelSegments:1,steps:1}),material);
  mesh.rotation.x=-Math.PI/2;
  return mesh;
}

export function createArenaArchitecture(zone:(seat:number,loc:string,seq?:number)=>any,modern:boolean):any {
  const group=new (THREE as any).Group();group.name='arena-stone-architecture';
  const grainCanvas=document.createElement('canvas');grainCanvas.width=256;grainCanvas.height=288;
  const grain=grainCanvas.getContext('2d')!;
  grain.fillStyle='#b7bcb7';grain.fillRect(0,0,256,288);
  let state=191;
  const random=()=>{state=(Math.imul(state,1664525)+1013904223)|0;return(state>>>0)/4294967296;};
  for(let i=0;i<9500;i++){grain.fillStyle=random()>.5?'#ffffff18':'#1b292b18';grain.fillRect(random()*256,random()*288,random()*3+1,random()*2+1);}
  grain.strokeStyle='#3b4b4b70';grain.lineWidth=1.4;
  for(let i=0;i<6;i++){const x=random()*256,y=random()*288;grain.beginPath();grain.moveTo(x,y);grain.lineTo(x+18,y+13);grain.lineTo(x+11,y+27);grain.lineTo(x+26,y+36);grain.stroke();}
  grain.strokeStyle='#758b8050';grain.lineWidth=2;grain.strokeRect(12,12,232,264);
  grain.beginPath();grain.moveTo(128,94);grain.lineTo(166,144);grain.lineTo(128,194);grain.lineTo(90,144);grain.closePath();grain.stroke();
  const grainTexture=new (THREE as any).CanvasTexture(grainCanvas);grainTexture.colorSpace=THREE.SRGBColorSpace;
  grainTexture.wrapS=grainTexture.wrapT=THREE.RepeatWrapping;
  const stone=new (THREE as any).MeshStandardMaterial({color:0x354443,roughness:.94,metalness:.05});
  const dark=new (THREE as any).MeshStandardMaterial({color:0x172329,roughness:.82,metalness:.12});
  const bronze=new (THREE as any).MeshStandardMaterial({color:0x8c7952,roughness:.52,metalness:.65});
  const base=bevelledSlab(22.2,18,.62,dark);base.position.y=-.94;group.add(base);
  const plinth=bevelledSlab(21.6,17.6,.15,bronze);plinth.position.y=-.35;group.add(plinth);
  // Coursed masonry around both halves, rather than a floating rectangular mat.
  for(const side of [-1,1])for(let index=0;index<12;index++) {
    const block=new (THREE as any).Mesh(new (THREE as any).BoxGeometry(1.65,.36,.55),stone);
    block.position.set((index-5.5)*1.74,-.12,side*8.55);group.add(block);
  }
  for(const side of [-1,1])for(let index=0;index<9;index++) {
    const block=new (THREE as any).Mesh(new (THREE as any).BoxGeometry(.48,.34,1.7),stone);
    block.position.set(side*10.55,-.12,(index-4)*1.82);group.add(block);
  }
  const own=new (THREE as any).MeshStandardMaterial({color:0x657a72,map:grainTexture,roughness:.96,metalness:.04});
  const opponent=new (THREE as any).MeshStandardMaterial({color:0x7a7c60,map:grainTexture,roughness:.96,metalness:.04});
  const seen=new Set<string>();
  for(const seat of [0,1]) for(const loc of ['mzone','szone'])for(let seq=0;seq<(loc==='mzone'&&modern?7:5);seq++){
    const p=zone(seat,loc,seq);if(!p)continue;
    const key=`${p.x}:${p.z}`;if(seen.has(key))continue;seen.add(key);
    const tile=bevelledSlab(1.94,2.16,.035,seat===0?own:opponent);
    tile.position.set(p.x,-.105,p.z);group.add(tile);
  }
  // Round grave/banish sockets and corner beacons echo the reference's side furniture.
  for(const seat of [0,1])for(const loc of ['grave','banish']){
    const p=zone(seat,loc);if(!p)continue;
    const well=new (THREE as any).Mesh(new (THREE as any).CylinderGeometry(.91,.96,.12,36),dark);
    well.position.set(p.x,-.06,p.z);group.add(well);
    const ring=new (THREE as any).Mesh(new (THREE as any).TorusGeometry(.79,.045,6,40),bronze);
    ring.rotation.x=-Math.PI/2;ring.position.set(p.x,.015,p.z);group.add(ring);
  }
  for(const x of [-10.45,10.45])for(const z of [-7.9,7.9]){
    const podium=bevelledSlab(.85,.85,.32,bronze);podium.position.set(x,-.08,z);group.add(podium);
    const glow=new (THREE as any).MeshStandardMaterial({color:z>0?0x7fc5ba:0xd2ba79,emissive:z>0?0x2d6d69:0x655129,emissiveIntensity:.7,roughness:.25,metalness:.2});
    const crystal=new (THREE as any).Mesh(new (THREE as any).OctahedronGeometry(.3),glow);
    crystal.scale.y=1.7;crystal.position.set(x,.64,z);group.add(crystal);
  }
  const groundCanvas=document.createElement('canvas');groundCanvas.width=512;groundCanvas.height=512;
  const groundContext=groundCanvas.getContext('2d')!;
  const fade=groundContext.createRadialGradient(256,256,70,256,256,255);
  fade.addColorStop(0,'#243831');fade.addColorStop(.63,'#1b302c');fade.addColorStop(1,'#14201f00');
  groundContext.fillStyle=fade;groundContext.fillRect(0,0,512,512);
  const groundMap=new (THREE as any).CanvasTexture(groundCanvas);groundMap.colorSpace=THREE.SRGBColorSpace;
  const ground=new (THREE as any).Mesh(new (THREE as any).PlaneGeometry(40,32),new (THREE as any).MeshBasicMaterial({map:groundMap,transparent:true,depthWrite:false}));
  ground.rotation.x=-Math.PI/2;ground.position.y=-1.2;ground.renderOrder=-1;group.add(ground);
  const moss=new (THREE as any).MeshStandardMaterial({color:0x344d36,roughness:1});
  for(const side of [-1,1])for(const z of [-5.1,4.9]){
    const column=new (THREE as any).Mesh(new (THREE as any).CylinderGeometry(.28,.42,1.35,8),stone);
    column.position.set(side*11.05,.15,z);group.add(column);
    const cap=new (THREE as any).Mesh(new (THREE as any).BoxGeometry(.9,.18,.9),bronze);
    cap.position.set(side*11.05,.9,z);group.add(cap);
    for(let i=0;i<3;i++){
      const shrub=new (THREE as any).Mesh(new (THREE as any).IcosahedronGeometry(.48,1),moss);
      shrub.scale.set(1.05,.5,.9);shrub.position.set(side*(10.9+random()*.3),-.05,z+(i-1)*.62);group.add(shrub);
    }
  }
  batchStaticScenery(group);
  return group;
}

// Identical static stones share an instanced draw. Hash the actual attributes,
// rather than shape bounding boxes, so distinct meshes cannot be combined.
function batchStaticScenery(group:any):void {
  const batches=new Map<string,any[]>();
  for(const object of group.children){
    if(!object.isMesh||Array.isArray(object.material))continue;
    let hash=2166136261;
    const arrays=Object.keys(object.geometry.attributes).sort().map(key=>object.geometry.attributes[key].array);
    if(object.geometry.index)arrays.push(object.geometry.index.array);
    for(const array of arrays){
      const bytes=new Uint8Array(array.buffer,array.byteOffset,array.byteLength);
      for(const byte of bytes)hash=Math.imul(hash^byte,16777619);
    }
    const key=`${object.material.uuid}:${object.geometry.type}:${object.geometry.attributes.position.count}:${hash>>>0}`;
    if(!batches.has(key))batches.set(key,[]);
    batches.get(key)!.push(object);
  }
  for(const meshes of batches.values()){
    if(meshes.length<2)continue;
    const first=meshes[0];
    const instances=new (THREE as any).InstancedMesh(first.geometry,first.material,meshes.length);
    meshes.forEach((mesh,index)=>{
      mesh.updateMatrix();instances.setMatrixAt(index,mesh.matrix);group.remove(mesh);
      if(mesh.geometry!==first.geometry)mesh.geometry.dispose();
    });
    instances.instanceMatrix.needsUpdate=true;instances.computeBoundingSphere();group.add(instances);
  }
}

export function createEnvironmentMask():any {
  const canvas=document.createElement('canvas');canvas.width=512;canvas.height=512;
  const ctx=canvas.getContext('2d')!;ctx.fillStyle='black';ctx.fillRect(0,0,512,512);
  const gradient=ctx.createRadialGradient(256,256,85,256,256,300);
  gradient.addColorStop(0,'white');gradient.addColorStop(.6,'#e0e0e0');gradient.addColorStop(1,'black');
  ctx.fillStyle=gradient;ctx.fillRect(0,0,512,512);
  const texture=new (THREE as any).CanvasTexture(canvas);
  return texture;
}

export function disposeArenaArchitecture(group:any):void {
  const geometries=new Set<any>(),materials=new Set<any>(),textures=new Set<any>();
  group.traverse((object:any)=>{if(object.isInstancedMesh)object.dispose();if(object.geometry)geometries.add(object.geometry);if(object.material){for(const m of Array.isArray(object.material)?object.material:[object.material])materials.add(m);}});
  geometries.forEach(g=>g.dispose());materials.forEach(m=>{if(m.map)textures.add(m.map);m.dispose();});textures.forEach(t=>t.dispose());
}
