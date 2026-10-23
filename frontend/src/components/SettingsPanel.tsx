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
import UiButton from '../ui/UiButton.tsx';
import { UiCheckbox } from '../ui/UiInput.tsx';
import GfwSelect from './ui/GfwSelect.tsx';
import { WailsBridge } from '../wails_bridge.ts';
import AppearancePanel from './AppearancePanel.tsx';

/** 原版 chkLFlist+cbLFlist（game.cpp:473-480）：勾选启用禁限卡表并选表，
 *  表序号写入 default_lflist（use_lflist=0 = N/A 无限制） */
function LFListRow() {
  const s = useSyncExternalStore(settingsStore.subscribe, settingsStore.getSnapshot);
  const [lists, setLists] = useState<{ hash: number; name: string }[]>([]);
  React.useEffect(() => {
    let alive = true;
    WailsBridge.listLFLists().then((r) => { if (alive && r) setLists(r); }).catch(() => {});
    return () => { alive = false; };
  }, []);
  const useList = !!s.use_lflist;
  const idx = Math.min(Math.max(Number(s.default_lflist ?? 0) || 0, 0), Math.max(lists.length - 1, 0));
  return (
    <>
      <UiCheckbox
        classNames={{ root: 'settings-row', label: 'settings-row-label' }}
        data-setting="use_lflist"
        checked={useList}
        onChange={(e) => settingsStore.set('use_lflist', e.target.checked ? 1 : 0)}
        label="使用禁限卡表"
      />
      <label className="settings-row" style={{ opacity: useList ? 1 : 0.5 }}>
        <span className="settings-row-label">默认禁限卡表</span>
        <GfwSelect
          id="settings-lflist-select"
          width={150}
          disabled={!useList}
          value={lists[idx] ? String(idx) : ''}
          onValueChange={(v) => settingsStore.set('default_lflist', Number(v))}
          options={lists.map((l, i) => ({ value: String(i), label: l.name }))}
        />
      </label>
    </>
  );
}

function CheckRow({ k, label, hint }: { k: string; label: string; hint?: string }) {
  const s = useSyncExternalStore(settingsStore.subscribe, settingsStore.getSnapshot);
  const checked = !!s[k];
  return (
    <UiCheckbox
      classNames={{ root: 'settings-row', label: 'settings-row-label' }}
      title={hint || ''}
      data-setting={k}
      checked={checked}
      onChange={(e) => settingsStore.set(k, e.target.checked ? 1 : 0)}
      label={label}
    />
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

// 动画速度（neos-ts animation 设置的等价物）：10-200%，100=原速
function SpeedRow({ k, label }: { k: string; label: string }) {
  const s = useSyncExternalStore(settingsStore.subscribe, settingsStore.getSnapshot);
  return (
    <label className="settings-row settings-row-slider">
      <span className="settings-row-label">{label}</span>
      <input
        type="range"
        data-setting={k}
        min={10}
        max={200}
        step={10}
        value={Number(s[k]) || 100}
        onChange={(e) => settingsStore.set(k, Number(e.target.value))}
      />
      <span className="settings-row-value" data-value-for={k}>{`${s[k]}%`}</span>
    </label>
  );
}

export default function SettingsPanel({ open, onClose }: { open: boolean; onClose: () => void }) {
  const [tab, setTab] = useState<'system' | 'helper' | 'appearance'>('system');

  return (
    <GameDialog open={open}>
      {open ? (
        <div id="settings-panel" className="modal-box settings-panel gfw-window" style={{ position: 'static', transform: 'none', width: '460px' }}>
          <div className="gfw-title">环境设置</div>
          <div className="settings-tabs">
            <button
              type="button"
              aria-pressed={tab === 'system'}
              className={`settings-tab${tab === 'system' ? ' active' : ''}`}
              data-tab="system"
              onClick={() => setTab('system')}
            >系统设定</button>
            <button
              type="button"
              aria-pressed={tab === 'helper'}
              className={`settings-tab${tab === 'helper' ? ' active' : ''}`}
              data-tab="helper"
              onClick={() => setTab('helper')}
            >辅助功能</button>
            <button type="button" aria-pressed={tab === 'appearance'} className={`settings-tab${tab === 'appearance' ? ' active' : ''}`} data-tab="appearance" onClick={() => setTab('appearance')}>决斗外观</button>
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
              <CheckRow k="swap_yes_no_button" label="左右互换「是/否」按钮位置" />
              <CheckRow k="hide_hint_button" label="隐藏提示按钮（完成/取消）" />
              <CheckRow k="music_mode" label="随机播放音乐" hint="music_mode：BGM 各场景多套音型随机（原版 chkMusicMode）" />
              <LFListRow />
            </div>
          ) : tab === 'appearance' ? <AppearancePanel /> : (
            <div className="settings-body" data-page="helper">
              <CheckRow k="automonsterpos" label="自动选择怪兽卡片位置" />
              <CheckRow k="autospellpos" label="自动选择魔陷卡片位置" />
              <CheckRow k="randompos" label="↑随机选择位置" />
              <CheckRow k="autochain" label="自动发动并排序必发效果" />
              <CheckRow k="waitchain" label="没有可连锁的卡时延迟回应" />
              <CheckRow k="showchain" label="开局默认显示所有时点" />
              <CheckRow k="quick_animation" label="加快动画效果" />
              <SpeedRow k="animation_speed" label="动画速度" />
              <CheckRow k="auto_save_replay" label="自动保存录像" />
              <CheckRow k="draw_single_chain" label="只有连锁1也显示连锁动画" />
              <CheckRow k="auto_search_limit" label="无搜索限制" hint="auto_search_limit：卡组编辑器搜索结果不分页上限（原版 chkAutoSearch）" />
              <CheckRow k="search_multiple_keywords" label="多个关键词搜索" hint="search_multiple_keywords：卡组编辑器搜索支持空格分隔多关键词（原版 chkMultiKeywords）" />
              <CheckRow k="ignore_deck_changes" label="更换卡组时不提示保存" hint="ignore_deck_changes：编辑器载入别的卡组前不再弹未保存确认（原版 chkIgnoreDeckChanges 的编辑器侧语义）" />
            </div>
          )}
          <div className="settings-footer">
            <UiButton id="settings-close" onClick={onClose}>关闭</UiButton>
          </div>
        </div>
      ) : null}
    </GameDialog>
  );
}
