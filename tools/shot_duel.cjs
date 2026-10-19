const pw = require(process.env.PLAYWRIGHT_MODULE_PATH || 'playwright');
(async () => {
  const browser = await pw.chromium.launch({ headless: true, executablePath: process.env.CHROME_PATH || 'C:/Program Files/Google/Chrome/Application/chrome.exe' });
  const page = await browser.newPage({ viewport: { width: 1600, height: 900 } });
  await page.goto('http://127.0.0.1:19500/stage_smoke.html');
  await page.waitForFunction(() => window.__stageSmoke && window.__stageSmoke.ready, null, { timeout: 45000 }).catch(() => {});
  await page.waitForTimeout(1200);
  await page.screenshot({ path: process.argv[2] || 'tmp_dbg/duel-before.png' });
  await browser.close();
})();
