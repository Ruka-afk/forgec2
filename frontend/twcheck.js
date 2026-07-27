/* eslint-disable @typescript-eslint/no-require-imports */
const fs = require('fs');
let postcss;
try { postcss = require('postcss'); } catch { postcss = require('@tailwindcss/postcss/node_modules/postcss'); }
const tw = require('@tailwindcss/postcss');
const css = fs.readFileSync('src/app/globals.css', 'utf8');
postcss([tw()]).process(css, { from: 'src/app/globals.css' }).then(r => {
  const out = r.css;
  fs.writeFileSync('C:\\Users\\18354\\AppData\\Local\\Temp\\opencode\\out.css', out);
  const checks = ['--color-text-primary', '--text-primary', '--background', '--color-background',
    '--color-border', '--border', ':root', 'gap-4', '--sidebar-width'];
  checks.forEach(c => {
    // for selectors like :root, check presence as substring
    console.log((out.includes(c) ? 'FOUND  ' : 'MISSING') + ': ' + c);
  });
  // show first 600 chars to inspect :root / theme output
  console.log('--- HEAD ---');
  console.log(out.slice(0, 800));
}).catch(e => { console.error('ERR', e.message); });
