import { createTheme, type MantineColorsTuple } from '@mantine/core';

// 10 档色阶（Mantine 约定：dark scheme 下填充色取 6 档，边框/暗面取 7-9）
// navy = 中性炭黑面板色阶（原「钢铁蓝」已退役），与 css/style.css 的
// #1b2434/#161d2b/#131a26/#0d131d 同源
const navy: MantineColorsTuple = [
  '#eef1f6', '#d7dde8', '#b6c1d3', '#93a1b8', '#74849f',
  '#5a6b87', '#43536c', '#1b2434', '#161d2b', '#0d131d',
];
// masterblue = 主色（Master Duel 深蓝），与 --primary #3b82f6 同源
const masterblue: MantineColorsTuple = [
  '#eff5ff', '#dbe8fe', '#bfd7fe', '#93bbfd', '#609afa',
  '#3b82f6', '#2570eb', '#1d64d8', '#1e59af', '#1e4f8a',
];
// gold = 强调色（选中/卡牌/主操作），与 --accent-gold #eab308 同源
const gold: MantineColorsTuple = [
  '#fefce8', '#fef9c3', '#fef08a', '#fde047', '#facc15',
  '#eab308', '#ca8a04', '#a16207', '#854d0e', '#713f12',
];

/**
 * 统一 UI 主题（Mantine）。尺寸对齐原 gframe 几何：按钮 26px = compact-sm、
 * 输入框 22px = compact-xs、复选框 14px ≈ xs。颜色/圆角/文字色在
 * css/ui-kit.css 里用 CSS 变量与类覆盖（未分层 CSS 恒胜 Mantine 的 @layer）。
 */
export const ygoTheme = createTheme({
  primaryColor: 'masterblue',
  colors: { navy, masterblue, gold },
  fontFamily: "'Segoe UI', -apple-system, BlinkMacSystemFont, Roboto, 'Microsoft YaHei', sans-serif",
  defaultRadius: 'sm',
  components: {
    Button: { defaultProps: { size: 'compact-sm' } },
    TextInput: { defaultProps: { size: 'compact-xs' } },
    Checkbox: { defaultProps: { size: 'xs' } },
  },
});
