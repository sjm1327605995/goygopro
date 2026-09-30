import React from 'react';
import { MantineProvider } from '@mantine/core';
import { ygoTheme } from './theme.ts';
// Mantine 样式 + 本游戏皮肤在此集中引入：冒烟页不经过 main.tsx，
// 以 UiRoot 为唯一入口保证任何挂载点都带样式（vite 会去重）。
import '@mantine/core/styles.css';
import '../../css/ui-kit.css';

// 应用与冒烟页的统一外壳：Mantine v8 的组件离开 MantineProvider 会直接
// 抛错（不回退默认主题），所有 React 挂载点统一用它包一层。
export default function UiRoot({ children }: { children: React.ReactNode }) {
  return <MantineProvider theme={ygoTheme} forceColorScheme="dark">{children}</MantineProvider>;
}
