export function YesNoModal({ title, message, respond }: { title: string; message: string; respond: (yes: boolean) => void }) {
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