const pw=require(process.env.PLAYWRIGHT_MODULE_PATH||'playwright');const fs=require('node:fs');
(async()=>{const browser=await pw.chromium.launch({headless:true,executablePath:process.env.CHROME_PATH||'C:/Program Files/Google/Chrome/Application/chrome.exe'});
const results=[];try{for(const name of process.argv.slice(2)){
 const page=await browser.newPage({viewport:{width:1024,height:640}});const errors=[];page.on('pageerror',e=>errors.push(e.message));
 await page.goto('http://127.0.0.1:19500/'+name+'_smoke.html');
 // 页面侧的全局键并不全是 __<name>Smoke：settings 汇总在 __smokeResult
 // （完成时才赋值，无 ready 字段）、netplay_lobby 用 __netplaySmoke。
 const KEY={ui_parity:'__uiParitySmoke',netplay_lobby:'__netplaySmoke',settings:'__smokeResult'};
 const key=KEY[name]||'__'+name+'Smoke';
 try{await page.waitForFunction(k=>window[k]&&window[k].ready!==false,key,{timeout:45000});}catch(e){errors.push(e.message);}
 const state=await page.evaluate(k=>window[k]?.checks||{},key);const failed=Object.entries(state).filter(([k,v])=>v===false||k==='fatalMsg'||k==='fatal');
 results.push({name,total:Object.keys(state).length,failed,errors});console.log(JSON.stringify(results.at(-1)));await page.close();
}fs.writeFileSync('tmp_dbg/ui-smoke-results.json',JSON.stringify(results,null,2));if(results.some(x=>x.failed.length||x.errors.length))process.exitCode=1;}finally{await browser.close();}})().catch(e=>{console.error(e);process.exitCode=1;});
