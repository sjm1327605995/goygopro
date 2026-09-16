/**
 * 环境设置（原版 wInfos 的 tabSystem 系统设置 + tabHelper 决斗辅助两页，
 * game.cpp:380-501；Web 客户端做成独立设置弹窗）。
 *
 * 每个控件直接读写 settingsStore —— 落盘由 store debounce 处理。
 * 不复刻的键：窗口尺寸/字体/D3D 等桌面专属项（web 无意义），见 settings.ts。
 */
import React, { useState } from 'react';
import { useSyncExternalStore } from 'react';
import { settingsStore } from '../domain/settings.ts';
import GameDialog from './GameDialog.tsx';

function CheckRow({ k, label, hint }: { k: string; label: string; hint?: string }) {
  const s = useSyncExternalStore(settingsStore.subscribe, settingsStore.getSnapshot);
  const checked = !!s[k];
  return (
    <label className="settings-row" title={hint || ''}>
      <input
        type="checkbox"
        data-setting={k}
        checked={checked}
        onChange={(e) => settingsStore.set(k, e.target.checked ? 1 : 0)}
      />
      <span className="settings-row-label">{label}</span>
    </label>
  );
}

function VolumeRow({ k, label }: { k: string; label: string }) {
  const s = useSyncExternalStore(settingsStore.subscribe, settingsStore.getSnapshot);
  return (
    <label className="settings-row settings-row-slider">
      <span className="settings-row-label">{label}</span>
      <input
        type="range"
        data-setting={k}
        min={0}
        max={100}
        value={Number(s[k]) || 0}
        onChange={(e) => settingsStore.set(k, Number(e.target.value))}
      />
      <span className="settings-row-value" data-value-for={k}>{String(s[k])}</span>
    </label>
  );
}

export default function SettingsPanel({ open, onClose }: { open: boolean; onClose: () => void }) {
  const [tab, setTab] = useState<'system' | 'helper'>('system');

  return (
    <GameDialog open={open}>
      {open ? (
        <div id="settings-panel" className="modal-box settings-panel gfw-window" style={{ position: 'static', transform: 'none', width: '360px' }}>
          <div className="gfw-title">环境设置</div>
          <div className="settings-tabs">
            <div
              className={`settings-tab${tab === 'system' ? ' active' : ''}`}
              data-tab="system"
              onClick={() => setTab('system')}
            >系统设定</div>
            <div
              className={`settings-tab${tab === 'helper' ? ' active' : ''}`}
              data-tab="helper"
              onClick={() => setTab('helper')}
            >辅助功能</div>
          </div>
          {tab === 'system' ? (
            <div className="settings-body" data-page="system">
              <CheckRow k="mute_opponent" label="禁用聊天功能" />
              <CheckRow k="mute_spectators" label="忽略观战者发言" />
              <CheckRow k="hide_player_name" label="隐藏玩家昵称" />
              <CheckRow k="hide_setname" label="卡片信息隐藏系列名" />
              <CheckRow k="enable_sound" label="开启音效" />
              <VolumeRow k="sound_volume" label="音效音量" />
              <CheckRow k="enable_music" label="开启音乐" />
              <VolumeRow k="music_volume" label="音乐音量" />
              <CheckRow k="draw_field_spell" label="显示场地魔法背景" />
            </div>
          ) : (
            <div className="settings-body" data-page="helper">
              <CheckRow k="automonsterpos" label="自动选择怪兽卡片位置" />
              <CheckRow k="autospellpos" label="自动选择魔陷卡片位置" />
              <CheckRow k="randompos" label="↑随机选择位置" />
              <CheckRow k="autochain" label="自动发动并排序必发效果" />
              <CheckRow k="waitchain" label="没有可连锁的卡时延迟回应" />
              <CheckRow k="showchain" label="开局默认显示所有时点" />
              <CheckRow k="quick_animation" label="加快动画效果" />
              <CheckRow k="auto_save_replay" label="自动保存录像" />
              <CheckRow k="draw_single_chain" label="只有连锁1也显示连锁动画" />
            </div>
          )}
          <div className="settings-footer">
            <button id="settings-close" className="gfw-btn btn" onClick={onClose}>关闭</button>
          </div>
        </div>
      ) : null}
    </GameDialog>
  );
}
