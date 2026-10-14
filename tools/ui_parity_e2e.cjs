// Run against Vite (default :19500). PLAYWRIGHT_MODULE_PATH may point to a bundled runtime.
const pw = require(process.env.PLAYWRIGHT_MODULE_PATH || 'playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const out = process.env.PARITY_OUTPUT || 'tmp_dbg';
fs.mkdirSync(out,{recursive:true});
(async()=>{
 const browser=await pw.chromium.launch({headless:true,executablePath:process.env.CHROME_PATH||'C:/Program Files/Google/Chrome/Application/chrome.exe'});
 const page=await browser.newPage({viewport:{width:1024,height:640}});
 const errors=[]; page.on('pageerror',e=>errors.push(e.message)); page.on('dialog',d=>d.accept());
 const checks=[]; const check=(name,ok=true)=>{assert.ok(ok,name);checks.push(name);console.log('PASS '+name);};
 try {
  await page.goto(process.env.PARITY_URL||'http://127.0.0.1:19500/');
  await page.getByRole('button',{name:'编辑卡组',exact:true}).waitFor();
  await page.evaluate(async()=>{
   const {WailsBridge:b,eventBus}=await import('/src/wails_bridge.ts');
   const cards=[
    {code:1001,name:'Alpha',type:0x11,level:4,attack:1000,defense:1000},
    {code:1002,name:'Beta',type:0x21,level:2,attack:3000,defense:0},
    {code:1003,name:'Gamma',type:0x11,level:8,attack:2000,defense:2000},
    {code:1004,name:'Delta',type:2,level:0,attack:0,defense:0},
    {code:1005,name:'Side',type:4,level:0,attack:0,defense:0},
    {code:2001,name:'Fusion',type:0x41,level:6,attack:2200,defense:1000},
    {code:2002,name:'Synchro',type:0x2001,level:8,attack:2500,defense:1500},
   ].map(c=>({...c,alias:0,ot:3,race:1,attribute:1,desc:'fixture',strings:[]}));
   window.__parityCards=cards; window.__parityBridge=b;window.__parityBus=eventBus;window.__responses=[];window.__surrenders=0;
   b.listDecks=async()=>['Fixture'];b.loadDeck=async()=>({name:'Fixture',main:[1001,1002,1003,1004],extra:[2001,2002],side:[1005]});
   b.getCard=async code=>{const c=cards.find(x=>x.code===code)||cards[0];b._cardCache.set(code,c);return c;};
   b.getCardImage=async code=>({url:'data:image/svg+xml;charset=utf-8,'+encodeURIComponent(`<svg xmlns="http://www.w3.org/2000/svg" width="177" height="254"><rect width="177" height="254" fill="#bd8748"/><text x="10" y="40">${code}</text></svg>`)});
   b.searchCards=async f=>{window.__searchFilter=f;return [...cards].reverse();};b.listLFLists=async()=>[{hash:0,name:'N/A'}];b.lfListContent=async()=>({});
   b.surrender=()=>window.__surrenders++;
   b.respondIdleCmd=(idx,type)=>window.__responses.push([idx,type]);
  });
  await page.getByRole('button',{name:'编辑卡组',exact:true}).click();
  const main=page.locator('.ds-grid-main .deck-card-chip');
  await main.nth(3).waitFor();check('loaded-deck-selected',(await page.locator('#deck-select').innerText()).includes('Fixture'));
  await main.first().dblclick();await page.locator('#deck-zoom-overlay').waitFor();check('doubleclick-preserves-deck',await main.count()===4);
  await page.locator('#deck-big-card-close').click();
  await main.first().dragTo(main.nth(3));check('drag-inserts-at-target',JSON.stringify(await main.evaluateAll(es=>es.map(e=>e.title)))===JSON.stringify(['Beta','Gamma','Alpha','Delta']));
  await main.nth(2).click({button:'middle'});check('middle-copies',await main.count()===5);
  await main.nth(2).click({button:'right'});check('rightclick-removes',await main.count()===4);
  await page.getByRole('button',{name:'搜索',exact:true}).click();await page.locator('.deck-result-row').nth(6).waitFor();
  check('editor-requests-all-results',await page.evaluate(()=>window.__searchFilter.allResults===true));
  check('default-level-sort',(await page.locator('.deck-result-row').first().getAttribute('title'))==='Gamma');
  await page.locator('#deck-result-sort').click();await page.getByRole('option',{name:'攻击',exact:true}).click();
  check('attack-sort',(await page.locator('.deck-result-row').first().getAttribute('title'))==='Beta');
  const fusion=page.locator('.deck-result-row[title="Fusion"]');await fusion.dragTo(page.locator('.ds-grid-side'));
  check('search-extra-can-drop-into-side',await page.locator('.ds-grid-side .deck-card-chip[title="Fusion"]').count()===1);
  await page.locator('.ds-grid-extra .deck-card-chip').first().dragTo(page.locator('.search-results-grid'));
  check('drag-to-results-removes',await page.locator('.ds-grid-extra .deck-card-chip').count()===1);
  const extras=await page.locator('.ds-grid-extra .deck-card-chip').evaluateAll(es=>es.map(e=>e.title));
  const sides=await page.locator('.ds-grid-side .deck-card-chip').evaluateAll(es=>es.map(e=>e.title));
  await page.locator('#deck-shuffle-btn').click();
  check('shuffle-keeps-extra-side',JSON.stringify(await page.locator('.ds-grid-extra .deck-card-chip').evaluateAll(es=>es.map(e=>e.title)))===JSON.stringify(extras)&&JSON.stringify(await page.locator('.ds-grid-side .deck-card-chip').evaluateAll(es=>es.map(e=>e.title)))===JSON.stringify(sides));
  await page.screenshot({path:out+'/cpp-deck.png'});
  await page.getByRole('button',{name:'退出编辑',exact:true}).click();await page.getByRole('button',{name:'编辑卡组',exact:true}).waitFor();check('exit-click-not-obstructed');
  await page.evaluate(async()=>{const {duelStore}=await import('/src/duel/store.ts');window.__store=duelStore;const e=window.__parityBus;e.emit('duel:start',{playerType:0,duelRule:5,lp0:8000,lp1:8000});e.emit('duel:draw',{player:0,count:2,cards:[1001,1001]});e.emit('nav','duel');});
  await page.locator('#duel-screen').waitFor();
  // Access only for scene geometry; user interaction below uses real projected coordinates.
  const stage=await page.evaluate(async()=>{const {DuelField3D}=await import('/src/duel/field3d.ts');return !!DuelField3D;});check('duel-mounted',stage);
  await page.evaluate(()=>window.__parityBus.emit('duel:new_phase',{phase:0x4}));
  await page.locator('#phase-status').waitFor();check('one-current-phase',await page.locator('#phase-status').innerText()==='Ｍ１'&&await page.locator('.phase-fold').count()===0&&await page.locator('#phase-dp').count()===0);
  await page.evaluate(()=>window.__parityBus.emit('duel:select_idlecmd',{summon:[{code:1001,c:0,l:2,s:0,idx:0},{code:1001,c:0,l:2,s:1,idx:1}],toBP:true,toEP:true}));
  await page.locator('#phase-btn-bp').waitFor({state:'visible'});check('phase-buttons-original-positions',Math.round((await page.locator('#phase-btn-bp').boundingBox()).x)===640&&Math.round((await page.locator('#phase-btn-ep').boundingBox()).x)===800);
  const handClick=await page.evaluate(async()=>{
    const THREE=await import('/libs/three.module.js');
    const camera=new THREE.PerspectiveCamera();camera.position.set(.4,15.6,16);camera.lookAt(.4,0,0);
    camera.projectionMatrix.makePerspective(-1.8,.9,.84,-.84,2,200);camera.updateMatrixWorld(true);
    // ClientField LOCATION_HAND sequence 1 of two cards: qx=4.3,qy=4,qz=.501.
    const v=new THREE.Vector3(.6,1.002,8).project(camera);return {x:(v.x+1)*innerWidth/2,y:(1-v.y)*innerHeight/2};
  });
  await page.mouse.click(handClick.x,handClick.y);
  await page.getByRole('button',{name:'召唤',exact:true}).click();
  check('duplicate-hand-card-keeps-sequence',await page.evaluate(()=>window.__responses.at(-1)[0]===1));
  await page.evaluate(async()=>{const {CARD_QUESTION,PHINT_DESC_ADD}=await import('/src/domain/constants.ts');window.__parityBus.emit('duel:move',{code:1003,pc:0,pl:2,ps:0,pp:1,cc:0,cl:16,cs:0,cp:1});window.__parityBus.emit('duel:player_hint',{player:0,type:PHINT_DESC_ADD,data:CARD_QUESTION});});
  await page.keyboard.press('F1');check('grave-hotkey-respects-lock',await page.locator('#card-display-overlay').count()===0);
  await page.evaluate(()=>window.__parityBus.emit('ui:show_pile',{player:0,pile:'grave'}));check('all-grave-entrypoints-respect-lock',await page.locator('#card-display-overlay').count()===0);
  await page.evaluate(()=>{
    window.__fieldReplies=[];
    window.__parityBridge.respondSelectCard=indices=>window.__fieldReplies.push({kind:'tribute',indices});
    window.__parityBridge.respondCounter=counts=>window.__fieldReplies.push({kind:'counter',counts});
    window.__parityBridge.respondSelectSum=(count,indices)=>window.__fieldReplies.push({kind:'sum',count,indices});
    window.__parityBus.emit('duel:update_data',{player:0,location:4,cards:[null,null,{code:1001,position:{c:0,l:4,s:2,p:1},type:0x11,attack:1000,defense:1000}]});
  });
  const fieldClick=await page.evaluate(async()=>{
    const THREE=await import('/libs/three.module.js');
    const camera=new THREE.PerspectiveCamera();camera.position.set(.4,15.6,16);camera.lookAt(.4,0,0);camera.projectionMatrix.makePerspective(-1.8,.9,.84,-.84,2,200);camera.updateMatrixWorld(true);
    const v=new THREE.Vector3(-.1,.07,2.8).project(camera);return {x:(v.x+1)*innerWidth/2,y:(1-v.y)*innerHeight/2};
  });
  await page.evaluate(()=>window.__parityBus.emit('duel:select_card',{tribute:true,player:0,min:2,max:2,cancelable:true,cards:[{code:1001,c:0,l:4,s:2,p:2}]}));
  await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
  await page.mouse.click(fieldClick.x,fieldClick.y);
  await page.waitForFunction(()=>window.__fieldReplies.at(-1)?.kind==='tribute');
  check('native-field-double-tribute',await page.evaluate(()=>JSON.stringify(window.__fieldReplies.at(-1).indices)==='[0]'));
  await page.evaluate(()=>window.__parityBus.emit('duel:select_counter',{player:0,count:3,countertype:1,cards:[{code:1001,c:0,l:4,s:2,cnt:3}]}));
  await page.getByText('请取除3个[魔力指示物]',{exact:true}).waitFor();
  await page.mouse.click(fieldClick.x,fieldClick.y);await page.mouse.click(fieldClick.x,fieldClick.y);await page.mouse.click(fieldClick.x,fieldClick.y);
  await page.waitForFunction(()=>window.__fieldReplies.at(-1)?.kind==='counter');
  check('native-field-counter',await page.evaluate(()=>window.__fieldReplies.at(-1).counts[0]===3));
  await page.evaluate(()=>window.__parityBus.emit('duel:select_sum',{player:0,sumMode:0,acc:7,min:1,max:1,must:[],select:[{code:1001,c:0,l:4,s:2,param:(7<<16)|2}]}));
  await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
  await page.mouse.click(fieldClick.x,fieldClick.y);
  await page.waitForFunction(()=>window.__fieldReplies.at(-1)?.kind==='sum');
  check('native-field-alternative-sum',await page.evaluate(()=>window.__fieldReplies.at(-1).count===1));
  await page.evaluate(()=>{
    window.__ordinaryReplies=0;
    window.__parityBridge.respondSelectCard=()=>window.__ordinaryReplies++;
    window.__parityBus.emit('duel:select_card',{player:0,cancelable:true,min:1,max:1,cards:[{code:1001,c:0,l:4,s:2}]});
  });
  await page.locator('#hint-bar').filter({hasText:'请选择(1-1)'}).waitFor();
  await page.mouse.click(fieldClick.x,fieldClick.y);
  await page.waitForFunction(()=>window.__ordinaryReplies>0);
  check('ordinary-field-selection-replies-once',await page.evaluate(()=>window.__ordinaryReplies===1));
  await page.screenshot({path:out+'/cpp-duel.png'});
  await page.locator('#surrender-btn').click();await page.locator('#surrender-yes').click();
  check('surrender-sent-once',await page.evaluate(()=>window.__surrenders===1));check('surrender-waits-for-server',await page.locator('#duel-screen').count()===1);
  check('no-runtime-errors',errors.length===0);
  fs.writeFileSync(out+'/ui-parity-results.json',JSON.stringify({checks,errors},null,2));
 }catch(e){await page.screenshot({path:out+'/cpp-parity-failure.png'});throw e;}
 finally{await browser.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});
