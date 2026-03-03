import puppeteer from 'puppeteer';
import { fileURLToPath } from 'url';
import { dirname, join } from 'path';

const __dirname = dirname(fileURLToPath(import.meta.url));

async function capture() {
  const browser = await puppeteer.launch({ headless: true });
  const page = await browser.newPage();

  // Dashboard light mode
  await page.setViewport({ width: 1400, height: 900, deviceScaleFactor: 2 });
  await page.goto(`file://${join(__dirname, 'dashboard-light.html')}`, { waitUntil: 'networkidle0' });
  await page.screenshot({ path: join(__dirname, 'dashboard-light.png'), fullPage: true });
  console.log('Captured: dashboard-light.png');

  // Dashboard dark mode
  await page.goto(`file://${join(__dirname, 'dashboard-dark.html')}`, { waitUntil: 'networkidle0' });
  await page.screenshot({ path: join(__dirname, 'dashboard-dark.png'), fullPage: true });
  console.log('Captured: dashboard-dark.png');

  // Dashboard light - cropped to viewport only (hero image)
  await page.goto(`file://${join(__dirname, 'dashboard-light.html')}`, { waitUntil: 'networkidle0' });
  await page.setViewport({ width: 1400, height: 800, deviceScaleFactor: 2 });
  await page.screenshot({ path: join(__dirname, 'dashboard-hero.png'), fullPage: false });
  console.log('Captured: dashboard-hero.png');

  await browser.close();
  console.log('Done!');
}

capture().catch(console.error);
