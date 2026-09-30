// One-off CDP eval printer: node tools/dbg_eval.js <page> <debugPort> <serverPort> <waitMs> <evalJsFile>
const { spawn } = require('child_process');
const fs = require('fs');
const [page, portS, serverS, waitS, evalFile] = process.argv.slice(2);
const port = Number(portS), serverPort = Number(serverS || 9501), waitMs = Number(waitS || 1000);
const CHROME = 'C:/Program Files/Google/Chrome/Application/chrome.exe';
const chrome = spawn(CHROME, ['--headless=new', '--disable-gpu', '--window-size=1280,800',
  `--remote-debugging-port=${port}`, '--user-data-dir=C:/tmp/cdp-eval-' + port, 'about:blank'], { stdio: 'ignore' });
const wait = (ms) => new Promise(r => setTimeout(r, ms));
(async () => {
  let ws;
  for (let i = 0; i < 50; i++) {
    try {
      const l = await (await fetch(`http://127.0.0.1:${port}/json/list`)).json();
      const pg = l.find(t => t.type === 'page');
      if (pg) { ws = new WebSocket(pg.webSocketDebuggerUrl); break; }
    } catch {}
    await wait(200);
  }
  if (!ws) { console.log('no devtools'); chrome.kill(); process.exit(1); }
  let id = 0; const pending = new Map();
  const send = (m, p = {}) => new Promise((res, rej) => { const i = ++id; pending.set(i, { res, rej }); ws.send(JSON.stringify({ id: i, method: m, params: p })); });
  ws.onmessage = (ev) => { const m = JSON.parse(ev.data); if (m.id && pending.has(m.id)) { pending.get(m.id).res(m.result); pending.delete(m.id); } };
  await new Promise(r => { ws.onopen = r; });
  await send('Page.enable');
  await send('Page.navigate', { url: `http://127.0.0.1:${serverPort}/${page}.html` });
  await wait(waitMs);
  const expression = fs.readFileSync(evalFile, 'utf8');
  const res = await send('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true });
  console.log(res.result && res.result.value !== undefined ? res.result.value : JSON.stringify(res));
  chrome.kill(); process.exit(0);
})().catch(e => { console.error('FATAL', e.message); chrome.kill(); process.exit(1); });
