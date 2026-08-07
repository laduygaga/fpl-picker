import { defineConfig } from 'vite';
import { resolve } from 'path';
import { copyFileSync, mkdirSync, existsSync } from 'fs';

function copyExtensionAssets() {
  return {
    name: 'copy-extension-assets',
    closeBundle() {
      // Ensure manifest and icon directory exist in dist
      if (!existsSync('dist')) {
        mkdirSync('dist', { recursive: true });
      }
      copyFileSync('manifest.json', 'dist/manifest.json');
      
      if (existsSync('src/content/content.css')) {
        if (!existsSync('dist/src/content')) {
          mkdirSync('dist/src/content', { recursive: true });
        }
        copyFileSync('src/content/content.css', 'dist/src/content/content.css');
      }

      if (!existsSync('dist/icons')) {
        mkdirSync('dist/icons', { recursive: true });
      }
      // Create SVG/PNG fallback placeholder icons if missing
      const icons = ['icon16.png', 'icon48.png', 'icon128.png'];
      for (const icon of icons) {
        if (!existsSync(`dist/icons/${icon}`)) {
          // If icon doesn't exist, generate a basic placeholder icon
          if (existsSync(`public/icons/${icon}`)) {
            copyFileSync(`public/icons/${icon}`, `dist/icons/${icon}`);
          }
        }
      }
    }
  };
}

export default defineConfig({
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    target: 'es2022',
    rollupOptions: {
      input: {
        popup: resolve(__dirname, 'src/popup/popup.html'),
        'service-worker': resolve(__dirname, 'src/background/service-worker.ts'),
        content: resolve(__dirname, 'src/content/content.ts'),
      },
      output: {
        entryFileNames: (chunkInfo) => {
          if (chunkInfo.name === 'service-worker') {
            return 'src/background/service-worker.js';
          }
          if (chunkInfo.name === 'content') {
            return 'src/content/content.js';
          }
          return 'src/[name]/[name].js';
        },
        chunkFileNames: 'assets/[name]-[hash].js',
        assetFileNames: (assetInfo) => {
          if (assetInfo.name === 'content.css') {
            return 'src/content/content.css';
          }
          return 'assets/[name]-[hash][extname]';
        },
      },
    },
  },
  plugins: [copyExtensionAssets()],
});
