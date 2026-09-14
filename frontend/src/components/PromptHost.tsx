/**
 * PromptHost — 全部引擎询问弹窗的唯一宿主（原版 wHand/wPosSelect/wCardSelect/
 * wANNumber/wANCard/wOptions 等的 React 化）。
 *
 * 事件→弹窗→应答闭环都在这里：订阅 eventBus 的 select_* / announce_* / rps
 * 等事件，
 * 解析卡名（异步 getCard）后渲染单槽弹窗（新弹窗顶掉旧弹窗），用户点击后
 * 调 WailsBridge 的语义化 Respond* 方法（字节编码在 Go responses.go）。
 *
 * duel_manager 不再处理任何弹窗——连锁三键的「ignore 自动代答」也收编在
 * select_chain 处理器里（gframe duelclient.cpp:1776 的本地代答语义）。
 * 回放剧场不挂载本组件（DuelStage interactive=false）。
 *
 * 冒烟契约：所有 id / class 与旧 hud.js 逐字一致（#modal-btn-yes、
 * .modal-overlay.active、.pos-opt、.rps-hand-btn、#counter-confirm、
 * .btn-flash-gold……），断言可平移。
 */
import React, { useEffect, useRef, useState } from 'react';
import { eventBus, WailsBridge } from '../wails_bridge.ts';
import chainPrefs from '../duel/chain_prefs.ts';
import {
  POS_FACEUP_ATTACK, POS_FACEDOWN_ATTACK, POS_FACEUP_DEFENSE, POS_FACEDOWN_DEFENSE,
  RACES, ATTRS,
} from '../domain/constants.ts';
import GameDialog from './GameDialog.tsx';

// ---- 公共小件 ----

/** 卡牌图块：cover 兜底 + 异步真实卡图（旧 fillCardTileImages 的 React 版） */
function CardTile({ code, name, className = '', children, onPick, dataAttrs }: {
  code: number;
  name: string;
  className?: string;
  children?: React.ReactNode;
  onPick?: () => void;
  dataAttrs?: Record<string, string | number>;
}) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    let alive = true;
    if (!code) return undefined;
    WailsBridge.getCardImage(code).then((pic) => {
      if (!alive || !pic || !pic.url || !ref.current) return;
      ref.current.style.backgroundImage = `url("${pic.url}")`;
      ref.current.classList.add('card-tile-loaded');
    }).catch(() => {});
    return () => { alive = false; };
  }, [code]);

  return (
    <div
      ref={ref}
      className={`select-card-item card-tile ${className}`}
      data-code={code}
      {...(dataAttrs || {})}
      onClick={onPick}
    >
      {children}
      <div className="card-tile-name">{name}</div>
    </div>
  );
}

function YesNoModal({ title, message, respond }: { title: string; message: string; respond: (yes: boolean) => void }) {
  return (
    <div className="modal-box">
      <div className="modal-title">{title}</div>
      <p style={{ color: 'var(--text-main)', fontSize: '14px', lineHeight: 1.5 }}>{message}</p>
      <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '12px', marginTop: '10px' }}>
        <button id="modal-btn-no" className="btn btn-secondary" onClick={() => respond(false)}>否</button>
        <button id="modal-btn-yes" className="btn btn-primary" onClick={() => respond(true)}>是</button>
      </div>
    </div>
  );
}

interface SelectCard {
  code: number;
  name: string;
  [k: string]: unknown;
}

function CardSelectModal({ title, cards, min, max, cancelable, respond }: {
  title: string;
  cards: SelectCard[];
  min: number;
  max: number;
  cancelable: boolean;
  respond: (result: { indices: number[] | null }) => void;
}) {
  const [selected, setSelected] = useState<number[]>([]);
  const done = (indices: number[] | null) => respond({ indices });

  return (
    <div className="modal-box" style={{ minWidth: '550px' }}>
      <div className="modal-title">{`${title}（选择 ${min}-${max} 张）`}</div>
      <div className="modal-cards-grid">
        {cards.map((c, idx) => (
          <CardTile
            key={idx}
            code={c.code}
            name={c.name}
            dataAttrs={{ 'data-idx': idx }}
            className={selected.includes(idx) ? 'selected' : ''}
            onPick={() => {
              setSelected((cur) => {
                if (cur.includes(idx)) return cur.filter((i) => i !== idx);
                if (cur.length >= max) return cur;
                return [...cur, idx];
              });
            }}
          />
        ))}
      </div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginTop: '12px' }}>
        <span id="select-count-text" style={{ fontSize: '13px', color: 'var(--text-muted)' }}>
          {`已选 ${selected.length} / ${max} 张`}
        </span>
        <div style={{ display: 'flex', gap: '8px' }}>
          {cancelable && (
            <button id="modal-select-cancel" className="btn btn-secondary" onClick={() => done(null)}>取消</button>
          )}
          <button
            id="modal-select-confirm"
            className={`btn btn-gold${selected.length >= max ? ' btn-flash-gold' : ''}`}
            disabled={selected.length < min}
            onClick={() => done(selected)}
          >确认</button>
        </div>
      </div>
    </div>
  );
}

/** 表示形式选择（原版 wPosSelect，game.cpp:567-575）：137×137 方形按钮，
 *  表侧真实卡图（守备旋转 90°），里侧卡背（gframe duelclient.cpp:1920-1931） */
function PositionModal({ positions, code, respond }: { positions: number; code: number; respond: (pos: number) => void }) {
  const [pic, setPic] = useState<string | null>(null);
  useEffect(() => {
    let alive = true;
    if (!code) return undefined;
    WailsBridge.getCardImage(code).then((p) => {
      if (alive && p && p.url) setPic(p.url);
    }).catch(() => {});
    return () => { alive = false; };
  }, [code]);

  const options: [number, string, boolean, boolean][] = [];
  if (positions & POS_FACEUP_ATTACK) options.push([POS_FACEUP_ATTACK, '表侧攻击表示', false, false]);
  if (positions & POS_FACEDOWN_ATTACK) options.push([POS_FACEDOWN_ATTACK, '里侧攻击表示', true, false]);
  if (positions & POS_FACEUP_DEFENSE) options.push([POS_FACEUP_DEFENSE, '表侧守备表示', false, true]);
  if (positions & POS_FACEDOWN_DEFENSE) options.push([POS_FACEDOWN_DEFENSE, '里侧守备表示', true, true]);
  if (!options.length) {
    options.push([POS_FACEUP_ATTACK, '表侧攻击表示', false, false], [POS_FACEDOWN_DEFENSE, '里侧守备表示', true, true]);
  }

  return (
    <div className="modal-box modal-pos">
      <div className="modal-title">选择表示形式</div>
      <div className="pos-row">
        {options.map(([pos, label, facedown, rotated], i) => (
          <button
            key={pos}
            data-pos={pos}
            className="pos-opt"
            id={i === 0 ? 'pos-first-btn' : undefined}
            title={label}
            onClick={() => respond(pos)}
          >
            <span
              className={`pos-opt-img${facedown ? ' pos-opt-facedown' : ''}${rotated ? ' pos-opt-rot' : ''}`}
              style={pic && !facedown ? { backgroundImage: `url("${pic}")` } : undefined}
            />
          </button>
        ))}
      </div>
    </div>
  );
}

/** 猜拳（原版 wHand）：f1=石头→1、f2=剪刀→2、f3=布→3（gframe
 *  event_handler.cpp:33；引擎判定 1 胜 2、2 胜 3、3 胜 1 operations.cpp:6538） */
function RpsModal({ respond }: { respond: (choice: number) => void }) {
  return (
    <div className="modal-box modal-rps">
      <div className="modal-title">猜拳</div>
      <div className="rps-row">
        <button id="rps-rock" className="rps-hand-btn rps-hand-rock" title="石头" onClick={() => respond(1)} />
        <button id="rps-scissors" className="rps-hand-btn rps-hand-scissors" title="剪刀" onClick={() => respond(2)} />
        <button id="rps-paper" className="rps-hand-btn rps-hand-paper" title="布" onClick={() => respond(3)} />
      </div>
    </div>
  );
}

/** Counter removal（MSG_SELECT_COUNTER）：每卡一个 uint16，合计恰为 count */
function CounterModal({ title, cards, count, respond }: {
  title: string;
  cards: { code: number; name: string; cnt: number }[];
  count: number;
  respond: (counts: number[]) => void;
}) {
  const [counts, setCounts] = useState<number[]>(() => cards.map(() => 0));
  const maxTotal = cards.reduce((s, c) => s + (c.cnt || 0), 0);
  const total = counts.reduce((s, v) => s + v, 0);

  return (
    <div className="modal-box" style={{ minWidth: '500px' }}>
      <div className="modal-title">{`${title}（需移除 ${count} 个指示物）`}</div>
      <div>
        {cards.map((c, i) => (
          <div key={i} className="select-card-item counter-row" data-idx={i}
            style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', background: '#1e293b', padding: '8px' }}>
            <div style={{ fontSize: '12px', color: '#38bdf8', fontWeight: 700 }}>{c.name}</div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <span style={{ fontSize: '11px', color: 'var(--text-muted)' }}>{`可移 ${c.cnt || 0}`}</span>
              <button className="btn btn-secondary counter-dec" data-idx={i} style={{ padding: '2px 10px' }}
                onClick={() => setCounts((cur) => cur.map((v, j) => (j === i && v > 0 ? v - 1 : v)))}>-</button>
              <span className="counter-val" data-idx={i} style={{ minWidth: '24px', textAlign: 'center', fontWeight: 900 }}>{counts[i]}</span>
              <button className="btn btn-secondary counter-inc" data-idx={i} style={{ padding: '2px 10px' }}
                onClick={() => setCounts((cur) => cur.map((v, j) => (j === i && v < (c.cnt || 0) ? v + 1 : v)))}>+</button>
            </div>
          </div>
        ))}
      </div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginTop: '12px' }}>
        <span id="counter-total" style={{ fontSize: '13px', color: 'var(--text-muted)' }}>
          {`已选 ${total} / ${count}（总供给 ${maxTotal}）`}
        </span>
        <button id="counter-confirm" className="btn btn-gold" disabled={total !== count} onClick={() => respond(counts)}>确认</button>
      </div>
    </div>
  );
}

/** sum-limited selection（MSG_SELECT_SUM）。sumMode 0 = 恰好 acc（超量/链接），
 *  1 = 至少 acc（上级召唤/仪式）；must 卡已锁定只展示 */
function SumSelectModal({ title, cards, must, acc, min, max, sumMode, respond }: {
  title: string;
  cards: SelectCard[];
  must: SelectCard[];
  acc: number;
  min: number;
  max: number;
  sumMode: number;
  respond: (selected: number[]) => void;
}) {
  const [selected, setSelected] = useState<number[]>([]);
  const cardVal = (c: SelectCard): number => {
    // sum_param 打包两个备选值；一张卡按其中较小者计
    const o1 = (Number(c.param) || 0) & 0xffff;
    const o2 = (Number(c.param) || 0) >>> 16;
    return o2 && o2 < o1 ? o2 : o1;
  };
  const mustSum = must.reduce((s, c) => s + cardVal(c), 0);
  const sum = mustSum + selected.reduce((s, i) => s + cardVal(cards[i]), 0);
  const countOk = selected.length >= min && selected.length <= max;
  const sumOk = sumMode === 0 ? sum === acc : sum >= acc;

  return (
    <div className="modal-box" style={{ minWidth: '560px' }}>
      <div className="modal-title">{title}</div>
      <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginBottom: '8px' }}>
        {`需要 ${sumMode === 0 ? '合计恰好' : '合计至少'} ${acc}${mustSum ? `（必选已占 ${mustSum}）` : ''}，选择 ${min}-${max} 张`}
      </div>
      {must.length > 0 && (
        <div style={{ marginBottom: '8px' }}>
          {must.map((c, i) => (
            <div key={i} style={{ background: '#0f172a', padding: '6px', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <div style={{ fontSize: '11px', color: 'var(--text-muted)' }}>{`🔒 ${c.name}`}</div>
              <span style={{ fontSize: '11px', color: '#f59e0b' }}>{cardVal(c)}</span>
            </div>
          ))}
        </div>
      )}
      <div className="modal-cards-grid">
        {cards.map((c, idx) => (
          <CardTile
            key={idx}
            code={c.code}
            name={c.name}
            className={`sum-card${selected.includes(idx) ? ' selected' : ''}`}
            dataAttrs={{ 'data-idx': idx }}
            onPick={() => {
              setSelected((cur) => {
                if (cur.includes(idx)) return cur.filter((i) => i !== idx);
                if (cur.length >= max) return cur;
                return [...cur, idx];
              });
            }}
          >
            <span className="sum-param">{cardVal(c)}</span>
          </CardTile>
        ))}
      </div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginTop: '12px' }}>
        <span id="sum-status" style={{ fontSize: '13px', color: sumOk && countOk ? '#22c55e' : 'var(--text-muted)' }}>
          {`已选 ${selected.length} 张，合计 ${sum}`}
        </span>
        <button
          id="sum-confirm"
          className={`btn btn-gold${sumOk && countOk && selected.length >= max ? ' btn-flash-gold' : ''}`}
          disabled={!(sumOk && countOk)}
          onClick={() => respond(selected)}
        >确认</button>
      </div>
    </div>
  );
}

/** Card ordering（MSG_SORT_CARD）：按顺序点击；确认回排序置换字节 */
function SortModal({ title, cards, respond }: {
  title: string;
  cards: SelectCard[];
  respond: (result: { order: number[] | null }) => void;
}) {
  const [order, setOrder] = useState<number[]>([]);

  return (
    <div className="modal-box" style={{ minWidth: '520px' }}>
      <div className="modal-title">{title}</div>
      <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginBottom: '8px' }}>
        按希望的顺序依次点击卡牌（点击已排序的卡可撤销）
      </div>
      <div className="modal-cards-grid">
        {cards.map((c, idx) => {
          const rank = order.indexOf(idx);
          return (
            <CardTile
              key={idx}
              code={c.code}
              name={c.name}
              className="sort-card"
              dataAttrs={{ 'data-idx': idx }}
              onPick={() => {
                setOrder((cur) => (cur.includes(idx) ? cur.filter((i) => i !== idx) : (cur.length < cards.length ? [...cur, idx] : cur)));
              }}
            >
              <span className="sort-badge" data-idx={idx} style={{ display: rank > -1 ? 'block' : 'none' }}>
                {rank > -1 ? rank + 1 : ''}
              </span>
            </CardTile>
          );
        })}
      </div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginTop: '12px' }}>
        <button id="sort-cancel" className="btn btn-secondary" onClick={() => respond({ order: null })}>放弃排序</button>
        <button
          id="sort-confirm"
          className="btn btn-gold"
          disabled={order.length !== cards.length}
          onClick={() => respond({ order })}
        >确认</button>
      </div>
    </div>
  );
}

/** Multi-pick bitmask（MSG_ANNOUNCE_RACE / ATTRIB）：恰好 count 项，回位掩码 */
function BitmaskModal({ title, options, count, respond }: {
  title: string;
  options: [number, string][];
  count: number;
  respond: (mask: number) => void;
}) {
  const [mask, setMask] = useState(0);
  const picked = options.reduce((s, [v]) => s + (mask & v ? 1 : 0), 0);

  return (
    <div className="modal-box" style={{ minWidth: '480px', textAlign: 'center' }}>
      <div className="modal-title">{`${title}（选择 ${count} 项）`}</div>
      <div style={{ margin: '12px 0' }}>
        {options.map(([value, label]) => (
          <button
            key={value}
            className={`btn ${mask & value ? 'btn-primary' : 'btn-secondary'} mask-opt`}
            data-value={value}
            style={{ margin: '4px', padding: '6px 14px' }}
            onClick={() => {
              setMask((cur) => {
                if (cur & value) return cur & ~value;
                if (picked < count) return cur | value;
                return cur;
              });
            }}
          >{label}</button>
        ))}
      </div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginTop: '8px' }}>
        <span id="mask-status" style={{ fontSize: '13px', color: 'var(--text-muted)' }}>{`已选 ${picked} / ${count}`}</span>
        <button id="mask-confirm" className="btn btn-gold" disabled={picked !== count} onClick={() => respond(mask)}>确认</button>
      </div>
    </div>
  );
}

/** 通用选项列表（MSG_ANNOUNCE_NUMBER / SELECT_OPTION）：原版 wOptions 一页
 *  至多 5 项，<<< / >>> 翻页；回所选选项下标 */
function OptionListModal({ title, optionLabels, respond }: {
  title: string;
  optionLabels: string[];
  respond: (idx: number) => void;
}) {
  const PAGE_SIZE = 5;
  const pageCount = Math.ceil(optionLabels.length / PAGE_SIZE);
  const [page, setPage] = useState(0);
  const start = page * PAGE_SIZE;
  const slice = optionLabels.slice(start, start + PAGE_SIZE);

  return (
    <div className="modal-box modal-options">
      <div className="modal-title">{title}</div>
      <div id="opt-list" className="opt-list">
        {slice.map((label, i) => (
          <button key={start + i} className="num-opt" data-idx={start + i} onClick={() => respond(start + i)}>{label}</button>
        ))}
      </div>
      {pageCount > 1 && (
        <div className="opt-pager">
          <button id="opt-prev" className="btn btn-secondary" disabled={page === 0} onClick={() => setPage((p) => Math.max(0, p - 1))}>&lt;&lt;&lt;</button>
          <span id="opt-page-label">{`${page + 1} / ${pageCount}`}</span>
          <button id="opt-next" className="btn btn-secondary" disabled={page === pageCount - 1} onClick={() => setPage((p) => Math.min(pageCount - 1, p + 1))}>&gt;&gt;&gt;</button>
        </div>
      )}
    </div>
  );
}

/** Card announcement（MSG_ANNOUNCE_CARD）：有候选出卡图按钮，否则自由输入卡号 */
function AnnounceCardModal({ title, candidates, respond }: {
  title: string;
  candidates: { code: number; name: string }[];
  respond: (code: number) => void;
}) {
  const [input, setInput] = useState('');
  const confirmInput = () => {
    const v = parseInt(input, 10);
    if (!Number.isNaN(v) && v > 0) respond(v);
  };

  return (
    <div className="modal-box modal-announce-card">
      <div className="modal-title">{title}</div>
      {candidates.length > 0 && (
        <div className="modal-cards-grid">
          {candidates.map(({ code, name }, idx) => (
            <CardTile key={idx} code={code} name={name} className="cand-card" dataAttrs={{ 'data-idx': idx }} onPick={() => respond(code)} />
          ))}
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
        <button id="announce-card-confirm" className="btn btn-gold" onClick={confirmInput}>确认</button>
      </div>
    </div>
  );
}

// ---- 事件→弹窗路由 ----

interface Prompt {
  key: number;
  body: React.ReactNode;
}

const withName = async (c: { code: number }): Promise<{ code: number; name: string }> => {
  const info = await WailsBridge.getCard(c.code);
  return { code: c.code, name: (info && info.name) || `卡牌 #${c.code}` };
};

export default function PromptHost() {
  const [prompt, setPrompt] = useState<Prompt | null>(null);
  const seqRef = useRef(0);

  useEffect(() => {
    let alive = true;
    const show = (body: React.ReactNode) => {
      if (!alive) return;
      seqRef.current += 1;
      setPrompt({ key: seqRef.current, body });
    };
    // 应答即关弹窗（原版 hud 应答后销毁模态；下一个询问会再 show）
    const answer = <A extends unknown[]>(respond: (...args: A) => void) => (...args: A) => {
      if (!alive) return;
      setPrompt(null);
      respond(...args);
    };
    // 弹窗打开即从 store 清掉 stHintMsg（reducer 的 HINT_CLEARING 同步做；
    // 这里不再重复管理提示条）。
    const handler = (event: string, fn: (data: any) => void) => eventBus.on(event, fn);
    const off = (event: string, fn: (data: any) => void) => eventBus.off(event, fn);
    const subs: [string, (data: any) => void][] = [];

    const reg = (event: string, fn: (data: any) => void) => { handler(event, fn); subs.push([event, fn]); };

    reg('stoc:select_hand', () => {
      show(<RpsModal respond={answer((choice) => WailsBridge.sendHandResult(choice))} />);
    });

    reg('stoc:select_tp', () => {
      show(
        <YesNoModal
          title="先攻选择"
          message="你先攻吗？（先攻的第一个回合不能攻击）"
          respond={answer((yes) => WailsBridge.sendTPResult(yes ? 1 : 0))}
        />,
      );
    });

    reg('duel:select_yesno', async (data) => {
      // desc 是引擎字符串 id：App.ResolveDesc 能解出卡牌字符串（aux.Stringid
      // 为主的弹窗），系统字符串（无 strings.conf）返回空，退化为通用确认。
      let message = "是否执行该操作？";
      if (data && data.desc) {
        const text = await WailsBridge.resolveDesc(data.desc);
        if (text) message = text;
      }
      show(
        <YesNoModal
          title="确认"
          message={message}
          respond={answer((yes) => WailsBridge.sendResponseI(yes ? 1 : 0))}
        />,
      );
    });

    reg('duel:select_option', async (data) => {
      // options 携带未解析的 desc id：逐个解析为真实文本，解析不出则按编号兜底
      const options = data.options || [];
      const names = await Promise.all(options.map(async (desc: number, i: number) => {
        const text = await WailsBridge.resolveDesc(desc);
        return text || `选项 ${i + 1}`;
      }));
      const cards = names.map((name, i: number) => ({ code: i, name }));
      show(
        <CardSelectModal
          title="选择一个效果"
          cards={cards}
          min={1}
          max={1}
          cancelable={false}
          respond={answer(({ indices }) => { if (indices && indices.length) WailsBridge.sendResponseI(indices[0]); })}
        />,
      );
    });

    reg('duel:select_chain', async (data) => {
      const chains = data.chains || [];
      // 连锁模式偏好（gframe duelclient.cpp:1776 的本地代答逻辑）：非强制
      // 提示下，ignore 模式一律自动 -1。强制连锁不能跳过。
      if (!data.forced && chainPrefs.get() === 'ignore') {
        WailsBridge.sendResponseI(-1);
        return;
      }
      // 空连锁列表（或 UI 尚无法呈现的强制连锁）立即拒绝，不让对局干等
      if (!chains.length) {
        WailsBridge.sendResponseI(-1);
        return;
      }
      const cards = await Promise.all(chains.map(withName));
      // 强制连锁只有一张候选时直接应答（没有拒绝的余地）
      if (data.forced && cards.length === 1) {
        WailsBridge.sendResponseI(0);
        return;
      }
      if (cards.length === 1) {
        show(
          <YesNoModal
            title="连锁"
            message={`是否发动「${cards[0].name}」的连锁效果？`}
            respond={answer((yes) => WailsBridge.sendResponseI(yes ? 0 : -1))}
          />,
        );
      } else {
        // 强制连锁没有取消按钮：发 -1 引擎只会再问一遍
        show(
          <CardSelectModal
            title="选择要发动的效果"
            cards={cards}
            min={1}
            max={1}
            cancelable={!data.forced}
            respond={answer(({ indices }) => { if (indices && indices.length) WailsBridge.sendResponseI(indices[0]); })}
          />,
        );
      }
    });

    reg('duel:select_effectyn', async (data) => {
      // desc 通常指向具体发动的那条效果串（(code<<4)|offset），能解析就用原文；
      // 否则退化为按卡名询问。
      let message = "";
      if (data && data.desc) {
        message = await WailsBridge.resolveDesc(data.desc);
      }
      if (!message) {
        const info = await WailsBridge.getCard(data.code);
        const name = (info && info.name) || data.code;
        message = `是否发动「${name}」的效果？`;
      }
      show(
        <YesNoModal
          title="发动卡牌效果？"
          message={message}
          respond={answer((yes) => WailsBridge.sendResponseI(yes ? 1 : 0))}
        />,
      );
    });

    reg('duel:select_card', async (data) => {
      const cards = await Promise.all((data.cards || []).map(withName));
      show(
        <CardSelectModal
          title="选择卡牌"
          cards={cards}
          min={data.min}
          max={data.max}
          cancelable={!!data.cancelable}
          respond={answer(({ indices }) => {
            // ocgcore：可取消的选择用 -1 表示拒绝
            if (indices === null) WailsBridge.sendResponseI(-1);
            else WailsBridge.respondSelectCard(indices);
          })}
        />,
      );
    });

    reg('duel:select_position', (data) => {
      show(<PositionModal positions={data.positions} code={data.code} respond={answer((pos) => WailsBridge.sendResponseI(pos))} />);
    });

    // MSG_SELECT_COUNTER: 每卡一个 uint16 LE（与卡片列表同序，合计 = count）
    reg('duel:select_counter', async (data) => {
      const cards = await Promise.all((data.cards || []).map(async (c: { code: number; cnt: number }) => ({
        ...c, name: (await withName(c)).name,
      })));
      show(
        <CounterModal
          title="移除指示物"
          cards={cards}
          count={data.count}
          respond={answer((counts) => WailsBridge.respondCounter(counts))}
        />,
      );
    });

    // MSG_SELECT_SUM: 回包 [mustCount + pickedCount, ...select-list 下标]
    reg('duel:select_sum', async (data) => {
      // 保留 param（sum 选择要按卡面数值求和，withName 只留 code/name）
      const cards = await Promise.all((data.select || []).map(async (c: { code: number; param: number }) => ({
        ...c, name: (await withName(c)).name,
      })));
      const must = await Promise.all((data.must || []).map(async (c: { code: number; param: number }) => ({
        ...c, name: (await withName(c)).name,
      })));
      show(
        <SumSelectModal
          title="选择卡牌"
          cards={cards}
          must={must}
          acc={data.acc}
          min={data.min}
          max={data.max}
          sumMode={data.sumMode}
          respond={answer((selected) => WailsBridge.respondSelectSum(must.length + selected.length, selected))}
        />,
      );
    });

    // MSG_SORT_CARD: 回包是置换——字节 i = 用户给卡 i 排的位置；单发 0xff 取消
    reg('duel:sort_card', async (data) => {
      const cards = await Promise.all((data.cards || []).map(withName));
      show(
        <SortModal
          title="排列卡牌顺序"
          cards={cards}
          respond={answer(({ order }) => {
            if (order === null) {
              WailsBridge.respondSortCardCancel();
              return;
            }
            const perm = new Array(cards.length).fill(0);
            order.forEach((cardIdx: number, pos: number) => { perm[cardIdx] = pos; });
            WailsBridge.respondSortCard(perm);
          })}
        />,
      );
    });

    // MSG_SELECT_UNSELECT_CARD: 回包下标跨两张列表（select 在前，unselect 在后）
    reg('duel:select_unselect', async (data) => {
      const cards = await Promise.all((data.cards || []).map(withName));
      const unselect = await Promise.all((data.unselectList || []).map(withName));
      const combined = cards.map((c: SelectCard) => ({ ...c, kind: 'select' }))
        .concat(unselect.map((c: SelectCard) => ({ ...c, kind: 'unselect' })));
      if (!combined.length) {
        WailsBridge.sendResponseI(-1);
        return;
      }
      show(
        <CardSelectModal
          title={data.cancelable || data.finishable ? '选择/取消选择卡牌' : '选择卡牌'}
          cards={combined}
          min={1}
          max={1}
          cancelable={!!(data.cancelable || data.finishable)}
          respond={answer(({ indices }) => {
            if (indices === null) WailsBridge.sendResponseI(-1);
            else if (indices.length) WailsBridge.respondSelectUnselect(indices[0]);
          })}
        />,
      );
    });

    // MSG_ANNOUNCE_RACE / ATTRIB: 回包是 int32 位掩码，恰好 count 位来自 available
    reg('duel:announce_race', (data) => {
      const available = data.available >>> 0;
      show(
        <BitmaskModal
          title="宣言种族"
          options={RACES.filter(([v]) => available & (v as number)) as [number, string][]}
          count={data.count}
          respond={answer((mask) => WailsBridge.sendResponseI(mask))}
        />,
      );
    });
    reg('duel:announce_attrib', (data) => {
      const available = data.available >>> 0;
      show(
        <BitmaskModal
          title="宣言属性"
          options={ATTRS.filter(([v]) => available & (v as number)) as [number, string][]}
          count={data.count}
          respond={answer((mask) => WailsBridge.sendResponseI(mask))}
        />,
      );
    });

    // MSG_ANNOUNCE_NUMBER: 回包是所选选项下标
    reg('duel:announce_number', (data) => {
      const options = (data.options || []).map((v: number, i: number) => `选项 ${i + 1}（${v}）`);
      show(<OptionListModal title="宣言数字" optionLabels={options} respond={answer((idx) => WailsBridge.sendResponseI(idx))} />);
    });

    // MSG_ANNOUNCE_CARD: Go 已解码 opcode 表达式（candidates/decodable，
    // engine_bindings.go decorateAnnounceCard —— candidates 是裸卡号数组）；
    // 不可解码回退自由输入卡号
    reg('duel:announce_card', async (data) => {
      const raw = (data.decodable ? (data.candidates || []) : []) as (number | { code: number })[];
      const named = await Promise.all(raw.map((c) => withName(typeof c === 'object' ? c : { code: c })));
      show(<AnnounceCardModal title="宣言卡牌" candidates={named} respond={answer((code) => WailsBridge.sendResponseI(code))} />);
    });

    // MSG_ROCK_PAPER_SCISSORS：出拳值与 STOC 选择手牌同套（f1/f2/f3）
    reg('duel:rps', () => {
      show(<RpsModal respond={answer((choice) => WailsBridge.sendResponseI(choice))} />);
    });

    return () => {
      alive = false;
      subs.forEach(([event, fn]) => off(event, fn));
    };
  }, []);

  // Radix Dialog 壳：焦点圈定 + 背景滚动锁；视觉与冒烟 DOM 契约不变
  // （#modal-overlay 常驻、active 切换、内容仍是 overlay 后代）。
  return (
    <GameDialog open={!!prompt}>
      {prompt ? <div key={prompt.key}>{prompt.body}</div> : null}
    </GameDialog>
  );
}
