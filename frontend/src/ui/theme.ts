import { createTheme, type MantineColorsTuple } from '@mantine/core';

// 10 档色阶（Mantine 约定：dark scheme 下填充色取 6 档，边框/暗面取 7-9）
// navy = 控件底色（钢铁蓝），与 gframe-window.css 的 #34446f/#283654/#1f2a46 同源
const navy: MantineColorsTuple = [
  '#e7ecfb', '#c9d4f0', '#a3b6e2', '#7d97d4', '#5f7cc6',
  '#4a68b8', '#3d58a6', '#34446f', '#283654', '#1f2a46',
];
// gold = 主操作色（保存/开始/确定），与 --accent-gold #f59e0b 同源
const gold: MantineColorsTuple = [
  '#fdf3e0', '#f9e3bb', '#f4d08e', '#efbc60', '#eaac3d',
  '#f59e0b', '#d98605', '#b87204', '#965e03', '#7a4c02',
];

/**
 * 统一 UI 主题（Mantine）。尺寸对齐原 gframe 几何：按钮 26px = compact-sm、
 * 输入框 22px = compact-xs、复选框 14px ≈ xs。颜色/圆角/文字色在
 * css/ui-kit.css 里用 CSS 变量与类覆盖（未分层 CSS 恒胜 Mantine 的 @layer）。
 */
export const ygoTheme = createTheme({
  primaryColor: 'cyan',
  colors: { navy, gold },
  fontFamily: "'Segoe UI', -apple-system, BlinkMacSystemFont, Roboto, 'Microsoft YaHei', sans-serif",
  defaultRadius: 'sm',
  components: {
    Button: { defaultProps: { size: 'compact-sm' } },
    TextInput: { defaultProps: { size: 'compact-xs' } },
    Checkbox: { defaultProps: { size: 'xs' } },
  },
});
