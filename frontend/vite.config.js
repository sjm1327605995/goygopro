import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { cpSync, existsSync, mkdirSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { resolve } from 'node:path';

const root = fileURLToPath(new URL('.', import.meta.url));

// The 3D field loads some textures at runtime (e.g. the card back) via a
// relative URL, so the textures/ directory must exist next to the built
// index.html. Vite only bundles assets it sees through imports/CSS url(); this
// plugin copies the raw texture files into dist after each build.
function copyTextures() {
  return {
    name: 'copy-textures',
    closeBundle() {
      const src = resolve(root, 'textures');
      const dst = resolve(root, 'dist', 'textures');
      if (existsSync(src)) {
        mkdirSync(dst, { recursive: true });
        cpSync(src, dst, { recursive: true });
      }
    },
  };
}

// Relative base so the built assets load correctly from the Wails v3
// AssetFileServerFS handler (file-like custom scheme), not just over http.
export default defineConfig({
  plugins: [react(), copyTextures()],
  base: './',
  build: {
    target: 'es2022', // 顶层 await（wails runtime 动态 import）
    outDir: 'dist',
    emptyOutDir: true,
    rollupOptions: {
      // /wails/runtime.js 由 Wails 资产服务器在运行时提供，构建期不存在
      external: [/^\/wails\//],
      output: {
        manualChunks(id) {
          if (id.includes('libs/three.module.js')) return 'three';
        },
      },
    },
  },
});
