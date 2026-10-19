const frames = (count: number) => count * 1000 / 60;

// The native client blocks Analyze with WaitFrameSignal before reading the next
// engine message. Keep live Wails ingress in that order; replay seek stays instant.
export function presentationDelay(name: string, data: any): number {
  if (data?.__instant) return 0;
  switch (name) {
    case 'duel:damage': case 'duel:recover': case 'duel:pay_lpcost': return frames(41);
    case 'duel:lp_update': return frames(11);
    case 'duel:start': case 'duel:new_turn': case 'duel:new_phase': case 'duel:attack': return frames(40);
    case 'duel:summoning': case 'duel:flipsummoning': return frames(41);
    case 'duel:spsummoning': return data?.code ? frames(41) : 0;
    case 'duel:chaining': case 'duel:chain_negated': return frames(30);
    case 'duel:card_hint': return data?.type === 1 && data?.data ? frames(30) : 0;
    case 'duel:hint':
      return [5, 10].includes(data?.type) ? frames(30) : [4, 6, 7, 8, 9].includes(data?.type) ? frames(40) : 0;
    case 'duel:win': return frames(120);
    case 'duel:hand_res': return frames(60);
    case 'duel:toss_coin': case 'duel:toss_dice': return frames(40);
    default: return 0;
  }
}

export class PresentationQueue {
  private pending: { name: string; data: any }[] = [];
  private timer: ReturnType<typeof setTimeout> | null = null;
  private draining = false;
  private epoch = 0;
  constructor(private deliver: (name: string, data: any) => void) {}

  clear() {
    this.epoch++;
    this.pending = [];
    if (this.timer !== null) clearTimeout(this.timer);
    this.timer = null;
  }

  enqueue(name: string, data: any) {
    if (name === 'duel:start' || name === 'stoc:disconnected') this.clear();
    const ordered = name.startsWith('duel:') || ['stoc:replay', 'stoc:duel_end', 'stoc:change_side', 'single:ended', 'single:replay'].includes(name);
    if (!ordered) { this.deliver(name, data); return; }
    this.pending.push({ name, data });
    this.drain();
  }

  private drain() {
    if (this.draining) return;
    this.draining = true;
    const epoch = this.epoch;
    try {
      while (this.timer === null && this.pending.length && epoch === this.epoch) {
        const next = this.pending.shift()!;
        const delay = presentationDelay(next.name, next.data);
        // deliver 抛错（emit 链路外的回调异常）不阻塞队列：当前事件已出队，
        // 记日志后继续按原定延迟调度剩余事件
        try {
          this.deliver(next.name, next.data);
        } catch (err) {
          console.error(`[presentationQueue] deliver "${next.name}" threw:`, err);
        }
        if (delay && epoch === this.epoch) {
          this.timer = setTimeout(() => { this.timer = null; this.drain(); }, Math.ceil(delay));
        }
      }
    } finally {
      this.draining = false;
      if (epoch !== this.epoch && this.pending.length) this.drain();
    }
  }
}
