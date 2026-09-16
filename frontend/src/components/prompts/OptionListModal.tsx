import { useState } from 'react';

/** 通用选项列表（MSG_ANNOUNCE_NUMBER / SELECT_OPTION）：原版 wOptions 一页
 *  至多 5 项，<<< / >>> 翻页；回所选选项下标 */
export function OptionListModal({ title, optionLabels, respond }: {
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