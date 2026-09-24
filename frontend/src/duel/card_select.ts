/**
 * select_card / select_unselect 的应答出口（场上 3D 点选与弹窗共用）。
 *
 * 选择态在 duelStore.cardSelect（reducer 建，见 domain/reducer.ts 的
 * selectHandlers）；候选中的场上卡（mzone/szone）由 duel_manager 布 3D
 * 高亮并把点击 toggle 进同一 selected，弹窗（PromptHost）操作同一份
 * state，因此两条路径天然同步。这里只做「读 selected → 发应答 → 清态」：
 *  - select_card：应答全部已选下标（respondSelectCard）；
 *  - select_unselect：单选，应答唯一已选下标（respondSelectUnselect）；
 *  - 取消：可取消的询问回 -1（sendResponseI）。
 */
import { WailsBridge } from '../wails_bridge.ts';
import { duelStore } from './store.ts';

export function respondCardSelection(): void {
  const cs = duelStore.getState().cardSelect;
  if (!cs) return;
  if (cs.kind === 'card') {
    WailsBridge.respondSelectCard([...cs.selected]);
  } else {
    if (!cs.selected.length) return;
    WailsBridge.respondSelectUnselect(cs.selected[0]);
  }
  duelStore.disarmSelectHint();
  duelStore.endCardSelect();
}

export function cancelCardSelection(): void {
  const cs = duelStore.getState().cardSelect;
  if (!cs) return;
  WailsBridge.sendResponseI(-1);
  duelStore.disarmSelectHint();
  duelStore.endCardSelect();
}
