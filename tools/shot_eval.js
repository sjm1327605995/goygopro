// Screenshot with a pre-shot eval: node shot_eval.js <page> <debugPort> <serverPort> <out.png> <waitMs> <evalJsFile>
const { spawn } = require('child_process');
const fs = require('fs');
const page = process.argv[2];
const port = Number(process.argv[3] || 9610);
const serverPort = Number(process.argv[4] || 9501);
const out = process.argv[5] || `shots/${page}.png`;
const waitMs = Number(process.argv[6] || 3000);
const evalFile = process.argv[7];
const CHROME = 'C:/Program Files/Google/Chrome/Application/chrome.exe';
const chrome = spawn(CHROME, [
  '--headless=new', '--disable-gpu', '--window-size=1280,800',
  `--remote-debugging-port=${port}`, '--user-data-dir=C:/tmp/cdp-shot-' + port,
  'about:blank',
], { stdio: 'ignore' });
const wait = (ms) => new Promise(r => setTimeout(r, ms));
async function getWsUrl() {
  for (let i = 0; i < 50; i++) {
    try {
      const res = await fetch(`http://127.0.0.1:${port}/json/list`);
      const pg = (await res.json()).find(t => t.type === 'page');
      if (pg) return pg.webSocketDebuggerUrl;
    } catch {}
    await wait(200);
  }
  throw new Error('no devtools');
}
(async () => {
  const ws = new WebSocket(await getWsUrl());
  let id = 0; const pending = new Map();
  const send = (m, p = {}) => new Promise((res, rej) => { const i = ++id; pending.set(i, { res, rej }); ws.send(JSON.stringify({ id: i, method: m, params: p })); });
  ws.onmessage = (ev) => { const m = JSON.parse(ev.data); if (m.id && pending.has(m.id)) { pending.get(m.id).res(m.result); pending.delete(m.id); } };
  await new Promise(r => { ws.onopen = r; });
  await send('Page.enable');
  await send('Page.navigate', { url: `http://127.0.0.1:${serverPort}/${page}.html` });
  await wait(waitMs);
  if (evalFile) {
    const expression = fs.readFileSync(evalFile, 'utf8');
    await send('Runtime.evaluate', { expression, awaitPromise: true });
    await wait(400);
  }
  const shot = await send('Page.captureScreenshot', { format: 'png' });
  if (shot && shot.data) { fs.writeFileSync(out, Buffer.from(shot.data, 'base64')); console.log('saved', out); }
  else console.log('FAILED');
  chrome.kill();
  process.exit(0);
})().catch(e => { console.error('FATAL', e.message); chrome.kill(); process.exit(1); });
