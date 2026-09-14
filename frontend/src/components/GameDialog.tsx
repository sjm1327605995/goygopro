/**
 * GameDialog — 原版风格弹窗的 Radix Dialog 壳（P7 波 9）。
 *
 * 只提供「行为」不提供「视觉」：焦点圈定（Tab 循环在弹窗内）、打开时锁背景
 * 滚动、aria role/dialog——视觉仍由 .modal-overlay / .modal-box（style.css +
 * duel-original.css）负责。
 *
 * 冒烟契约（prompts_smoke / practice_smoke）：
 *   - #modal-overlay 必须常驻 DOM（空闲也在），active 类只在打开时出现；
 *   - 弹窗内容是 overlay 的后代节点（overlay().querySelector(...) 可达）。
 * 因此不用 Dialog.Portal（内联渲染在 overlay div 里），Overlay 视觉层沿用
 * 自有 div。Esc / 点外部不允许关窗——引擎询问必须显式应答，静默关闭会让
 * 对局卡死（原版 gframe 同语义）。
 */
import React from 'react';
import * as Dialog from '@radix-ui/react-dialog';

export default function GameDialog({ open, children }: {
  open: boolean;
  children: React.ReactNode;
}) {
  return (
    <div id="modal-overlay" className={`modal-overlay${open ? ' active' : ''}`}>
      <Dialog.Root open={open}>
        {open ? (
          <Dialog.Content
            className="modal-dialog-content"
            aria-describedby={undefined}
            onEscapeKeyDown={(e) => e.preventDefault()}
            onPointerDownOutside={(e) => e.preventDefault()}
            onInteractOutside={(e) => e.preventDefault()}
          >
            {children}
          </Dialog.Content>
        ) : null}
      </Dialog.Root>
    </div>
  );
}
