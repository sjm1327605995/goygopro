import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

/**
 * cn() —— shadcn/ui 约定的 className 合并工具（clsx 条件拼接 +
 * tailwind-merge 去重冲突）。所有 ui/ 组件的 className 合并统一走这里，
 * 将来组件实现换成 shadcn 时调用处零改动。
 */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
