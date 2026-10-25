// node render.js file.svg out.png WIDTH HEIGHT SCALE [transparent]
const { chromium } = require('playwright');
(async () => {
  const [,, svg, png, w, h, scale, transparent] = process.argv;
  const b = await chromium.launch();
  const p = await b.newPage({ viewport: { width: +w, height: +h }, deviceScaleFactor: +scale || 1 });
  await p.goto(`file://${svg}`);
  await p.waitForTimeout(500);
  await p.screenshot({ path: png, omitBackground: !!transparent });
  await b.close();
})();
