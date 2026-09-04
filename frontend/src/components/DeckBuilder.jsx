import React, { useEffect, useState } from 'react';
import { WailsBridge } from '../wails_bridge.js';

const EXTRA_TYPES = 0x40 | 0x2000 | 0x800000 | 0x4000000;

// Fetches a card's display name once and caches it locally.
function CardChip({ code, onInspect, onRemove }) {
  const [info, setInfo] = useState(null);
  useEffect(() => {
    let alive = true;
    WailsBridge.getCard(code).then((c) => { if (alive) setInfo(c); });
    return () => { alive = false; };
  }, [code]);

  return (
    <div
      className="select-card-item"
      style={{ background: '#0f172a', padding: '4px', height: '100px', cursor: 'pointer' }}
      onMouseEnter={() => info && onInspect && onInspect(info)}
      onClick={() => onRemove && onRemove()}
    >
      <div style={{ fontSize: '10px', fontWeight: 700, color: '#38bdf8', overflow: 'hidden', textOverflow: 'ellipsis' }}>
        {info ? info.name : code}
      </div>
    </div>
  );
}

export default function DeckBuilder({ onNavigate }) {
  const [deckList, setDeckList] = useState([]);
  const [currentDeck, setCurrentDeck] = useState({ name: '新卡组', main: [], extra: [], side: [] });
  const [deckName, setDeckName] = useState('新卡组');
  const [searchKeyword, setSearchKeyword] = useState('');
  const [typeFilter, setTypeFilter] = useState('0');
  const [searchResults, setSearchResults] = useState([]);
  const [inspected, setInspected] = useState(null);

  useEffect(() => {
    (async () => {
      const list = await WailsBridge.listDecks();
      setDeckList(list);
      if (list.length > 0) {
        const deck = await WailsBridge.loadDeck(list[0]);
        if (deck) { setCurrentDeck(deck); setDeckName(deck.name || list[0]); }
      }
    })();
  }, []);

  const loadDeck = async (name) => {
    const deck = await WailsBridge.loadDeck(name);
    if (deck) { setCurrentDeck(deck); setDeckName(deck.name || name); }
  };

  const saveDeck = async () => {
    const name = deckName.trim() || '未命名卡组';
    const deck = { ...currentDeck, name };
    setCurrentDeck(deck);
    setDeckName(name);
    await WailsBridge.saveDeck(deck);
    alert(`卡组「${name}」保存成功！`);
    const list = await WailsBridge.listDecks();
    setDeckList(list);
  };

  const newDeck = () => {
    setCurrentDeck({ name: '新卡组', main: [], extra: [], side: [] });
    setDeckName('新卡组');
  };

  const performSearch = async () => {
    const results = await WailsBridge.searchCards({ keyword: searchKeyword.trim(), type: parseInt(typeFilter, 10) || 0, limit: 40 });
    setSearchResults(results || []);
  };

  const addCardToDeck = (card) => {
    const isExtra = (card.type & EXTRA_TYPES) !== 0;
    const deck = { ...currentDeck, main: [...currentDeck.main], extra: [...currentDeck.extra], side: [...currentDeck.side] };
    if (isExtra) {
      if (deck.extra.length >= 15) { alert('额外卡组已满（最多 15 张）。'); return; }
      deck.extra.push(card.code);
    } else {
      if (deck.main.length >= 60) { alert('主卡组已满（最多 60 张）。'); return; }
      deck.main.push(card.code);
    }
    setCurrentDeck(deck);
  };

  const removeCardFromDeck = (section, index) => {
    const deck = { ...currentDeck, main: [...currentDeck.main], extra: [...currentDeck.extra], side: [...currentDeck.side] };
    deck[section].splice(index, 1);
    setCurrentDeck(deck);
  };

  const simulateSampleHand = () => {
    if (currentDeck.main.length < 5) { alert('主卡组至少需要 5 张卡才能测试起手。'); return; }
    const shuffled = [...currentDeck.main].sort(() => 0.5 - Math.random());
    alert('模拟起手 5 张：\n' + shuffled.slice(0, 5).join('\n'));
  };

  const renderSection = (codes, section) => (
    <div className="deck-grid" style={{ minHeight: section === 'main' ? '220px' : '80px' }}>
      {codes.map((code, i) => (
        <CardChip key={`${code}-${i}`} code={code} onInspect={setInspected} onRemove={() => removeCardFromDeck(section, i)} />
      ))}
    </div>
  );

  return (
    <div id="deck-screen" className="screen active">
      <div className="deck-header">
        <div style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
          <h2 style={{ color: 'var(--primary)' }}>卡组构筑</h2>
          <select className="form-select" style={{ width: '200px' }} value={deckName} onChange={(e) => loadDeck(e.target.value)}>
            {deckList.map((n) => <option key={n} value={n}>{n}</option>)}
          </select>
          <input className="form-input" style={{ width: '180px' }} value={deckName} onChange={(e) => setDeckName(e.target.value)} />
          <button className="btn btn-primary" onClick={saveDeck}>保存卡组</button>
          <button className="btn btn-secondary" onClick={newDeck}>新建卡组</button>
          <button className="btn btn-gold" onClick={simulateSampleHand}>测试起手 5 张</button>
        </div>
        <button className="btn btn-secondary" onClick={() => onNavigate('menu')}>← 返回主菜单</button>
      </div>

      <div className="deck-main-layout">
        <div className="card-inspector" style={{ position: 'static', width: '100%', height: '100%' }}>
          <div className="inspector-pic-box">
            <canvas id="deck-inspector-pic" width="220" height="230"></canvas>
          </div>
          <div className="inspector-details">
            <div className="inspector-name">{inspected ? inspected.name : '卡牌详情'}</div>
            <div className="inspector-desc">
              {inspected
                ? `${inspected.type & 0x1 ? '怪兽 ' : inspected.type & 0x2 ? '魔法 ' : inspected.type & 0x4 ? '陷阱 ' : ''}${inspected.attack !== undefined ? `攻击/${inspected.attack} 守备/${inspected.defense}` : ''}\n${inspected.desc || ''}`
                : '悬停任意卡牌查看详情。'}
            </div>
          </div>
        </div>

        <div className="deck-zones-container">
          <div>
            <div className="deck-section-title">
              <span>主卡组（40 - 60）</span>
              <span className="badge badge-attr">{currentDeck.main.length} / 60</span>
            </div>
            {renderSection(currentDeck.main, 'main')}
          </div>
          <div>
            <div className="deck-section-title">
              <span>额外卡组（0 - 15）</span>
              <span className="badge badge-type">{currentDeck.extra.length} / 15</span>
            </div>
            {renderSection(currentDeck.extra, 'extra')}
          </div>
          <div>
            <div className="deck-section-title">
              <span>副卡组（0 - 15）</span>
              <span className="badge badge-attr">{currentDeck.side.length} / 15</span>
            </div>
            {renderSection(currentDeck.side, 'side')}
          </div>
        </div>

        <div className="deck-search-panel">
          <h3 style={{ color: 'var(--primary)', fontSize: '16px' }}>卡牌搜索</h3>
          <input className="form-input" type="text" placeholder="搜索名称、效果、编号..." value={searchKeyword} onChange={(e) => setSearchKeyword(e.target.value)} onKeyDown={(e) => { if (e.key === 'Enter') performSearch(); }} />
          <select className="form-select" value={typeFilter} onChange={(e) => setTypeFilter(e.target.value)}>
            <option value="0">全部种类</option>
            <option value="1">怪兽</option>
            <option value="2">魔法</option>
            <option value="4">陷阱</option>
          </select>
          <button className="btn btn-primary" style={{ marginTop: '8px' }} onClick={performSearch}>搜索</button>

          <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '4px' }}>搜索结果（点击添加）：</div>
          <div className="search-results-grid">
            {searchResults.map((card, i) => (
              <div
                key={`${card.code}-${i}`}
                className="select-card-item"
                style={{ background: '#1e293b', padding: '6px', cursor: 'pointer' }}
                onMouseEnter={() => setInspected(card)}
                onClick={() => addCardToDeck(card)}
              >
                <div style={{ fontSize: '11px', fontWeight: 700, color: '#38bdf8', overflow: 'hidden', textOverflow: 'ellipsis' }}>{card.name}</div>
                <div style={{ fontSize: '10px', color: '#94a3b8', marginTop: '4px' }}>{card.attack !== undefined ? 'ATK/' + card.attack : ''}</div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
