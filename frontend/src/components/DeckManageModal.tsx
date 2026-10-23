import React, { useMemo, useState, useRef, useEffect } from 'react';
import { WailsBridge } from '../wails_bridge.ts';
import GfwSelect from './ui/GfwSelect.tsx';
import UiButton from '../ui/UiButton.tsx';
import { UiInput } from '../ui/UiInput.tsx';
import DesignSpace from '../ui/DesignSpace.tsx';
import SourceWindow from '../ui/SourceWindow.tsx';

// 卡组/分类管理窗口（原版 wDeckManage，game.cpp:660-683 + deck_con.cpp:423-676）。
// 左分类列表 / 右该分类下的卡组列表；分类：新建/重命名/删除；卡组：新建/
// 重命名/删除/复制到分类/移动到分类。文件操作全走 Go 侧 deck_manage.go 绑定，
// 成功后由父组件刷新卡组清单。
export default function DeckManageModal({ deckList, onClose, onRefresh, onImport, onExport, categoryList = [], onSelectDeck }: {
  deckList: string[];
  onClose: () => void;
  onRefresh: (list: string[]) => void;
  onImport?: () => void;
  onExport?: () => void;
  categoryList?: string[];
  onSelectDeck?: (name:string)=>void;
}) {
  const [selCategory, setSelCategory] = useState(''); // '' = 未分类（根目录）
  const [selDeck, setSelDeck] = useState(''); // 完整相对名（含分类前缀）
  const [nameInput, setNameInput] = useState('');
  const [targetCategory, setTargetCategory] = useState('');
  const [busy, setBusy] = useState(false);
  const [query,setQuery]=useState<{message:string;kind:'name'|'category'|'confirm'}|null>(null);
  const queryResolve=useRef<((value:string|null)=>void)|null>(null);
  useEffect(()=>()=>{queryResolve.current?.(null);queryResolve.current=null;},[]);
  const ask=(message:string,kind:'name'|'category'|'confirm',initial=''):Promise<string|null>=>{
    setNameInput(initial);setTargetCategory(selCategory);setQuery({message,kind});
    return new Promise(resolve=>{queryResolve.current=resolve;});
  };
  const answer=(value:string|null)=>{setQuery(null);queryResolve.current?.(value);queryResolve.current=null;};

  const categories = useMemo(
    () => Array.from(new Set([...categoryList,...deckList.filter((n) => n.includes('/')).map((n) => n.split('/')[0])])),
    [deckList,categoryList],
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
    const name = await ask('请输入新分类名。','name');
    if (!name) return;
    await WailsBridge.createDeckCategory(name);
    setSelCategory(name);
    setSelDeck('');
  });

  const renameCategory = (): Promise<void> => run(async () => {
    if (!selCategory) { alert('根目录（未分类）不能重命名。'); return; }
    const name = await ask('请输入新分类名。','name',selCategory);
    if (!name) return;
    await WailsBridge.renameDeckCategory(selCategory, name);
    setSelCategory(name);
    setSelDeck('');
  });

  const deleteCategory = (): Promise<void> => run(async () => {
    if (!selCategory) { alert('根目录（未分类）不能删除。'); return; }
    if (!await ask(`确定删除分类「${selCategory}」？`,'confirm')) return;
    await WailsBridge.deleteDeckCategory(selCategory);
    setSelCategory('');
    setSelDeck('');
  });

  const newDeck = (): Promise<void> => run(async () => {
    const base = await ask('请输入新卡组名。','name');
    if (!base) return;
    const name = selCategory ? `${selCategory}/${base}` : base;
    if (deckList.includes(name)) { alert(`卡组「${name}」已存在。`); return; }
    await WailsBridge.saveDeck({ name, main: [], extra: [], side: [] });
    setSelDeck(name);
    onSelectDeck?.(name);
  });

  const renameDeck = (): Promise<void> => run(async () => {
    if (!selDeck) { alert('请先选择卡组。'); return; }
    const base = await ask('请输入新卡组名。','name',selDeck.split('/').pop());
    if (!base) return;
    await WailsBridge.renameDeck(selDeck, base);
    const cat = selDeck.includes('/') ? selDeck.split('/')[0] : '';
    setSelDeck(cat ? `${cat}/${base}` : base);
  });

  const deleteDeck = (): Promise<void> => run(async () => {
    if (!selDeck) { alert('请先选择卡组。'); return; }
    if (!await ask(`确定删除卡组「${selDeck}」？`,'confirm')) return;
    await WailsBridge.deleteDeck(selDeck);
    setSelDeck('');
  });

  const copyDeck = (): Promise<void> => run(async () => {
    if (!selDeck) { alert('请先选择卡组。'); return; }
    const target=await ask('请选择目标分类。','category');
    if(target===null)return;
    await WailsBridge.copyDeck(selDeck, target);
  });

  const moveDeck = (): Promise<void> => run(async () => {
    if (!selDeck) { alert('请先选择卡组。'); return; }
    const target=await ask('请选择目标分类。','category');
    if(target===null)return;
    await WailsBridge.moveDeck(selDeck, target);
    if (selCategory !== target) setSelDeck('');
  });

  return (
    <div
      id="deck-manage-overlay"
      style={{
        position: 'fixed', inset: 0, background: 'transparent', zIndex: 2200,
      }}
    >
      {/* 原版 wDeckManage（game.cpp:660-683）：(310,135)-(800,515) 490×380，
          lstCategories (10,30) 130×340 / lstDecks (150,30) 190×340 /
          按钮列 x350-480 宽 130、y30 起 35 距 25 高。名称输入/目标分类/关闭是
          扩展件（原版走 wDMQuery 弹窗），压在底部扩展行（列表收短到 340）。 */}
      <DesignSpace>
      <SourceWindow x={310} y={135} width={490} height={380}
        id="deck-manage-modal"
        className="modal-box"
        style={{
          position: 'absolute', left: '310px', top: '135px', width: '490px', height: '380px',
          minWidth: 0, maxWidth: 'none', maxHeight: 'none', margin: 0, padding: '0 10px 8px',
          boxSizing: 'border-box', overflow: 'hidden', gap: '4px',
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="modal-title">卡组管理<UiButton id="dm-close" disabled={busy} style={{position:'absolute',right:4,top:2,width:20,height:20,padding:0}} onClick={onClose}>×</UiButton></div>
        <div style={{ position: 'relative', flex: 1, minHeight: 0 }}>
          {/* 左：分类列表（原版 lstCategories (10,30)-(140,370)） */}
          <div style={{ position: 'absolute', left: 0, top: '6px', width: '130px' }}>
            <div id="dm-category-list" style={{
              border: '1px solid rgba(148,163,184,0.18)', borderRadius: 8, height: '340px', overflowY: 'auto',
              padding: 5, background: 'rgba(13,19,29,0.6)',
            }}>
              {['', ...categories].map((c) => (
                <div
                  key={c || '__root__'}
                  className={`dm-list-item${selCategory === c ? ' selected' : ''}`}
                  style={{
                    padding: '3px 8px', cursor: 'pointer', borderRadius: 6, fontSize: 12,
                    background: selCategory === c ? 'rgba(59,130,246,0.22)' : 'transparent',
                    color: selCategory === c ? '#bfdbfe' : 'var(--text-main)',
                  }}
                  onClick={() => { setSelCategory(c); setSelDeck(''); }}
                >
                  {c || '未分类卡组'}
                </div>
              ))}
            </div>
          </div>
          {/* 中：卡组列表（原版 lstDecks (150,30)-(340,370)） */}
          <div style={{ position: 'absolute', left: '140px', top: '6px', width: '190px' }}>
            <div id="dm-deck-list" style={{
              border: '1px solid rgba(148,163,184,0.18)', borderRadius: 8, height: '340px', overflowY: 'auto',
              padding: 5, background: 'rgba(13,19,29,0.6)',
            }}>
              {decksInCategory.length === 0 ? (
                <div style={{ fontSize: 12, color: 'var(--text-muted)', padding: 6 }}>该分类下没有卡组。</div>
              ) : decksInCategory.map((n) => (
                <div
                  key={n}
                  className={`dm-list-item${selDeck === n ? ' selected' : ''}`}
                  style={{
                    padding: '3px 8px', cursor: 'pointer', borderRadius: 6, fontSize: 12,
                    background: selDeck === n ? 'rgba(59,130,246,0.22)' : 'transparent',
                    color: selDeck === n ? '#bfdbfe' : 'var(--text-main)',
                  }}
                  onClick={() => {setSelDeck(n);onSelectDeck?.(n);}}
                >
                  {n.split('/').pop()}
                </div>
              ))}
            </div>
          </div>
          {/* 右：按钮列（原版 x350-480，y30 起 35 距；顺序 新建/重命名/删除分类 →
              新建/重命名/删除卡组 → 移动 → 复制） */}
          <div style={{ position: 'absolute', left: '340px', top: '6px', width: '130px', display: 'flex', flexDirection: 'column', gap: '6px' }}>
            <UiButton id="dm-new-category" disabled={busy} onClick={newCategory}>新建分类</UiButton>
            <UiButton id="dm-rename-category" disabled={busy || !selCategory} onClick={renameCategory}>重命名分类</UiButton>
            <UiButton id="dm-delete-category" variant="danger" disabled={busy || !selCategory} onClick={deleteCategory}>删除分类</UiButton>
            <UiButton id="dm-new-deck" disabled={busy} onClick={newDeck}>新建卡组</UiButton>
            <UiButton id="dm-rename-deck" disabled={busy || !selDeck} onClick={renameDeck}>重命名</UiButton>
            <UiButton id="dm-delete-deck" variant="danger" disabled={busy || !selDeck} onClick={deleteDeck}>删除</UiButton>
            <UiButton id="dm-move-deck" disabled={busy || !selDeck} onClick={moveDeck}>移动到分类</UiButton>
            <UiButton id="dm-copy-deck" disabled={busy || !selDeck} onClick={copyDeck}>复制到分类</UiButton>
            <UiButton id="deck-import-code-btn" onClick={onImport}>导入卡组码</UiButton>
            <UiButton id="deck-export-code-btn" onClick={onExport}>导出卡组码</UiButton>
          </div>
        </div>
      </SourceWindow>
      {query&&<SourceWindow x={400} y={200} width={310} height={120} id="dm-query" className="gfw-window" style={{zIndex:3000}}>
        <div className="gfw-title">卡组管理</div>
        <div style={{position:'relative',height:96}}>
          <div style={{position:'absolute',left:20,top:6,width:270,textAlign:'center'}}>{query.message}</div>
          {query.kind==='name'&&
            <UiInput
              id="dm-name-input"
              style={{position:'absolute',left:30,top:26,width:250,height:20}}
              placeholder="新建/重命名用的名称"
              value={nameInput}
              onChange={(e) => setNameInput(e.target.value)}
            />}
            {/* 目标分类（原版 cbDMCategory；__root__ 哨兵 = 未分类） */}
            {query.kind==='category'&&<div style={{position:'absolute',left:30,top:26}}><GfwSelect
              id="dm-target-category"
              width={250}
              value={targetCategory || '__root__'}
              onValueChange={(v) => setTargetCategory(v === '__root__' ? '' : v)}
              options={[
                { value: '__root__', label: '未分类卡组' },
                ...categories.map((c) => ({ value: c, label: c })),
              ]}
            /></div>}
            <UiButton id="dm-query-ok" style={{position:'absolute',left:60,top:56,width:80,height:25}} onClick={()=>{const value=query.kind==='name'?nameInput.trim():query.kind==='category'?targetCategory:'yes';if(query.kind!=='name'||value)answer(value);}}>确定</UiButton>
            <UiButton id="dm-query-cancel" style={{position:'absolute',left:170,top:56,width:80,height:25}} onClick={()=>answer(null)}>取消</UiButton>
        </div>
      </SourceWindow>}
      </DesignSpace>
    </div>
  );
}
