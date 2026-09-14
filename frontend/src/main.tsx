import React from 'react';
import { createRoot } from 'react-dom/client';
import App from './App.tsx';
import '../css/style.css';
import '../css/gframe-window.css';
import '../css/duel-original.css';
// duelStore 单例在此被创建并接到事件总线上（所有 store 消费方 import 它）
import './duel/store.ts';

// No StrictMode on purpose: the imperative Three.js field (DuelField3D) and the
// Wails event bridge are set up once on mount; double-invoking effects would
// create duplicate WebGL renderers and duplicate event listeners.
createRoot(document.getElementById('root')!).render(<App />);
