/**
 * PromptHost — 全部引擎询问弹窗的唯一宿主（原版 wHand/wPosSelect/wCardSelect/
 * wANNumber/wANCard/wOptions 等的 React 化）。
 *
 * 事件→弹窗→应答闭环都在这里：订阅 eventBus 的 select_* / announce_* / rps
 * 等事件，解析卡名（异步 getCard）后渲染单槽弹窗（新弹窗顶掉旧弹窗），用户
 * 点击后调 WailsBridge 的语义化 Respond* 方法（字节编码在 Go responses.go）。
 *
 * 各弹窗组件已拆到 components/prompts/（P5 第 1 项），本文件只剩事件路由；公共
 * 小件 CardTile 与多选状态机 useCardSelection 也随迁。
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
import { RACES, ATTRS } from '../domain/constants.ts';
import GameDialog from './GameDialog.tsx';
import { YesNoModal } from './prompts/YesNoModal.tsx';
import { CardSelectModal } from './prompts/CardSelectModal.tsx';
import { PositionModal } from './prompts/PositionModal.tsx';
import { RpsModal } from './prompts/RpsModal.tsx';
import { CounterModal } from './prompts/CounterModal.tsx';
import { SumSelectModal } from './prompts/SumSelectModal.tsx';
import { SortModal } from './prompts/SortModal.tsx';
import { BitmaskModal } from './prompts/BitmaskModal.tsx';
import { OptionListModal } from './prompts/OptionListModal.tsx';
import { AnnounceCardModal } from './prompts/AnnounceCardModal.tsx';
import type { SelectCard } from './prompts/CardTile.tsx';

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
          title="请选择"
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
          title="请选择排列顺序"
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
          title="请选择要宣言的种族"
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
          title="请选择要宣言的属性"
          options={ATTRS.filter(([v]) => available & (v as number)) as [number, string][]}
          count={data.count}
          respond={answer((mask) => WailsBridge.sendResponseI(mask))}
        />,
      );
    });

    // MSG_ANNOUNCE_NUMBER: 回包是所选选项下标
    reg('duel:announce_number', (data) => {
      const options = (data.options || []).map((v: number, i: number) => `选项 ${i + 1}（${v}）`);
      show(<OptionListModal title="请选择一个数字" optionLabels={options} respond={answer((idx) => WailsBridge.sendResponseI(idx))} />);
    });

    // MSG_ANNOUNCE_CARD: Go 已解码 opcode 表达式（candidates/decodable，
    // engine_bindings.go decorateAnnounceCard —— candidates 是裸卡号数组）；
    // 不可解码回退自由输入卡号
    reg('duel:announce_card', async (data) => {
      const raw = (data.decodable ? (data.candidates || []) : []) as (number | { code: number })[];
      const named = await Promise.all(raw.map((c) => withName(typeof c === 'object' ? c : { code: c })));
      show(<AnnounceCardModal title="请宣言一个卡名" candidates={named} respond={answer((code) => WailsBridge.sendResponseI(code))} />);
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