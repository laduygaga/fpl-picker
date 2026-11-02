import { defineConfig } from 'vite';
import { resolve } from 'path';
import { copyFileSync, mkdirSync, existsSync, readFileSync, writeFileSync } from 'fs';

function copyExtensionAssets(isFirefox: boolean) {
  return {
    name: 'copy-extension-assets',
    closeBundle() {
      const outDir = isFirefox ? 'dist-firefox' : 'dist';
      if (!existsSync(outDir)) {
        mkdirSync(outDir, { recursive: true });
      }

      if (isFirefox) {
        const manifestRaw = readFileSync('manifest.json', 'utf-8');
        const manifest = JSON.parse(manifestRaw);
        manifest.browser_specific_settings = {
          gecko: {
            id: 'fpl-picker-ai@fplpicker.com',
            strict_min_version: '121.0'
          }
        };
        if (manifest.background && manifest.background.service_worker) {
          manifest.background.scripts = [manifest.background.service_worker];
          delete manifest.background.service_worker;
        }
        writeFileSync(`${outDir}/manifest.json`, JSON.stringify(manifest, null, 2));
      } else {
        copyFileSync('manifest.json', `${outDir}/manifest.json`);
      }
      
      if (existsSync('src/content/content.css')) {
        const contentDir = `${outDir}/src/content`;
        if (!existsSync(contentDir)) {
          mkdirSync(contentDir, { recursive: true });
        }
        copyFileSync('src/content/content.css', `${contentDir}/content.css`);
      }

      const iconsDir = `${outDir}/icons`;
      if (!existsSync(iconsDir)) {
        mkdirSync(iconsDir, { recursive: true });
      }
      if (existsSync('public/icons/icon.svg')) {
        copyFileSync('public/icons/icon.svg', `${iconsDir}/icon.svg`);
      }
    }
  };
}

export default defineConfig(({ mode }) => {
  const isFirefox = mode === 'firefox';
  const outDir = isFirefox ? 'dist-firefox' : 'dist';

  return {
    resolve: {
      alias: {
        '@': resolve(__dirname, 'src'),
      },
    },
    build: {
      outDir,
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
    plugins: [copyExtensionAssets(isFirefox)],
  };
});
