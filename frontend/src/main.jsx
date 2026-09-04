import React from 'react';
import { createRoot } from 'react-dom/client';
import App from './App.jsx';
import '../css/style.css';

// No StrictMode on purpose: the imperative Three.js field (DuelField3D) and the
// Wails event bridge are set up once on mount; double-invoking effects would
// create duplicate WebGL renderers and duplicate event listeners.
createRoot(document.getElementById('root')).render(<App />);
