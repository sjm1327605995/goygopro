/** 先后手选择（原版 wFTSelect，game.cpp:514-518）：窗口 (550,240)-(780,340)
 *  230×100 无标题文字，btnFirst (10,30)-(220,55) 先攻（SysString 100）、
 *  btnSecond (10,60)-(220,85) 后攻（SysString 101）。 */
export function FtSelectModal({ respond }: { respond: (first: boolean) => void }) {
  return (
    <div className="modal-box modal-ftselect">
      <button id="ft-first-btn" className="btn btn-primary" onClick={() => respond(true)}>先攻</button>
      <button id="ft-second-btn" className="btn btn-secondary" onClick={() => respond(false)}>后攻</button>
    </div>
  );
}
