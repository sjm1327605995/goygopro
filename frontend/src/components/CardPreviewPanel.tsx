/**
 * 左侧卡牌预览面板（原版 wCardImg + wInfos Info/Log 页签）。
 *
 * 数据源：duelStore.inspection（field3d 悬停回调通过 duelStore.inspect
 * 写入；卡信息与卡图异步 getCard/getCardImage 回填）。
 * 视觉按原版：177×254 卡图 + 页签化的名称/类型/种族属性等级/效果文本。
 */
import React, { useEffect, useRef, useState, useSyncExternalStore } from 'react';
import * as Tabs from '@radix-ui/react-tabs';
import { duelStore } from '../duel/store.ts';
import { settingsStore } from '../domain/settings.ts';
import { WailsBridge } from '../wails_bridge.ts';
import {formatCardName,formatCardTypeLine,formatCardStats,splitPendulumDescription} from '../domain/card_information.ts';
import {TYPE_MONSTER,TYPE_PENDULUM} from '../domain/constants.ts';
import CardImageFrame from '../ui/CardImageFrame.tsx';

export default function CardPreviewPanel({ fluidImage = false }: { fluidImage?: boolean } = {}) {
  const state = useSyncExternalStore(duelStore.subscribe, duelStore.getState);
  const settings = useSyncExternalStore(settingsStore.subscribe, settingsStore.getSnapshot);
  const insp = state.inspection;
  const [tab, setTab] = useState<'info' | 'log'>('info');
  const [img, setImg] = useState<string | null>(null);
  const logRef = useRef<HTMLDivElement | null>(null);

  const code = insp ? insp.code : null;
  useEffect(() => {
    let alive = true;
    if (!code) { setImg(null); return undefined; }
    WailsBridge.getCard(code).then(async (info) => {
      const original=info?.alias?await WailsBridge.getCard(info.alias):null;
      if (alive) duelStore.resolveInspectInfo(code, original?{...info,originalName:original.name}:info);
    });
    WailsBridge.getCardImage(code).then((pic) => {
      if (alive) setImg(pic && pic.url ? pic.url : null);
    });
    return () => { alive = false; };
  }, [code]);

  useEffect(() => {
    const el = logRef.current;
    if (tab === 'log' && el) el.scrollTop = el.scrollHeight;
  }, [tab, state.log]);

  const info = (insp && insp.info) || null;
  const type = info ? Number(info.type || 0) : 0;
  const isMonster = (type & TYPE_MONSTER) !== 0;
  const description = info ? String(info.desc ?? '') : '';
  const pendulumDescription = info && (type & TYPE_PENDULUM)
    ? splitPendulumDescription(description)
    : null;

  return (
    <div id="card-preview-panel" className="card-preview-panel">
      <CardImageFrame className="preview-image-box" fluid={fluidImage}>
        {img
          ? <img className="preview-image" src={img} alt="" />
          : <img className="preview-image" src="textures/cover.jpg" alt="" />}
      </CardImageFrame>
      {/* Radix Tabs：键盘左右切换 + roving tabindex；.active 类保留给冒烟/样式。
          两个 Tabs.Content 都在 Root 内，条件渲染保持旧的「非活动页不挂载」语义。 */}
      <Tabs.Root
        className="preview-winfos"
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
            {fluidImage && !code && <p className="arena-preview-empty">悬停场上卡片或手牌<br />查看卡片详情与效果</p>}
            <div id="preview-card-name" className="preview-name" title={info ? formatCardName(info) : undefined}>
              {info ? formatCardName(info) : ''}
            </div>
            {info && type ? <div className="preview-type">{formatCardTypeLine(info)}</div> : null}
            {info && isMonster ? (
              <div className="preview-stats">
                {formatCardStats(info)}
              </div>
            ) : null}
            {info && !settings.hide_setname && Array.isArray(info.setNames) && info.setNames.length ? <div className="preview-setnames">系列：{info.setNames.join(' / ')}</div> : null}
            <div id="preview-card-desc" className="preview-desc">
              {pendulumDescription ? (
                <>
                  {pendulumDescription[0] && (
                    <section className="preview-effect-section">
                      <div className="preview-effect-heading">灵摆效果</div>
                      <div className="preview-effect-text">{pendulumDescription[0]}</div>
                    </section>
                  )}
                  {pendulumDescription[1] && (
                    <section className="preview-effect-section">
                      <div className="preview-effect-heading">怪兽效果</div>
                      <div className="preview-effect-text">{pendulumDescription[1]}</div>
                    </section>
                  )}
                </>
              ) : description}
            </div>
          </Tabs.Content>
        ) : (
          <Tabs.Content value="log" id="preview-log" className="preview-log" ref={logRef}>
            {/* duel-log-list：日志条目的稳定锚点（旧右侧 LogDrawer 已删，
                原版 wInfos 只有这一个 Log 页签） */}
            <div id="duel-log-list">
              {state.log.slice(-50).map((e) => (
                <div key={e.id} className={`preview-log-line ${e.cls}`}>{e.text}</div>
              ))}
            </div>
          </Tabs.Content>
        )}
      </Tabs.Root>
    </div>
  );
}
