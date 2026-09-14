/**
 * 运行时全局注入的 Wails v3 环境与浏览器方言 API。
 * 只声明桥层实际用到的最小面，避免把整个 runtime 类型搬进来。
 */

interface Window {
  /** Wails v3 注入的 runtime（独立浏览器预览/冒烟页没有，走 mock 分支） */
  _wails?: {
    Call?: {
      ByName: (name: string, ...args: any[]) => any;
    };
    Events?: {
      On: (eventName: string, callback: (ev: { data?: any }) => void) => void;
    };
  };
  /** Safari 旧版前缀 AudioContext */
  webkitAudioContext?: typeof AudioContext;
}
