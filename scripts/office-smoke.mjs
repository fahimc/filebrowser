// Run against a disposable preview folder containing the four named sample files.
// Usage: node scripts/office-smoke.mjs URL FILES_FOLDER SCREENSHOTS_FOLDER
import { chromium, expect } from '../frontend/node_modules/@playwright/test/index.mjs';
import { waitForEditorReady, typeIntoDocument } from '../third_party/ranuts-document/test/e2e/actions/editor.ts';
import { readFile, mkdir, stat } from 'node:fs/promises';
import { join } from 'node:path';
import { inflateRawSync } from 'node:zlib';

const [baseURL, filesDirectory, screenshotDirectory] = process.argv.slice(2);
const saveOnly = process.env.OFFICE_SMOKE_SAVE_ONLY === '1';
const creationOnly = process.env.OFFICE_SMOKE_CREATE_ONLY === '1';
if (!baseURL || !filesDirectory || !screenshotDirectory) throw new Error('Pass the preview URL, disposable files folder, and screenshots folder.');
await mkdir(screenshotDirectory, { recursive: true });
const browser = await chromium.launch({ executablePath: process.env.OFFICE_BROWSER || 'C:/Program Files/Google/Chrome/Application/chrome.exe', headless: true });
const page = await browser.newPage({ viewport: { width: 1440, height: 960 } });
await page.addInitScript(() => {
  window.__officeEvents = [];
  window.addEventListener('message', (event) => {
    if (event.data?.type === 'document:dirty-changed' || event.data?.type === 'document:saved') {
      window.__officeEvents.push({ type: event.data.type, dirty: event.data.payload?.dirty });
    }
  });
});
const failures = [];
page.on('response', (response) => { if (response.status() >= 400 && /\/(assets|sdkjs|web-apps|fonts|ran-fonts|ranui-iife)\//.test(response.url())) failures.push(`${response.status()} ${response.url()}`); });
page.on('pageerror', (error) => console.error('PAGE ERROR:', error.message));
const listing = new URL('/files/My%20Drive/', baseURL).href;
const status = page.locator('.document-editor__status');

function xmlText(bytes) {
  const eocd = bytes.lastIndexOf(Buffer.from([0x50, 0x4b, 0x05, 0x06]));
  if (eocd < 0) throw new Error('Saved Office document is not a ZIP file');
  let cursor = bytes.readUInt32LE(eocd + 16);
  let result = '';
  for (let i = 0; i < bytes.readUInt16LE(eocd + 10); i++) {
    const method = bytes.readUInt16LE(cursor + 10);
    const size = bytes.readUInt32LE(cursor + 20);
    const nameLength = bytes.readUInt16LE(cursor + 28);
    const extraLength = bytes.readUInt16LE(cursor + 30);
    const commentLength = bytes.readUInt16LE(cursor + 32);
    const offset = bytes.readUInt32LE(cursor + 42);
    const name = bytes.subarray(cursor + 46, cursor + 46 + nameLength).toString();
    if (/^(word\/document|xl\/(sharedStrings|worksheets\/sheet\d+)|ppt\/slides\/slide\d+)\.xml$/.test(name)) {
      const start = offset + 30 + bytes.readUInt16LE(offset + 26) + bytes.readUInt16LE(offset + 28);
      const compressed = bytes.subarray(start, start + size);
      result += (method === 0 ? compressed : inflateRawSync(compressed)).toString();
    }
    cursor += 46 + nameLength + extraLength + commentLength;
  }
  return result.replace(/<[^>]+>/g, '').replace(/&amp;/g, '&').replace(/&lt;/g, '<').replace(/&gt;/g, '>');
}

async function openFile(name) {
  await page.goto(listing);
  await page.getByRole('button', { name, exact: true }).dblclick();
  const ready = await waitForEditorReady(page);
  await expect(status).toHaveText('Ready', { timeout: 15000 });
  return ready.kind;
}
async function assertSaved(name, marker) {
  await expect(status).toHaveText('All changes saved', { timeout: 60000 });
  expect(xmlText(await readFile(join(filesDirectory, name))).includes(marker), `Saved ${name} contains the edited text`).toBe(true);
}

try {
  await page.goto(listing);
  await expect(page.getByRole('button', { name: 'Quarterly report.docx', exact: true })).toBeVisible();
  await page.screenshot({ path: join(screenshotDirectory, 'files.png') });

  if (!saveOnly && !creationOnly) for (const [name, label] of [['Quarterly report.docx', 'word'], ['Team budget.xlsx', 'excel'], ['Team update.pptx', 'powerpoint']]) {
    const kind = await openFile(name);
    const marker = `OfficeSaved${label}${Date.now()}`;
    await typeIntoDocument(page, kind, marker);
    await assertSaved(name, marker);
    await page.screenshot({ path: join(screenshotDirectory, `${label}.png`) });
    await page.locator('.document-editor__toolbar').getByRole('button', { name: 'Close', exact: true }).click();
    await expect(page.getByRole('button', { name, exact: true })).toBeVisible();
    await openFile(name);
    expect(xmlText(await readFile(join(filesDirectory, name))).includes(marker)).toBe(true);
    console.log(`PASS ${name}: edit, autosave to original path, reopen`);
  }

  if (!creationOnly) {
  const kind = await openFile('Quarterly report.docx');
  await page.route('**/api/resources?*', async (route) => {
    if (route.request().method() === 'PUT') await route.fulfill({ status: 503, contentType: 'text/plain', body: 'Simulated save failure' });
    else await route.continue();
  });
  const retryMarker = `OfficeSaveRetry${Date.now()}`;
  await typeIntoDocument(page, kind, retryMarker);
  await expect(status).toContainText('Simulated save failure', { timeout: 60000 });
  await expect(page.locator('.document-editor__save')).toBeEnabled();
  await page.unroute('**/api/resources?*');
  await page.locator('.document-editor__save').click();
  await assertSaved('Quarterly report.docx', retryMarker);
  const closeMarker = `Flush${Date.now()}`;
  // A host-toolbar click blurs the canvas; restore focus with a real click.
  await page.frames().find((frame) => frame.url().includes('/documenteditor/')).locator('#editor_sdk').click({ position: { x: 300, y: 250 } });
  await typeIntoDocument(page, kind, closeMarker);
  console.log('Before close:', await status.innerText(), await page.evaluate(() => window.__officeEvents));
  await page.screenshot({ path: join(screenshotDirectory, 'before-close.png') });
  await page.locator('.document-editor__toolbar').getByRole('button', { name: 'Close', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Quarterly report.docx', exact: true })).toBeVisible({ timeout: 60000 });
  expect(xmlText(await readFile(join(filesDirectory, 'Quarterly report.docx'))).includes(closeMarker), 'Closing flushes the last edits to disk').toBe(true);
  console.log('PASS failed-save retry and close-before-autosave preserve edits');
  }

  if (!saveOnly) {
  await openFile('Annual report.pdf');
  await page.evaluate(() => {
    const find = (win) => {
      if (win.Asc?.editor?.AddFreeTextAnnot) return win.Asc.editor;
      for (let i = 0; i < win.frames.length; i++) { const api = find(win.frames[i]); if (api) return api; }
    };
    const api = find(window);
    if (!api) throw new Error('PDF annotation API unavailable');
    api.AddFreeTextAnnot(0);
  });
  await expect(status).toHaveText('All changes saved', { timeout: 60000 });
  const pdf = (await readFile(join(filesDirectory, 'Annual report.pdf'))).toString('latin1');
  expect(pdf.startsWith('%PDF-')).toBe(true);
  expect(/\/Annots|\/FreeText/.test(pdf)).toBe(true);
  await page.screenshot({ path: join(screenshotDirectory, 'pdf.png') });
  console.log('PASS PDF: open, annotate, autosave ordinary PDF');

  for (const ext of ['docx', 'xlsx', 'pptx', 'pdf']) {
    console.log(`Checking create ${ext.toUpperCase()}...`);
    await page.goto(listing);
    await page.getByRole('button', { name: 'New', exact: true }).click();
    await page.getByRole('button', { name: 'New file', exact: true }).click();
    await page.getByRole('button', { name: `New file (${ext.toUpperCase()})`, exact: true }).click();
    const name = `Created ${Date.now()}.${ext}`;
    await page.getByLabel('New document filename').fill(name);
    const { kind: newKind } = await waitForEditorReady(page);
    await expect(page.locator('.document-editor__save')).toBeEnabled({ timeout: 15000 });
    await page.locator('.document-editor__save').click();
    await expect(status).toHaveText('All changes saved', { timeout: 60000 });
    expect((await stat(join(filesDirectory, name))).isFile()).toBe(true);
    expect((await stat(join(filesDirectory, name))).size).toBeGreaterThan(100);
    if (ext !== 'pdf') {
      const frame = page.frames().find((entry) => /\/(documenteditor|spreadsheeteditor|presentationeditor)\//.test(entry.url()));
      await frame.locator('#editor_sdk').click({ position: { x: 300, y: 250 } });
      const marker = `CreatedEdit${Date.now()}`;
      await typeIntoDocument(page, newKind, marker);
      await assertSaved(name, marker);
    }
    await page.screenshot({ path: join(screenshotDirectory, `create-${ext}.png`) });
    await page.locator('.office-editor-prompt .prompt-close').click();
    await expect(page.locator('.office-editor-prompt')).toHaveCount(0);
    console.log(`PASS create ${ext.toUpperCase()}: real file written in selected folder`);
  }
  const shares = [];
  try {
    for (const allowModify of [true, false]) {
      const response = await page.request.post(new URL('/api/share', baseURL).href, {
        data: { source: 'My Drive', path: '/', title: 'Office verification', shareType: 'normal', disableAnonymous: false, allowModify, allowCreate: allowModify, disableThumbnails: true },
      });
      expect(response.ok(), 'Create disposable local share').toBe(true);
      const share = await response.json();
      shares.push(share.hash);
      await page.goto(new URL(`/public/share/${share.hash}/Quarterly%20report.docx`, baseURL).href);
      const { kind: shareKind } = await waitForEditorReady(page);
      if (allowModify) {
        const marker = `SharedEdit${Date.now()}`;
        await expect(status).toHaveText('Ready');
        await typeIntoDocument(page, shareKind, marker);
        await assertSaved('Quarterly report.docx', marker);
        console.log('PASS shared document edits save to the same original file');
      } else {
        await expect(status).toHaveText('View only');
        await expect(page.locator('.document-editor__save')).toBeDisabled();
        const before = await readFile(join(filesDirectory, 'Quarterly report.docx'));
        await typeIntoDocument(page, shareKind, 'ReadonlyShouldNotPersist');
        await page.waitForTimeout(2200);
        expect((await readFile(join(filesDirectory, 'Quarterly report.docx'))).equals(before)).toBe(true);
        console.log('PASS view-only share cannot edit or save document contents');
      }
    }
    await page.goto(listing);
  } finally {
    for (const hash of shares) await page.request.delete(new URL(`/api/share?hash=${encodeURIComponent(hash)}`, baseURL).href);
  }
  }
  expect(failures).toEqual([]);
  console.log('PASS all integrated Office checks');
} catch (error) {
  await page.screenshot({ path: join(screenshotDirectory, 'failure.png') }).catch(() => {});
  throw error;
} finally {
  await browser.close();
}
