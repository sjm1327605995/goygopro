// Minimal CDP smoke runner: launches headless chrome with remote debugging,
// waits for the page's window.__<name>Smoke.ready flag, prints checks JSON.
// usage: node cdp_smoke.js <page> <debugPort> <serverPort>
const { spawn } = require('child_process');

const page = process.argv[2];
const port = Number(process.argv[3] || 9600);
const serverPort = Number(process.argv[4] || 9500);
const CHROME = 'C:/Program Files/Google/Chrome/Application/chrome.exe';

const chrome = spawn(CHROME, [
  '--headless=new', '--disable-gpu', '--window-size=1280,800',
  `--remote-debugging-port=${port}`, '--user-data-dir=C:/tmp/cdp-profile-' + port,
  'about:blank',
], { stdio: 'ignore' });

const wait = (ms) => new Promise(r => setTimeout(r, ms));

async function getWsUrl() {
  for (let i = 0; i < 50; i++) {
    try {
      const res = await fetch(`http://127.0.0.1:${port}/json/list`);
      const list = await res.json();
      const pg = list.find(t => t.type === 'page');
      if (pg) return pg.webSocketDebuggerUrl;
    } catch {}
    await wait(200);
  }
  throw new Error('chrome devtools not reachable');
}

(async () => {
  const ws = new WebSocket(await getWsUrl());
  let id = 0;
  const pending = new Map();
  const send = (method, params = {}) => new Promise((resolve, reject) => {
    const mid = ++id;
    pending.set(mid, { resolve, reject });
    ws.send(JSON.stringify({ id: mid, method, params }));
  });
  ws.onmessage = (ev) => {
    const msg = JSON.parse(ev.data);
    if (msg.id && pending.has(msg.id)) {
      pending.get(msg.id).resolve(msg.result);
      pending.delete(msg.id);
    }
  };
  await new Promise(r => { ws.onopen = r; });
  await send('Page.enable');
  await send('Page.navigate', { url: `http://127.0.0.1:${serverPort}/${page}.html` });
  // wait for ready flag (up to 90s real time)
  let checks = null;
  for (let i = 0; i < 180; i++) {
    await wait(500);
    const res = await send('Runtime.evaluate', {
      expression: `(() => {
        if (window.__smokeResult) return { key: '__smokeResult', checks: window.__smokeResult };
        const k = Object.keys(window).find(k => /Smoke$/.test(k) && window[k] && window[k].checks && window[k].ready);
        if (k) return { key: k, checks: window[k].checks };
        return null;
      })()`,
      returnByValue: true,
    });
    if (res && res.result && res.result.value) { checks = res.result.value; break; }
  }
  if (!checks) { console.log('TIMEOUT waiting for ready'); process.exit(2); }
  console.log(JSON.stringify(checks.checks, null, 1));
  process.exit(0);
})().catch(e => { console.error('ERR', e.message); process.exit(1); });
