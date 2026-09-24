import { useEffect, useMemo, useRef, useState } from 'react';
import { WailsBridge } from '../../wails_bridge.ts';
import { CardTile } from './CardTile.tsx';

/** Card announcement（MSG_ANNOUNCE_CARD）：候选网格 + 实时过滤 + 自由输入卡号。
 * 原版 ebANCard 输入实时过滤宣言候选（client_field.cpp:1535-1569
 * UpdateDeclarableList）：纯数字先按卡号精确匹配，否则按卡名包含匹配
 * （is_declarable 过滤在 Go 侧解码 candidates 时已完成）。候选数据走现有
 * 卡库查询通道（WailsBridge.searchCards 的 $卡名 语法），有候选清单时与
 * 清单求交集；无候选（decodable=false）时直接展示检索结果供点选，
 * 裸输卡号路径保留。 */
export function AnnounceCardModal({ title, candidates, respond }: {
  title: string;
  candidates: { code: number; name: string }[];
  respond: (code: number) => void;
}) {
  const [input, setInput] = useState('');
  const [search, setSearch] = useState('');
  const [results, setResults] = useState<{ code: number; name: string }[]>([]);
  const [searching, setSearching] = useState(false);
  const searchSeq = useRef(0);

  const candCodes = useMemo(() => new Set(candidates.map((c) => c.code)), [candidates]);
  const keyword = search.trim();

  useEffect(() => {
    if (!keyword) { setResults([]); setSearching(false); return; }
    const seq = ++searchSeq.current;
    setSearching(true);
    const timer = setTimeout(async () => {
      let found: { code: number; name: string }[] = [];
      try {
        // $ 前缀 = 仅卡名匹配（Go CardFilter 关键词语法，deck_con.cpp
        // search_multiple_keywords 的 $ 元素）；卡号匹配在下面本地合并
        const cards = await WailsBridge.searchCards({ keyword: `$${keyword}`, limit: 30 });
        found = ((cards || []) as any[])
          .map((c) => ({ code: c.code, name: c.name || String(c.code) }))
          .filter((c) => candCodes.size === 0 || candCodes.has(c.code));
      } catch { /* 检索失败时只留本地卡号匹配 */ }
      // 卡号包含匹配（原版 trycode 精确命中置顶；这里把候选中卡号片段
      // 命中的也补进来，精确命中排最前）
      const byCode = candidates.filter((c) => String(c.code).includes(keyword));
      const merged = [...byCode, ...found.filter((c) => !byCode.some((b) => b.code === c.code))];
      // 卡名精确匹配置顶（client_field.cpp:1562 insertItem(0)）
      merged.sort((a, b) => Number(b.name === keyword) - Number(a.name === keyword));
      if (searchSeq.current === seq) {
        setResults(merged);
        setSearching(false);
      }
    }, 150);
    return () => clearTimeout(timer);
  }, [keyword, candidates, candCodes]);

  const confirmInput = () => {
    const v = parseInt(input, 10);
    if (!Number.isNaN(v) && v > 0) respond(v);
  };

  const shown = keyword ? results : candidates;

  return (
    <div className="modal-box modal-announce-card">
      <div className="modal-title">{title}</div>
      <div className="announce-card-row">
        <input
          id="announce-card-filter"
          type="text"
          placeholder="按卡名/卡号过滤候选"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
      </div>
      {shown.length > 0 && (
        <div className="modal-cards-grid">
          {shown.map(({ code, name }, idx) => (
            <CardTile key={`${code}-${idx}`} code={code} name={name} className="cand-card" dataAttrs={{ 'data-idx': idx }} onPick={() => respond(code)} />
          ))}
        </div>
      )}
      {keyword && !searching && shown.length === 0 && (
        <div id="announce-card-empty" style={{ fontSize: 12, color: 'var(--text-muted)', margin: '4px 0' }}>
          没有匹配的候选卡。
        </div>
      )}
      <div className="announce-card-row">
        <input
          id="announce-card-input"
          type="number"
          min={0}
          placeholder="输入卡片密码"
          value={input}
          onChange={(e) => setInput(e.target.value)}
        />
        <button id="announce-card-confirm" className="btn btn-gold" onClick={confirmInput}>确定</button>
      </div>
    </div>
  );
}
