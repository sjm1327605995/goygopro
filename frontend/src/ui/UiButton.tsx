import React from 'react';
import { Button, type ButtonProps } from '@mantine/core';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from '../lib/utils.ts';

/**
 * UiButton —— 统一按钮。接口对齐 shadcn/ui Button（variant + className +
 * cn() 合并），实现层用 Mantine Button 承担行为（焦点/禁用/布局）。
 *
 * - 自动挂 `.btn` 裸类：冒烟测试靠 `.panel .btn` + textContent 定位，
 *   该类只作钩子、不再带皮肤（style.css 的 .btn 皮肤已加 :not(.ui-btn) 豁免）。
 * - variant 语义：default=钢铁蓝、gold=主操作、danger=危险（皮肤类在
 *   css/ui-kit.css，cva 只负责映射）。
 * - 几何：默认 compact-sm（26px，对齐原 gfw-btn）；主菜单等用 compact-md。
 */
const buttonVariants = cva('btn ui-btn', {
  variants: {
    variant: {
      default: '',
      gold: 'ui-btn--gold',
      danger: 'ui-btn--danger',
    },
  },
  defaultVariants: { variant: 'default' },
});

// Mantine ButtonProps 是多态泛型，不含原生 button 属性（id/onClick 等），
// 交叉 ComponentPropsWithoutRef<'button'> 补齐（冒烟测试靠 id 定位）；
// 用 type 交叉而非 interface extends（两者的 style 类型定义不同，extends 会冲突）。
export type UiButtonProps = Omit<ButtonProps, 'variant'>
  & Omit<React.ComponentPropsWithoutRef<'button'>, 'color'>
  & VariantProps<typeof buttonVariants>;

export default function UiButton({ variant, className, ...rest }: UiButtonProps) {
  // Mantine 自带 variant 语义不走（皮肤由 .ui-btn 提供），统一 default
  return <Button variant="default" className={cn(buttonVariants({ variant }), className)} {...rest} />;
}

export { buttonVariants };
