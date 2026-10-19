const pw = require(process.env.PLAYWRIGHT_MODULE_PATH || 'playwright');
(async () => {
  const browser = await pw.chromium.launch({ headless: true, executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe' });
  const page = await browser.newPage({ viewport: { width: 1600, height: 900 } });
  page.on('pageerror', (e) => console.log('PAGEERROR:', e.message));
  await page.goto('http://127.0.0.1:19500/stage_smoke.html');
  await page.waitForFunction(() => window.__stageSmoke && window.__stageSmoke.ready, null, { timeout: 45000 }).catch(() => {});
  await page.evaluate(() => {
    const eventBus = window.__stageSmoke.eventBus;
    eventBus.emit('duel:start', { playerType: 0, lp0: 8000, lp1: 8000, deck0: 40, extra0: 15, deck1: 40, extra1: 15 });
    eventBus.emit('duel:new_turn', { player: 0, turn: 3 });
    eventBus.emit('duel:select_idlecmd', { player: 0, toBP: 1, toEP: 1, shuffle: 1, summon: [], spsummon: [], repos: [], mset: [], sset: [], activate: [{ code: 89631139, c: 0, l: 0x4, s: 2, desc: 0 }] });
    eventBus.emit('duel:waiting', {});
    eventBus.emit('stoc:chat', { pos: 0, name: 'HostPlayer', msg: '决斗吧！' });
    eventBus.emit('stoc:time_limit', { player: 0, leftTime: 240 });
  });
  await page.waitForTimeout(900);
  await page.screenshot({ path: process.argv[2] || 'tmp_dbg/duel-full.png' });
  await browser.close();
})();
