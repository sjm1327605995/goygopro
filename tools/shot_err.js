// Dump console errors for a smoke page: node shot_err.js <page> <port> <serverPort>
const { spawn } = require('child_process');
const page = process.argv[2];
const port = Number(process.argv[3]);
const serverPort = Number(process.argv[4] || 9501);
const CHROME = 'C:/Program Files/Google/Chrome/Application/chrome.exe';
const chrome = spawn(CHROME, ['--headless=new', '--disable-gpu', '--window-size=1280,800',
  `--remote-debugging-port=${port}`, '--user-data-dir=C:/tmp/cdp-err-' + port, 'about:blank'], { stdio: 'ignore' });
const wait = (ms) => new Promise(r => setTimeout(r, ms));
(async () => {
  let wsUrl;
  for (let i = 0; i < 50 && !wsUrl; i++) {
    try { const l = await (await fetch(`http://127.0.0.1:${port}/json/list`)).json(); const p = l.find(t => t.type === 'page'); if (p) wsUrl = p.webSocketDebuggerUrl; } catch {}
    if (!wsUrl) await wait(200);
  }
  const ws = new WebSocket(wsUrl);
  let id = 0; const pending = new Map();
  const send = (m, p = {}) => new Promise((res) => { const i = ++id; pending.set(i, res); ws.send(JSON.stringify({ id: i, method: m, params: p })); });
  ws.onmessage = (ev) => {
    const m = JSON.parse(ev.data);
    if (m.id && pending.has(m.id)) { pending.get(m.id)(m.result); pending.delete(m.id); }
    if (m.method === 'Runtime.exceptionThrown') console.log('EXC:', JSON.stringify(m.params.exceptionDetails).slice(0, 800));
    if (m.method === 'Runtime.consoleAPICalled' && ['error', 'warning'].includes(m.params.type)) console.log(m.params.type.toUpperCase() + ':', m.params.args.map(a => a.value || a.description || '').join(' ').slice(0, 400));
  };
  await new Promise(r => { ws.onopen = r; });
  await send('Runtime.enable');
  await send('Page.enable');
  await send('Page.navigate', { url: `http://127.0.0.1:${serverPort}/${page}.html` });
  await wait(6000);
  chrome.kill(); process.exit(0);
})().catch(e => { console.error('FATAL', e.message); chrome.kill(); process.exit(1); });
