/**
 * 卡名同步缓存 — reducer 写日志时把卡密码翻译成卡名的唯一途径。
 *
 * reducer 是纯函数（不碰桥/DOM），拿不到异步 getCard；谁解析到卡名
 * （duel_manager、PromptHost、冒烟桩）就往这里登记一份，之后所有日志
 * 共享。未登记的卡显示 `#密码` 兜底。
 */
const names = new Map<number, string>();

export function registerCardName(code: number, name: string | undefined | null): void {
  if (code && name) names.set(code, name);
}

export function cardName(code: number): string {
  return names.get(code) || `#${code}`;
}

export function clearCardNames(): void {
  names.clear();
}
