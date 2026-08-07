import fs from 'fs';
import path from 'path';

// Create icons directory if it doesn't exist
const iconsDir = path.resolve('icons');
const publicIconsDir = path.resolve('public/icons');

[iconsDir, publicIconsDir].forEach((dir) => {
  if (!fs.existsSync(dir)) {
    fs.mkdirSync(dir, { recursive: true });
  }
});

// A minimal 1x1 transparent/green PNG base64 string
const base64Png = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==';
const buffer = Buffer.from(base64Png, 'base64');

['icon16.png', 'icon48.png', 'icon128.png'].forEach((file) => {
  fs.writeFileSync(path.join(iconsDir, file), buffer);
  fs.writeFileSync(path.join(publicIconsDir, file), buffer);
});

console.log('Generated extension icons successfully.');
