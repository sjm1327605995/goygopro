/** 猜拳（原版 wHand）：f1=剪刀→1、f2=石头→2、f3=布→3（gframe game.cpp:508-512
 *  btnHand[i] 贴图 f{i+1}.jpg、event_handler.cpp:33 发 i+1；引擎判定
 *  2 胜 1、3 胜 2、1 胜 3，operations.cpp:6544） */
export function RpsModal({ respond }: { respond: (choice: number) => void }) {
  return (
    <div className="modal-box modal-rps">
      <div className="modal-title">猜拳</div>
      <div className="rps-row">
        <button id="rps-rock" className="rps-hand-btn rps-hand-rock" title="石头" onClick={() => respond(2)} />
        <button id="rps-scissors" className="rps-hand-btn rps-hand-scissors" title="剪刀" onClick={() => respond(1)} />
        <button id="rps-paper" className="rps-hand-btn rps-hand-paper" title="布" onClick={() => respond(3)} />
      </div>
    </div>
  );
}