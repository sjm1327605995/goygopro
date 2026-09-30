// 双客户端真实 E2E：启动两个 client.exe（独立 WebView2 数据目录 + CDP 端口），
// A 建房 → B 加入，核对两端座位渲染与 debug_net.log。
// usage: node two_client_e2e.js
const { spawn } = require('child_process');
const fs = require('fs');

const EXE = 'E:/workspace/goygopro/client.exe';
const CWD = 'E:/workspace/goygopro';
const PORT_A = 9333, PORT_B = 9444;

const wait = (ms) => new Promise(r => setTimeout(r, ms));

function launch(name, port) {
  const env = {
    ...process.env,
    YGO_CDP_PORT: String(port),
    YGO_USER_DATA: `C:/tmp/wv2-e2e-${name}`,
  };
  const p = spawn(EXE, [], { cwd: CWD, env, stdio: 'ignore' });
  p.on('error', (e) => console.error(`[${name}] spawn error:`, e.message));
  return p;
}

async function getWsUrl(port) {
  for (let i = 0; i < 100; i++) {
    try {
      const res = await fetch(`http://127.0.0.1:${port}/json/list`);
      const list = await res.json();
      const pg = list.find(t => t.type === 'page' && !t.url.startsWith('devtools'));
      if (pg) return pg.webSocketDebuggerUrl;
    } catch {}
    await wait(300);
  }
  throw new Error(`cdp :${port} not reachable`);
}

async function connectCDP(port) {
  const ws = new WebSocket(await getWsUrl(port));
  let id = 0;
  const pending = new Map();
  ws.onmessage = (ev) => {
    const msg = JSON.parse(ev.data);
    if (msg.id && pending.has(msg.id)) {
      pending.get(msg.id).resolve(msg.result);
      pending.delete(msg.id);
    }
  };
  await new Promise(r => { ws.onopen = r; });
  const send = (method, params = {}) => new Promise((resolve) => {
    const mid = ++id;
    pending.set(mid, { resolve });
    ws.send(JSON.stringify({ id: mid, method, params }));
  });
  const evalJs = async (expression) => {
    const res = await send('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true });
    if (res && res.exceptionDetails) throw new Error('page js error: ' + JSON.stringify(res.exceptionDetails.exception?.description || res.exceptionDetails.text));
    return res && res.result ? res.result.value : undefined;
  };
  // run 在求值前先确保 __e2e 辅助对象存在（页面导航/重载会清掉 window 上的注入）。
  const run = (expr) => evalJs(`(function(){ ${HELPERS} ; return (${expr}); })()`);
  const waitFor = async (expr, timeoutMs = 30000, label = expr) => {
    const t0 = Date.now();
    while (Date.now() - t0 < timeoutMs) {
      try { if (await run(expr)) return true; } catch {}
      await wait(400);
    }
    throw new Error('TIMEOUT waiting: ' + label);
  };
  return { send, evalJs, run, waitFor };
}

// 页面内辅助函数（注入一次）
const HELPERS = `
window.__e2e = {
  byText(sel, text) {
    return [...document.querySelectorAll(sel)].find(b => (b.textContent || '').includes(text) && b.offsetParent !== null) || null;
  },
  click(el) { if (el) el.dispatchEvent(new MouseEvent('click', { bubbles: true })); },
  setInput(id, v) {
    const el = document.getElementById(id);
    if (!el) return false;
    const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;
    setter.call(el, v);
    el.dispatchEvent(new Event('input', { bubbles: true }));
    return true;
  },
  visible(id) { const el = document.getElementById(id); return !!el && el.offsetParent !== null; },
  seatText() { const el = document.getElementById('room-lobby-panel'); return el ? el.innerText : '(no room panel)'; },
  joinStatus() { const el = document.getElementById('lobby-join-status'); return el ? el.textContent : ''; },
  errMsg() { const el = document.getElementById('lobby-errmsg-ok'); return el ? el.parentElement.parentElement.textContent : ''; },
};
true`;

(async () => {
  const checks = {};
  const rec = (k, v) => { checks[k] = v; console.log(`${v ? 'PASS' : 'FAIL'}  ${k}`); };

  console.log('launching A and B ...');
  const pa = launch('A', PORT_A);
  const pb = launch('B', PORT_B);
  const kill = () => { try { pa.kill(); } catch {} try { pb.kill(); } catch {} };
  process.on('exit', kill);

  const A = await connectCDP(PORT_A);
  const B = await connectCDP(PORT_B);

  // --- 双方都进联机窗口 ---
  await A.waitFor(`!!document.getElementById('menu-btn-lan')`, 30000, 'A menu');
  await B.waitFor(`!!document.getElementById('menu-btn-lan')`, 30000, 'B menu');
  await A.run(`__e2e.click(document.getElementById('menu-btn-lan'))`);
  await B.run(`__e2e.click(document.getElementById('menu-btn-lan'))`);
  await A.waitFor(`__e2e.visible('server-connect-panel')`, 10000, 'A lan panel');
  await B.waitFor(`__e2e.visible('server-connect-panel')`, 10000, 'B lan panel');

  // --- A 建房（默认参数，空密码）---
  await A.run(`__e2e.click(__e2e.byText('#server-connect-panel .btn', '建立主机'))`);
  await A.waitFor(`!!__e2e.byText('.gfw-create .btn', '确定')`, 10000, 'A create panel');
  await A.run(`__e2e.click(__e2e.byText('.gfw-create .btn', '确定'))`);
  try {
    await A.waitFor(`__e2e.visible('room-lobby-panel')`, 30000, 'A in room');
  } catch (e) {
    console.log('A diagnostics after create timeout:');
    console.log('  visible panels:', JSON.stringify(await A.run(`['server-connect-panel','room-lobby-panel'].map(id => id + '=' + __e2e.visible(id))`)));
    console.log('  create panel visible:', await A.run(`!!__e2e.byText('.gfw-create .btn', '确定')`));
    console.log('  errMsg:', JSON.stringify(await A.run(`__e2e.errMsg()`)));
    console.log('  joinStatus:', JSON.stringify(await A.run(`__e2e.joinStatus()`)));
    console.log('  body snippet:', JSON.stringify((await A.evalJs(`document.body.innerText.slice(0, 400)`))));
    throw e;
  }
  rec('A-in-room', true);
  console.log('A seats:', JSON.stringify(await A.run(`__e2e.seatText()`)));

  // --- B 加入 ---
  await B.run(`__e2e.setInput('lobby-join-host', '127.0.0.1')`);
  await B.run(`__e2e.setInput('lobby-join-port', '7911')`);
  await B.run(`__e2e.setInput('lobby-host-pass', '')`);
  await B.run(`__e2e.click(__e2e.byText('#server-connect-panel .btn', '加入游戏'))`);
  await wait(3000);
  const bInRoom = await B.run(`__e2e.visible('room-lobby-panel')`);
  rec('B-in-room', bInRoom);
  rec('B-join-status-cleared', (await B.run(`__e2e.joinStatus()`)) === '');
  const bErr = await B.run(`__e2e.errMsg()`);
  rec('B-no-error-popup', bErr === '');
  if (bErr) console.log('B error popup:', bErr);

  await wait(1500);
  const seatsA = await A.run(`__e2e.seatText()`);
  const seatsB = await B.run(`__e2e.seatText()`);
  console.log('--- A seats ---\n' + seatsA);
  console.log('--- B seats ---\n' + seatsB);
  // 两端昵称同源（共享 cwd 的 system.conf），按占用座位行数断言
  const namesA = (seatsA.match(/已连接/g) || []).length;
  const namesB = (seatsB.match(/已连接/g) || []).length;
  rec('A-sees-2-players', namesA >= 2);
  rec('B-sees-2-players', namesB >= 2);

  console.log('--- debug_net.log tail ---');
  try {
    const log = fs.readFileSync(CWD + '/debug_net.log', 'utf8').trim().split('\n');
    console.log(log.slice(-25).join('\n'));
  } catch { console.log('(no debug_net.log)'); }

  kill();
  const fails = Object.entries(checks).filter(([, v]) => !v);
  console.log(fails.length === 0 ? '\nALL PASS' : `\n${fails.length} FAILED: ` + fails.map(([k]) => k).join(', '));
  process.exit(fails.length === 0 ? 0 : 1);
})().catch(e => { console.error('FATAL', e.message); process.exit(2); });
