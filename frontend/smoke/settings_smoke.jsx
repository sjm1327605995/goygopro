// Headless smoke for Wave A: the settings system (system.conf mirror).
//
// WailsBridge.getConfig/saveConfig are monkey-patched to a fake conf file
// BEFORE App mounts, so settingsStore.load() exercises the exact production
// path (bridge → store → useSyncExternalStore consumers). Assertions:
//   1. non-default conf backfills the settings panel UI
//   2. toggling a checkbox debounce-writes a patch through saveConfig
//   3. volume slider writes sound_volume and shows the numeric value
//   4. mute_opponent/mute_spectators actually filter stoc:chat in ChatOverlay
//   5. volumes reach soundManager (sound_volume 80 → sfxVolume 0.8)
//   6. autochain=1 seeds the chain-pref mode ('always')
//   7. Lobby prefill reads nickname/lasthost+lastport/gamename/serverport
import { createRoot } from 'react-dom/client';
import React from 'react';
import { eventBus, WailsBridge } from '../src/wails_bridge.ts';
import '../css/duel-original.css';
import App from '../src/App.tsx';
import Lobby from '../src/components/Lobby.tsx';
import ChatOverlay from '../src/components/ChatOverlay.tsx';
import { settingsStore } from '../src/domain/settings.ts';
import chainPrefs from '../src/duel/chain_prefs.ts';
import { soundManager } from '../src/audio/sound_manager.ts';
import { duelStore } from '../src/duel/store.ts';

soundManager.muted = true; // smoke 里别真的出声

// ---- 假 system.conf：全部是非默认值，验证回填而非碰巧命中默认 ----
const fakeConf = {
  sound_volume: 80,
  music_volume: 30,
  mute_opponent: 1,
  mute_spectators: 1,
  autochain: 1,
  nickname: '海马瀬人',
  gamename: ' kc 杯 ',
  lasthost: '10.0.0.5',
  lastport: '7912',
  serverport: 7920,
};

const savePatches = [];
WailsBridge.getConfig = async () => ({ ...fakeConf });
WailsBridge.saveConfig = async (patch) => {
  savePatches.push({ ...patch });
  return patch;
};
// Lobby 连接相关桥在这里用不到，给个安全兜底
WailsBridge.listDecks = async () => [];
WailsBridge.connectServer = async () => ({ success: true });
WailsBridge.startLocalServer = async () => ({ success: true });
WailsBridge.createGame = async () => ({ success: true });

const waitFor = (predicate, timeoutMs = 5000, what = 'waitFor') => new Promise((resolve, reject) => {
  const started = Date.now();
  const tick = () => {
    if (predicate()) return resolve();
    if (Date.now() - started > timeoutMs) return reject(new Error(`${what} timeout`));
    setTimeout(tick, 25);
  };
  tick();
});

const checks = {};
const failed = [];

function check(name, ok) {
  checks[name] = ok;
  if (!ok) failed.push(name);
}

const root = createRoot(document.getElementById('root'));
// App 提供 ⚙ 按钮 + 音量/连锁键接线 + SettingsPanel；Lobby 并排挂载验证预填；
// ChatOverlay 单独挂载（正式树里它在 DuelStage 内部）
root.render(
  <React.Fragment>
    <App />
    <ChatOverlay />
    <div style={{ position: 'fixed', left: '-9999px', top: 0 }}>
      <Lobby onNavigate={() => {}} />
    </div>
  </React.Fragment>,
);

(async () => {
// ---------- 1. 配置加载 ----------
await waitFor(() => settingsStore.get('sound_volume') === 80, 5000, 'conf load');
check('conf-loaded', settingsStore.get('sound_volume') === 80 && settingsStore.get('nickname') === '海马瀬人');

// ---------- 2. 音量到达 soundManager（80/30 → 0.8/0.3；必须在改滑条前测） ----------
check(
  'soundmanager-volumes',
  Math.abs(soundManager.sfxVolume - 0.8) < 1e-9 && Math.abs(soundManager.bgmVolume - 0.3) < 1e-9,
);

// ---------- 3. 聊天屏蔽（settings → ChatOverlay 联动；此刻 mute_opponent/
// mute_spectators 都是 1，须在面板开关打乱配置前测） ----------
// playerSlot 默认 0：player 1 是对手、≥7 是观战者（netserver.cpp:380-385）
eventBus.emit('stoc:chat', { player: 1, msg: '对手的话应该被屏蔽' });
eventBus.emit('stoc:chat', { player: 0, msg: '自己的话要显示' });
eventBus.emit('stoc:chat', { player: 9, msg: '观战者的话应该被屏蔽' });
await new Promise((r) => setTimeout(r, 50));
const chatText = document.getElementById('chat-overlay')
  ? document.getElementById('chat-overlay').textContent : '';
check('opponent-chat-filtered', !chatText.includes('对手的话应该被屏蔽'));
check('own-chat-shown', chatText.includes('自己的话要显示'));
check('spectator-chat-filtered', !chatText.includes('观战者的话应该被屏蔽'));

// ---------- 4. 设置面板 UI 回填 ----------
document.getElementById('btn-open-settings').click();
await waitFor(() => document.getElementById('settings-panel'), 3000, 'settings open');

const volInput = document.querySelector('[data-setting="sound_volume"]');
check('volume-backfilled', volInput && volInput.value === '80');
check('mute-opponent-checked', document.querySelector('[data-setting="mute_opponent"]').checked === true);
check('volume-shown', document.querySelector('[data-value-for="sound_volume"]').textContent === '80');

// ---------- 5. 页签切换 + helper 页回填 ----------
document.querySelector('[data-tab="helper"]').click();
await waitFor(() => document.querySelector('[data-page="helper"]'), 3000, 'helper tab');
check('autochain-backfilled', document.querySelector('[data-setting="autochain"]').checked === true);

// 回系统页做滑条/开关用例
document.querySelector('[data-tab="system"]').click();
await waitFor(() => document.querySelector('[data-page="system"]'), 3000, 'system tab');

// ---------- 6. 开关写回（debounce 300ms 落到 saveConfig） ----------
const before = savePatches.length;
document.querySelector('[data-setting="mute_opponent"]').click();
await waitFor(() => savePatches.length > before, 3000, 'save fired');
check(
  'toggle-writes-patch',
  savePatches.slice(before).some((p) => p.mute_opponent === 0),
);

// ---------- 7. 音量滑条：写回 + 数值显示 ----------
const vol = document.querySelector('[data-setting="sound_volume"]');
// React 受控输入：必须走原生 value setter 再派发 input，否则 onChange 不触发
const valueSetter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set;
valueSetter.call(vol, '65');
vol.dispatchEvent(new Event('input', { bubbles: true }));
await waitFor(() => savePatches.some((p) => p.sound_volume === 65), 3000, 'volume save');
check('volume-value-shown', document.querySelector('[data-value-for="sound_volume"]').textContent === '65');

// ---------- 8. 连锁偏好初始态（autochain=1 → 'always'） ----------
check('chain-pref-seeded', chainPrefs.get() === 'always');

// ---------- 9. 大厅预填（波 H 后按 id 取：原版 ebJoinHost/ebJoinPort 本就分两框） ----------
const g = (id) => document.getElementById(id);
check('lobby-host-prefilled', g('lobby-join-host') && g('lobby-join-host').value === '10.0.0.5');
check('lobby-port-prefilled', g('lobby-join-port') && g('lobby-join-port').value === '7912');
check('lobby-nickname-prefilled', g('lobby-nickname') && g('lobby-nickname').value === '海马瀬人');
check('lobby-localport-prefilled', g('lobby-local-port') && g('lobby-local-port').value === '7920');
check('lobby-roomname-prefilled', g('lobby-room-name') && g('lobby-room-name').value === ' kc 杯 ');

// 汇总
const total = Object.keys(checks).length;
window.__smokeResult = {
  ok: failed.length === 0,
  passed: total - failed.length,
  total,
  failed,
  checks,
  chainPref: chainPrefs.get(),
};
console.log(`[settings_smoke] ${total - failed.length}/${total} passed`, failed.length ? failed : '');
})();
