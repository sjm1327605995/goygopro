import React from 'react';
import * as Select from '@radix-ui/react-select';

/**
 * GfwSelect —— 对 @radix-ui/react-select 的 gfw 复古皮肤薄封装。
 *
 * 为什么不用原生 <select>：
 * - Wails webview2 里原生下拉的弹出层行为很差（定位、缩放、焦点都有问题），
 *   Radix 的弹出层渲染在 body 的 portal 里，行为稳定可控；
 * - 受控值不在 options 内时，原生 select 静默回退、Radix 会渲染异常，
 *   封装层统一做了「value 必须落在 options 内」的兜底。
 *
 * 外观规格与 gfw 原生下拉（.gfw-select，见 css/gframe-window.css）完全一致：
 * 白底、1px #888 边框、12px 字号、22px 高（与原 <select> 同高）、圆角 0。
 *
 * 注意：Radix 的 Select.Item 不允许空字符串 value（空值用于清空选中显示
 * placeholder），所以调用处传 '' 一类的选项时必须用哨兵值（如 '__root__'）
 * 并在 onValueChange 里翻译回来。
 */
export interface GfwSelectOption {
  value: string;
  label: string;
}

export interface GfwSelectProps {
  /** 挂到 trigger（按钮）上；冒烟测试靠它定位 */
  id?: string;
  /**
   * 受控值。不在 options 内时触发器显示 placeholder（对齐原生 select 的
   * 「值不匹配则空白」语义）。注意：不能擅自回退到 options[0] —— Radix 受控
   * 模式下重选当前值不会触发 onValueChange，「显示首项但实际未选中」会让
   * 用户点该项时毫无反应（DeckBuilder 切分类后点唯一卡组的场景踩过）。
   */
  value: string;
  onValueChange: (value: string) => void;
  options: GfwSelectOption[];
  /** 触发器宽度（px 或 CSS 长度），透传到 trigger style.width */
  width?: number | string;
  /** 触发器 flex，透传到 trigger style.flex */
  flex?: number | string;
  /** 值未匹配/空表时触发器里的占位文案 */
  placeholder?: string;
  disabled?: boolean;
  className?: string;
  'aria-label'?: string;
}

export default function GfwSelect({
  id,
  value,
  onValueChange,
  options,
  width,
  flex,
  placeholder = '（请选择）',
  disabled,
  className = '',
  'aria-label': ariaLabel,
}: GfwSelectProps) {
  // Radix Select 的受控值必须命中某个 Item，否则触发器渲染异常；
  // 「值不在 options 内」对齐原生 select 的空白语义：交给 Root 空值走 placeholder
  //（Root value '' 合法且显示 placeholder，只有 Item 禁止空串 value）。
  const safeValue = options.some((o) => o.value === value) ? value : '';

  return (
    <Select.Root value={safeValue} onValueChange={onValueChange} disabled={disabled}>
      <Select.Trigger
        id={id}
        aria-label={ariaLabel}
        // .gfw-select/.form-select 提供 gfw 原生下拉的边框/底色/22px 高/字号
        className={`gfw-select form-select inline-flex cursor-pointer items-center justify-between gap-1 overflow-hidden ${className}`}
        style={{ width, flex }}
      >
        <Select.Value className="truncate" placeholder={placeholder} />
        <Select.Icon className="shrink-0 text-[9px] leading-none text-[#666]">▼</Select.Icon>
      </Select.Trigger>
      <Select.Portal>
        {/* 弹出层样式对齐 gfw-list 列表项（gframe-window.css）：白底、#888 边框、
            12px 字、20px 行高、高亮 #b8c4dc、选中 #4a6fa5 反白 */}
        <Select.Content
          position="popper"
          sideOffset={2}
          className="z-[100] min-w-[var(--radix-select-trigger-width)] border border-[#888] bg-white shadow-[0_4px_12px_rgba(0,0,0,0.4)]"
        >
          <Select.Viewport>
            {options.map((o) => (
              <Select.Item
                key={o.value}
                value={o.value}
                // data-value 供冒烟测试按值精确定位（Radix 默认不输出 value 到 DOM）
                data-value={o.value}
                className="flex h-[20px] cursor-default items-center px-[6px] text-[12px] leading-[20px] text-[#222] outline-none data-[highlighted]:bg-[#b8c4dc] data-[selected]:bg-[#4a6fa5] data-[selected]:text-white"
              >
                <Select.ItemText>{o.label}</Select.ItemText>
              </Select.Item>
            ))}
          </Select.Viewport>
        </Select.Content>
      </Select.Portal>
    </Select.Root>
  );
}
