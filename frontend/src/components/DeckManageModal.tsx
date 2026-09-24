import React, { useMemo, useState } from 'react';
import { WailsBridge } from '../wails_bridge.ts';
import GfwSelect from './ui/GfwSelect.tsx';

// 卡组/分类管理窗口（原版 wDeckManage，game.cpp:660-683 + deck_con.cpp:423-676）。
// 左分类列表 / 右该分类下的卡组列表；分类：新建/重命名/删除；卡组：新建/
// 重命名/删除/复制到分类/移动到分类。文件操作全走 Go 侧 deck_manage.go 绑定，
// 成功后由父组件刷新卡组清单。
export default function DeckManageModal({ deckList, onClose, onRefresh }: {
  deckList: string[];
  onClose: () => void;
  onRefresh: (list: string[]) => void;
}) {
  const [selCategory, setSelCategory] = useState(''); // '' = 未分类（根目录）
  const [selDeck, setSelDeck] = useState(''); // 完整相对名（含分类前缀）
  const [nameInput, setNameInput] = useState('');
  const [targetCategory, setTargetCategory] = useState('');
  const [busy, setBusy] = useState(false);

  const categories = useMemo(
    () => Array.from(new Set(deckList.filter((n) => n.includes('/')).map((n) => n.split('/')[0]))),
    [deckList],
  );
  const decksInCategory = useMemo(
    () => deckList.filter((n) => (selCategory === '' ? !n.includes('/') : n.startsWith(selCategory + '/'))),
    [deckList, selCategory],
  );

  const refresh = async (): Promise<void> => {
    const list = (await WailsBridge.listDecks()) || [];
    onRefresh(list);
  };

  // 统一执行：Go 侧绑定失败会 reject，弹出错误后中断；成功则刷新清单
  const run = async (op: () => Promise<unknown>): Promise<void> => {
    if (busy) return;
    setBusy(true);
    try {
      await op();
      await refresh();
    } catch (e: any) {
      alert((e && e.message) || String(e));
    } finally {
      setBusy(false);
    }
  };

  const newCategory = (): Promise<void> => run(async () => {
    const name = nameInput.trim();
    if (!name) { alert('请输入分类名。'); return; }
    await WailsBridge.createDeckCategory(name);
    setSelCategory(name);
    setSelDeck('');
  });

  const renameCategory = (): Promise<void> => run(async () => {
    if (!selCategory) { alert('根目录（未分类）不能重命名。'); return; }
    const name = nameInput.trim();
    if (!name) { alert('请输入新分类名。'); return; }
    await WailsBridge.renameDeckCategory(selCategory, name);
    setSelCategory(name);
    setSelDeck('');
  });

  const deleteCategory = (): Promise<void> => run(async () => {
    if (!selCategory) { alert('根目录（未分类）不能删除。'); return; }
    if (!window.confirm(`确定删除分类「${selCategory}」及其中全部卡组？此操作不可撤销。`)) return;
    await WailsBridge.deleteDeckCategory(selCategory);
    setSelCategory('');
    setSelDeck('');
  });

  const newDeck = (): Promise<void> => run(async () => {
    const base = nameInput.trim();
    if (!base) { alert('请输入卡组名。'); return; }
    const name = selCategory ? `${selCategory}/${base}` : base;
    if (deckList.includes(name)) { alert(`卡组「${name}」已存在。`); return; }
    await WailsBridge.saveDeck({ name, main: [], extra: [], side: [] });
    setSelDeck(name);
  });

  const renameDeck = (): Promise<void> => run(async () => {
    if (!selDeck) { alert('请先选择卡组。'); return; }
    const base = nameInput.trim();
    if (!base) { alert('请输入新卡组名。'); return; }
    await WailsBridge.renameDeck(selDeck, base);
    const cat = selDeck.includes('/') ? selDeck.split('/')[0] : '';
    setSelDeck(cat ? `${cat}/${base}` : base);
  });

  const deleteDeck = (): Promise<void> => run(async () => {
    if (!selDeck) { alert('请先选择卡组。'); return; }
    if (!window.confirm(`确定删除卡组「${selDeck}」？此操作不可撤销。`)) return;
    await WailsBridge.deleteDeck(selDeck);
    setSelDeck('');
  });

  const copyDeck = (): Promise<void> => run(async () => {
    if (!selDeck) { alert('请先选择卡组。'); return; }
    await WailsBridge.copyDeck(selDeck, targetCategory);
  });

  const moveDeck = (): Promise<void> => run(async () => {
    if (!selDeck) { alert('请先选择卡组。'); return; }
    await WailsBridge.moveDeck(selDeck, targetCategory);
    if (selCategory !== targetCategory) setSelDeck('');
  });

  return (
    <div
      id="deck-manage-overlay"
      style={{
        position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.7)',
        display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 2200,
      }}
      onClick={onClose}
    >
      <div
        id="deck-manage-modal"
        className="modal-box"
        style={{ width: 640, maxWidth: '92vw', maxHeight: '84vh', overflowY: 'auto' }}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="modal-title">卡组管理</div>
        <div style={{ display: 'flex', gap: 8, margin: '8px 0' }}>
          {/* 左：分类列表（原版 lstCategories） */}
          <div style={{ width: 180, flexShrink: 0 }}>
            <div className="gfw-label">分类</div>
            <div id="dm-category-list" style={{
              border: '1px solid var(--primary)', borderRadius: 6, minHeight: 220,
              maxHeight: 300, overflowY: 'auto', padding: 4,
            }}>
              {['', ...categories].map((c) => (
                <div
                  key={c || '__root__'}
                  className={`dm-list-item${selCategory === c ? ' selected' : ''}`}
                  style={{
                    padding: '3px 6px', cursor: 'pointer', borderRadius: 4, fontSize: 12,
                    background: selCategory === c ? 'rgba(56,189,248,0.25)' : 'transparent',
                  }}
                  onClick={() => { setSelCategory(c); setSelDeck(''); }}
                >
                  {c || '未分类卡组'}
                </div>
              ))}
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 4, marginTop: 6 }}>
              <button id="dm-new-category" className="btn btn-secondary" disabled={busy} onClick={newCategory}>新建分类</button>
              <button id="dm-rename-category" className="btn btn-secondary" disabled={busy || !selCategory} onClick={renameCategory}>重命名分类</button>
              <button id="dm-delete-category" className="btn btn-danger" disabled={busy || !selCategory} onClick={deleteCategory}>删除分类</button>
            </div>
          </div>
          {/* 右：卡组列表（原版 lstDecks） */}
          <div style={{ flex: 1 }}>
            <div className="gfw-label">卡组</div>
            <div id="dm-deck-list" style={{
              border: '1px solid var(--primary)', borderRadius: 6, minHeight: 220,
              maxHeight: 300, overflowY: 'auto', padding: 4,
            }}>
              {decksInCategory.length === 0 ? (
                <div style={{ fontSize: 12, color: 'var(--text-muted)', padding: 6 }}>该分类下没有卡组。</div>
              ) : decksInCategory.map((n) => (
                <div
                  key={n}
                  className={`dm-list-item${selDeck === n ? ' selected' : ''}`}
                  style={{
                    padding: '3px 6px', cursor: 'pointer', borderRadius: 4, fontSize: 12,
                    background: selDeck === n ? 'rgba(56,189,248,0.25)' : 'transparent',
                  }}
                  onClick={() => setSelDeck(n)}
                >
                  {n.split('/').pop()}
                </div>
              ))}
            </div>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4, marginTop: 6 }}>
              <button id="dm-new-deck" className="btn btn-secondary" disabled={busy} onClick={newDeck}>新建卡组</button>
              <button id="dm-rename-deck" className="btn btn-secondary" disabled={busy || !selDeck} onClick={renameDeck}>重命名</button>
              <button id="dm-delete-deck" className="btn btn-danger" disabled={busy || !selDeck} onClick={deleteDeck}>删除</button>
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginTop: 6 }}>
              {/* 目标分类（原版 cbDMCategory；__root__ 哨兵 = 未分类） */}
              <GfwSelect
                id="dm-target-category"
                width={150}
                value={targetCategory || '__root__'}
                onValueChange={(v) => setTargetCategory(v === '__root__' ? '' : v)}
                options={[
                  { value: '__root__', label: '未分类卡组' },
                  ...categories.map((c) => ({ value: c, label: c })),
                ]}
              />
              <button id="dm-copy-deck" className="btn btn-secondary" disabled={busy || !selDeck} onClick={copyDeck}>复制到分类</button>
              <button id="dm-move-deck" className="btn btn-secondary" disabled={busy || !selDeck} onClick={moveDeck}>移动到分类</button>
            </div>
          </div>
        </div>
        {/* 名称输入（原版 wDMQuery 的 ebDMName）：新建/重命名共用 */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 8 }}>
          <label className="gfw-label">名称</label>
          <input
            id="dm-name-input"
            className="form-input"
            style={{ flex: 1 }}
            placeholder="新建/重命名用的名称"
            value={nameInput}
            onChange={(e) => setNameInput(e.target.value)}
          />
        </div>
        <div style={{ textAlign: 'right' }}>
          <button id="dm-close" className="btn btn-primary" onClick={onClose}>关闭</button>
        </div>
      </div>
    </div>
  );
}
