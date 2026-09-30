import React from 'react';
import { TextInput, type TextInputProps, Checkbox, type CheckboxProps } from '@mantine/core';
import { cn } from '../lib/utils.ts';

/**
 * UiInput / UiCheckbox —— Mantine TextInput/Checkbox 的统一皮肤封装
 * （皮肤类 .ui-input/.ui-check 在 css/ui-kit.css），className 合并走 cn()。
 * UiInput 不带 label 用法（标签沿用各窗口的 .gfw-label 文本），与 gfw-input 同高 22px。
 */
export function UiInput({ classNames, ...rest }: TextInputProps) {
  return (
    <TextInput
      classNames={{ input: cn('ui-input', typeof classNames === 'object' ? classNames?.input : undefined) }}
      {...rest}
    />
  );
}

/** UiCheckbox —— Mantine Checkbox 统一皮肤（暗底、选中青色）。 */
export function UiCheckbox({ classNames, ...rest }: CheckboxProps) {
  return (
    <Checkbox
      classNames={{ input: cn('ui-check', typeof classNames === 'object' ? classNames?.input : undefined) }}
      {...rest}
    />
  );
}
