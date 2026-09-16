/** 猜拳（原版 wHand）：f1=石头→1、f2=剪刀→2、f3=布→3（gframe
 *  event_handler.cpp:33；引擎判定 1 胜 2、2 胜 3、3 胜 1 operations.cpp:6538） */
export function RpsModal({ respond }: { respond: (choice: number) => void }) {
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