import React, {useEffect, useRef, useState, useSyncExternalStore} from 'react';
import {APPEARANCE_DEFAULTS, appearanceStore, BACKGROUNDS, backgroundURL, prepareAppearanceImage, SLEEVES, sleeveURL} from '../domain/appearance.ts';
import {settingsStore} from '../domain/settings.ts';
import {UiCheckbox} from '../ui/UiInput.tsx';
import '../../css/appearance.css';

export default function AppearancePanel() {
  const value = useSyncExternalStore(appearanceStore.subscribe, appearanceStore.getSnapshot);
  const settings = useSyncExternalStore(settingsStore.subscribe, settingsStore.getSnapshot);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const alive = useRef(true);
  useEffect(() => () => { alive.current = false; }, []);
  const apply = (patch: Parameters<typeof appearanceStore.set>[0]) => {
    try { appearanceStore.set(patch); setError(''); }
    catch (err) { setError((err as Error).message); }
  };
  const upload = async (kind: 'sleeve' | 'background', file?: File) => {
    if (!file) return;
    setBusy(true); setError('');
    try {
      const url = await prepareAppearanceImage(file, kind);
      if (alive.current) apply(kind === 'sleeve'
        ? {sleeve: 'custom', customSleeve: url, sleeveName: file.name.slice(0, 100)}
        : {background: 'custom', customBackground: url, backgroundName: file.name.slice(0, 100)});
    } catch (err) { if (alive.current) setError((err as Error).message); }
    finally { if (alive.current) setBusy(false); }
  };
  return <div className="appearance-panel" data-page="appearance" aria-busy={busy}>
    {error && <p className="appearance-error" role="alert">{error}</p>}
    <div className="appearance-preview" style={{backgroundImage: backgroundURL(value) ? `linear-gradient(#07111c40,#07111cb0),url("${backgroundURL(value)}")` : undefined}}>
      <span className="appearance-preview-label">DUEL ATELIER <b>外观预览</b></span>
      <div className="appearance-mini-field" aria-hidden="true">{Array.from({length:10},(_,i)=><i key={i} />)}</div>
      <img src={sleeveURL(value)} alt="所选卡套预览" />
      <span className="appearance-preview-note">仅保存在本机 · 对局内即时生效</span>
    </div>
    <section className="appearance-section" aria-labelledby="sleeve-heading">
      <div className="appearance-section-head"><h2 id="sleeve-heading">我方卡套</h2><span>CARD SLEEVE</span></div>
      <div className="appearance-options appearance-sleeves">
        {SLEEVES.map(item => <button type="button" disabled={busy} className="appearance-option" key={item.id} aria-pressed={value.sleeve===item.id} onClick={()=>apply({sleeve:item.id})}>
          <img src={item.url} alt="" /><span>{item.name}</span>
        </button>)}
        {value.customSleeve && <button type="button" disabled={busy} className="appearance-option" aria-pressed={value.sleeve==='custom'} onClick={()=>apply({sleeve:'custom'})}><img src={value.customSleeve} alt="" /><span>自定义</span></button>}
      </div>
      <div className="appearance-upload"><label>导入卡套<input aria-label="导入卡套图片" type="file" accept="image/png,image/jpeg,image/webp" disabled={busy} onChange={e=>{void upload('sleeve',e.currentTarget.files?.[0]);e.currentTarget.value='';}} /></label>
        <span title={value.sleeveName}>{value.customSleeve ? value.sleeveName : 'PNG / JPG / WebP · 居中裁切'}</span>
        {value.customSleeve && <button type="button" disabled={busy} onClick={()=>apply({customSleeve:'',sleeveName:'',sleeve:value.sleeve==='custom'?'classic':value.sleeve})}>移除</button>}
      </div>
    </section>
    <section className="appearance-section" aria-labelledby="background-heading">
      <div className="appearance-section-head"><h2 id="background-heading">场地背景</h2><span>DUEL BACKGROUND</span></div>
      <div className="appearance-options appearance-backgrounds">
        {BACKGROUNDS.map(item=><button type="button" disabled={busy} className="appearance-option" key={item.id} aria-pressed={value.background===item.id} onClick={()=>apply({background:item.id})}>
          <i style={{backgroundImage:item.url?`linear-gradient(#0a132540,#0a132580),url("${item.url}")`:undefined}} /><span>{item.name}</span>
        </button>)}
        {value.customBackground && <button type="button" disabled={busy} className="appearance-option" aria-pressed={value.background==='custom'} onClick={()=>apply({background:'custom'})}><i style={{backgroundImage:`url("${value.customBackground}")`}} /><span>自定义</span></button>}
      </div>
      <div className="appearance-upload"><label>导入背景<input aria-label="导入场地背景图片" type="file" accept="image/png,image/jpeg,image/webp" disabled={busy} onChange={e=>{void upload('background',e.currentTarget.files?.[0]);e.currentTarget.value='';}} /></label>
        <span title={value.backgroundName}>{value.customBackground ? value.backgroundName : '推荐 16:9 · 图片只保存在本机'}</span>
        {value.customBackground && <button type="button" disabled={busy} onClick={()=>apply({customBackground:'',backgroundName:'',background:value.background==='custom'?'arena':value.background})}>移除</button>}
      </div>
    </section>
    <section className="appearance-section appearance-environment" aria-labelledby="environment-heading">
      <div className="appearance-section-head"><h2 id="environment-heading">场地环境</h2><span>FIELD EFFECT</span></div>
      <UiCheckbox label="显示场地魔法环境" checked={!!settings.draw_field_spell} onChange={e=>settingsStore.set('draw_field_spell',e.target.checked?1:0)} />
      <label className="appearance-opacity">环境浓度<input aria-label="场地环境浓度" type="range" min="20" max="85" value={value.environmentOpacity} disabled={!settings.draw_field_spell||busy} onChange={e=>apply({environmentOpacity:Number(e.target.value)})} /><output>{value.environmentOpacity}%</output></label>
      <p>背景铺在基础场地与外围。场地魔法叠加于盘面，双方不同环境分半场显示；缺图或环境离场时恢复所选背景。</p>
    </section>
    <div className="appearance-bottom"><span role="status">{busy?'正在处理图片…':'外观更改自动保存'}</span><button type="button" disabled={busy} onClick={()=>apply({...APPEARANCE_DEFAULTS})}>恢复默认外观</button></div>
  </div>;
}
