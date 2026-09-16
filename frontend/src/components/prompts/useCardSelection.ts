import { useState } from 'react';

/**
 * 卡片多选的共用状态机：点选/取消，selected 保持点击先后顺序，达到 max 上限
 * 后不再新增。CardSelectModal / SumSelectModal / SortModal 三处共用（原来各写
 * 一份相同的 toggle-with-cap 逻辑）。SortModal 依赖 selected 的点击顺序生成
 * 排序置换，故这里刻意用「按下标依次 push」而非集合。
 */
export function useCardSelection(min: number, max: number) {
  const [selected, setSelected] = useState<number[]>([]);
  const toggle = (idx: number) => {
    setSelected((cur) => {
      if (cur.includes(idx)) return cur.filter((i) => i !== idx);
      if (cur.length >= max) return cur;
      return [...cur, idx];
    });
  };
  return {
    selected,
    toggle,
    /** 是否已到最少可选张数（上限 max 由 toggle 保证，无需再查） */
    minMet: selected.length >= min,
  };
}