/**
 * 左侧卡牌预览面板（原版 wCardImg + wInfos Info/Log 页签）。
 *
 * 数据源：duelStore.inspection（field3d 悬停回调通过 duelStore.inspect
 * 写入；卡信息与卡图异步 getCard/getCardImage 回填）。
 * 视觉按原版：177×254 卡图 + 页签化的名称/类型/种族属性等级/效果文本。
 */
import React, { useEffect, useState, useSyncExternalStore } from 'react';
import * as Tabs from '@radix-ui/react-tabs';
import { duelStore } from '../duel/store.ts';
import { WailsBridge } from '../wails_bridge.ts';
import {
  TYPE_MONSTER, TYPE_SPELL, TYPE_TRAP, TYPE_NORMAL, TYPE_EFFECT,
  TYPE_FUSION, TYPE_RITUAL, TYPE_SYNCHRO, TYPE_XYZ, TYPE_LINK,
  TYPE_PENDULUM, TYPE_QUICKPLAY, TYPE_CONTINUOUS, TYPE_EQUIP, TYPE_FIELD,
  TYPE_TUNER, TYPE_FLIP, TYPE_SPIRIT, TYPE_UNION, TYPE_DUAL,
  RACES, ATTRS,
} from '../domain/constants.ts';

const TYPE_SEGMENTS: [number, string][] = [
  [TYPE_MONSTER, '怪兽'], [TYPE_SPELL, '魔法'], [TYPE_TRAP, '陷阱'],
  [TYPE_NORMAL, '通常'], [TYPE_EFFECT, '效果'], [TYPE_FUSION, '融合'],
  [TYPE_RITUAL, '仪式'], [TYPE_SYNCHRO, '同调'], [TYPE_XYZ, '超量'],
  [TYPE_LINK, '链接'], [TYPE_PENDULUM, '灵摆'], [TYPE_QUICKPLAY, '速攻'],
  [TYPE_CONTINUOUS, '永续'], [TYPE_EQUIP, '装备'], [TYPE_FIELD, '场地'],
  [TYPE_TUNER, '调整'], [TYPE_FLIP, '反转'], [TYPE_SPIRIT, '灵魂'],
  [TYPE_UNION, '同盟'], [TYPE_DUAL, '二重'],
];

function typeLabel(type: number): string {
  return TYPE_SEGMENTS
    .filter(([bit]) => (type & bit) !== 0)
    .map(([, label]) => label)
    .join('/');
}

function attrRaceLine(type: number, info: Record<string, unknown>): string | null {
  if (!(type & TYPE_MONSTER)) return null;
  const attr = (ATTRS.find(([bit]) => bit === info.attribute) || [])[1];
  const race = (RACES.find(([bit]) => bit === info.race) || [])[1];
  const level = info.level;
  const parts: string[] = [];
  if (attr) parts.push(`${attr}属性`);
  if (race) parts.push(`${race}族`);
  if (level) parts.push(`${type & TYPE_XYZ ? '阶' : '星'} ${level}`);
  return parts.length ? parts.join('　') : null;
}

export default function CardPreviewPanel() {
  const state = useSyncExternalStore(duelStore.subscribe, duelStore.getState);
  const insp = state.inspection;
  const [tab, setTab] = useState<'info' | 'log'>('info');
  const [img, setImg] = useState<string | null>(null);

  const code = insp ? insp.code : null;
  useEffect(() => {
    let alive = true;
    if (!code) { setImg(null); return undefined; }
    WailsBridge.getCard(code).then((info) => {
      if (alive) duelStore.resolveInspectInfo(code, info);
    });
    WailsBridge.getCardImage(code).then((pic) => {
      if (alive) setImg(pic && pic.url ? pic.url : null);
    });
    return () => { alive = false; };
  }, [code]);

  const info = (insp && insp.info) || null;
  const type = info ? Number(info.type || 0) : 0;
  const isMonster = (type & TYPE_MONSTER) !== 0;

  return (
    <div id="card-preview-panel" className="card-preview-panel">
      <div className="preview-image-box">
        {img
          ? <img className="preview-image" src={img} alt="" />
          : <div className="preview-image preview-image-empty">{code ? `#${code}` : '卡片预览'}</div>}
      </div>
      {/* Radix Tabs：键盘左右切换 + roving tabindex；.active 类保留给冒烟/样式。
          两个 Tabs.Content 都在 Root 内，条件渲染保持旧的「非活动页不挂载」语义。 */}
      <Tabs.Root
        value={tab}
        onValueChange={(v) => setTab(v as 'info' | 'log')}
      >
        <Tabs.List className="preview-tabs">
          <Tabs.Trigger
            value="info"
            className={`preview-tab${tab === 'info' ? ' active' : ''}`}
          >卡片信息</Tabs.Trigger>
          <Tabs.Trigger
            value="log"
            className={`preview-tab${tab === 'log' ? ' active' : ''}`}
          >消息记录</Tabs.Trigger>
        </Tabs.List>
        {tab === 'info' ? (
          <Tabs.Content value="info" id="preview-info" className="preview-info">
            <div id="preview-card-name" className="preview-name">
              {info ? String(info.name ?? `#${code}`) : (code ? `#${code}` : '选择一张卡')}
            </div>
            {info && type ? <div className="preview-type">{typeLabel(type)}</div> : null}
            {info && attrRaceLine(type, info) ? (
              <div className="preview-attr">{attrRaceLine(type, info)}</div>
            ) : null}
            {info && isMonster ? (
              <div className="preview-stats">
                ATK {info.attack != null ? String(info.attack) : '?'} / DEF {info.defense != null ? String(info.defense) : '?'}
              </div>
            ) : null}
            <div id="preview-card-desc" className="preview-desc">
              {info ? String(info.desc ?? '') : '悬停场上的卡牌查看详情。'}
            </div>
          </Tabs.Content>
        ) : (
          <Tabs.Content value="log" id="preview-log" className="preview-log">
            {state.log.slice(-50).map((e) => (
              <div key={e.id} className={`preview-log-line ${e.cls}`}>{e.text}</div>
            ))}
          </Tabs.Content>
        )}
      </Tabs.Root>
    </div>
  );
}
