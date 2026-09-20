import { defineConfig } from 'vite';
import { cpSync, existsSync, mkdirSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { resolve } from 'node:path';

const root = fileURLToPath(new URL('.', import.meta.url));

// Same raw-texture copy as the production config, but targeting smoke-dist so
// the runtime `textures/cover.jpg` load works under the smoke page.
function copyTextures() {
  return {
    name: 'copy-textures',
    closeBundle() {
      const src = resolve(root, 'textures');
      const dst = resolve(root, 'smoke-dist', 'textures');
      if (existsSync(src)) {
        mkdirSync(dst, { recursive: true });
        cpSync(src, dst, { recursive: true });
      }
    },
  };
}

export default defineConfig({
  plugins: [copyTextures()],
  base: './',
  build: {
    outDir: 'smoke-dist',
    emptyOutDir: true,
    rollupOptions: {
      input: {
        smoke: resolve(root, 'smoke.html'),
        replay_smoke: resolve(root, 'replay_smoke.html'),
        practice_smoke: resolve(root, 'practice_smoke.html'),
        prompts_smoke: resolve(root, 'prompts_smoke.html'),
        widgets_smoke: resolve(root, 'widgets_smoke.html'),
        lobby_smoke: resolve(root, 'lobby_smoke.html'),
        netplay_lobby_smoke: resolve(root, 'netplay_lobby_smoke.html'),
        deck_smoke: resolve(root, 'deck_smoke.html'),
        spec_smoke: resolve(root, 'spec_smoke.html'),
        stage_smoke: resolve(root, 'stage_smoke.html'),
        theater_smoke: resolve(root, 'theater_smoke.html'),
        settings_smoke: resolve(root, 'settings_smoke.html'),
        menu_smoke: resolve(root, 'menu_smoke.html'),
        single_smoke: resolve(root, 'single_smoke.html'),
      },
    },
  },
});
