// Usage: node scripts/office-collaboration-smoke.mjs URL FILES_FOLDER SCREENSHOTS_FOLDER
// Optional OFFICE_TEST_PASSWORD_FILE signs in as OFFICE_TEST_USER (default tester).
import {chromium,expect} from '../frontend/node_modules/@playwright/test/index.mjs';
import {waitForEditorReady,typeIntoDocument,focusEditor} from '../third_party/ranuts-document/test/e2e/actions/editor.ts';
import {readFile,mkdir} from 'node:fs/promises';
import {join} from 'node:path';
import {inflateRawSync} from 'node:zlib';
const [base,files,screenshots]=process.argv.slice(2);
if(!base||!files||!screenshots)throw new Error('Pass URL, disposable files folder and screenshots folder.');
await mkdir(screenshots,{recursive:true});
const browser=await chromium.launch({executablePath:process.env.OFFICE_BROWSER||'C:/Program Files/Google/Chrome/Application/chrome.exe',headless:true});
const contexts=[];
async function device(){
 const context=await browser.newContext({viewport:{width:1440,height:960}});contexts.push(context);
 if(process.env.OFFICE_TEST_PASSWORD_FILE){
  const password=(await readFile(process.env.OFFICE_TEST_PASSWORD_FILE,'utf8')).trim();
  const login=await context.request.post(base+'/api/auth/login?username='+encodeURIComponent(process.env.OFFICE_TEST_USER||'tester'),{headers:{'X-Password':encodeURIComponent(password)}});
  expect(login.status()).toBe(200);
 }
 const page=await context.newPage();page.on('pageerror',e=>console.error('PAGE ERROR',e.message));return page;
}
async function open(page,name){
 await page.goto(base+'/files/My%20Drive/'+encodeURIComponent(name));await waitForEditorReady(page);
 await expect(page.locator('.document-editor__presence')).toContainText('Live',{timeout:30000});
}
async function model(page,kind){return page.evaluate(kind=>{
 const w=Array.from(window.frames).flatMap(w=>Array.from(w.frames)).find(w=>w.Asc?.editor),api=w.Asc.editor;
 if(kind==='cell')return Array.from({length:20},(_,r)=>Array.from({length:6},(_,c)=>api.wbModel.aWorksheets[0].getCell3(r,c).getValue())).flat().join('|');
 if(kind==='slide')return api.WordControl.m_oLogicDocument.Slides.map(s=>s.cSld.spTree.map(shape=>(shape.txBody?.content?.Content||[]).map(p=>p.GetText?.()||'').join('')).join('|')).join('\n');
 if(kind==='pdf')return api.getPDFDoc().annots.map(a=>a.Get_Id()).sort();
 const ps=[];api.WordControl.m_oLogicDocument.GetAllParagraphs({All:true},ps);return ps.map(p=>p.GetText()).join('\n');
},kind);}
async function saved(page){await expect(page.locator('.document-editor__status')).toHaveText('All changes saved',{timeout:60000});}
function officeText(bytes){
 const eocd=bytes.lastIndexOf(Buffer.from([0x50,0x4b,0x05,0x06]));expect(eocd).toBeGreaterThan(0);
 let offset=bytes.readUInt32LE(eocd+16),text='';
 for(let i=0;i<bytes.readUInt16LE(eocd+10);i++){
  const method=bytes.readUInt16LE(offset+10),size=bytes.readUInt32LE(offset+20),names=bytes.readUInt16LE(offset+28),extra=bytes.readUInt16LE(offset+30),comment=bytes.readUInt16LE(offset+32),local=bytes.readUInt32LE(offset+42);
  const name=bytes.subarray(offset+46,offset+46+names).toString();
  if(/^(word\/document|xl\/(sharedStrings|worksheets\/sheet\d+)|ppt\/slides\/slide\d+)\.xml$/.test(name)){
   const start=local+30+bytes.readUInt16LE(local+26)+bytes.readUInt16LE(local+28),compressed=bytes.subarray(start,start+size);
   text+=(method===0?compressed:inflateRawSync(compressed)).toString();
  }
  offset+=46+names+extra+comment;
 }
 return text.replace(/<[^>]+>/g,'');
}
async function close(page){
 await page.locator('.document-editor__toolbar').getByRole('button',{name:'Close',exact:true}).click();
 await expect(page.locator('.document-editor')).toHaveCount(0,{timeout:60000});await page.close();
}
try{
 for(const [name,kind] of [['Quarterly report.docx','word'],['Team budget.xlsx','cell'],['Team update.pptx','slide'],['Annual report.pdf','pdf']]){
  console.log('Checking two devices:',name);
  const a=await device(),b=await device();await open(a,name);await open(b,name);
  for(const p of [a,b])await expect(p.locator('.document-editor__presence')).toContainText('2 devices',{timeout:15000});
  const alpha='Alpha'+Date.now(),beta='Beta'+Date.now();
  if(kind==='cell'){
   await Promise.all([a,b].map(async(p,i)=>{await focusEditor(p);await p.keyboard.press('Control+Home');for(let n=0;n<6;n++)await p.keyboard.press('ArrowDown');if(i)await p.keyboard.press('ArrowRight');await p.keyboard.type(i?beta:alpha,{delay:35});await p.keyboard.press('Enter');}));
  }else if(kind==='pdf'){
   const count=(await model(a,kind)).length;
   await Promise.all([a,b].map(p=>p.evaluate(()=>Array.from(window.frames).flatMap(w=>Array.from(w.frames)).find(w=>w.Asc?.editor).Asc.editor.AddFreeTextAnnot(0))));
   for(const p of [a,b])await expect.poll(async()=>(await model(p,kind)).length,{timeout:60000}).toBe(count+2);
  }else await Promise.all([typeIntoDocument(a,kind,alpha),typeIntoDocument(b,kind,beta)]);
  if(kind!=='pdf'){
   for(const p of [a,b]){await expect.poll(()=>model(p,kind),{timeout:60000}).toContain(alpha);await expect.poll(()=>model(p,kind),{timeout:60000}).toContain(beta);}
  }
  expect(await model(a,kind)).toEqual(await model(b,kind));await saved(a);await saved(b);
  await a.screenshot({path:join(screenshots,kind+'.png')});
  const persisted=await readFile(join(files,name));expect(persisted.length).toBeGreaterThan(100);
  if(kind!=='pdf'){expect(officeText(persisted)).toContain(alpha);expect(officeText(persisted)).toContain(beta);}
  if(kind==='word'){
   const late=await device();await open(late,name);
   expect(await model(late,kind)).toEqual(await model(a,kind));console.log('PASS late join');
   await expect.poll(()=>b.evaluate(()=>Object.keys(Array.from(window.frames).flatMap(w=>Array.from(w.frames)).find(w=>w.Asc?.editor).AscCommon.CollaborativeEditing.m_aForeignCursors||{}).length),{timeout:15000}).toBeGreaterThan(0);
   console.log('PASS remote cursor and participant presence');
   // The upstream converter normally terminates after 30 seconds idle.
   // An embedded session must retain enough state to export after that period.
   await a.waitForTimeout(31000);
   await a.context().setOffline(true);
   const offline='Offline'+Date.now(),online='Online'+Date.now();
   await typeIntoDocument(a,kind,offline);
   const downloadPromise=a.waitForEvent('download');await a.getByRole('button',{name:'Download copy',exact:true}).click();
   const draft=await downloadPromise;expect(officeText(await readFile(await draft.path()))).toContain(offline);
   await a.frames().find(f=>f.url().includes('/documenteditor/')).locator('#editor_sdk').click({position:{x:300,y:250}});
   console.log('PASS offline draft download preserves unsent edits');
   await typeIntoDocument(b,kind,online);
   await expect.poll(()=>model(late,kind),{timeout:30000}).toContain(online);
   await a.context().setOffline(false);
   for(const p of [a,b]){await expect.poll(()=>model(p,kind),{timeout:60000}).toContain(offline);await expect.poll(()=>model(p,kind),{timeout:60000}).toContain(online);}
   expect(await model(a,kind)).toEqual(await model(b,kind));console.log('PASS reconnect with concurrent edits');
   let dropped=false;await a.route('**/api/office/changes?*',async r=>{if(!dropped){dropped=true;await r.fetch();await r.abort('failed');}else await r.continue();});
   const retry='Retry'+Date.now();await typeIntoDocument(a,kind,retry);
   await expect.poll(()=>model(b,kind),{timeout:60000}).toContain(retry);await saved(a);
   expect((await model(b,kind)).split(retry).length-1).toBe(1);await a.unroute('**/api/office/changes?*');console.log('PASS idempotent lost-response retry');
   await close(late);
   const stream='Stream'+Date.now()+'abcdefghijklmnopqrstuvwxyz0123456789';await focusEditor(a);await a.keyboard.press('Control+End');await a.keyboard.press('Enter');
   let complete=false;const typing=a.keyboard.type(stream,{delay:100}).then(()=>{complete=true;});
   await expect.poll(()=>model(b,kind),{timeout:4000}).toContain('Stream');expect(complete).toBe(false);await typing;
   await expect.poll(()=>model(b,kind),{timeout:30000}).toContain(stream);expect(await model(a,kind)).toEqual(await model(b,kind));
   console.log('PASS continuous typing streams before the typing action finishes');
   const before=await model(a,kind),count=(s,c)=>s.split(c).length-1;
   await Promise.all([typeIntoDocument(a,kind,'#'.repeat(60)),typeIntoDocument(b,kind,'~'.repeat(60))]);
   await expect.poll(async()=>JSON.stringify(await model(a,kind))===JSON.stringify(await model(b,kind)),{timeout:60000}).toBe(true);
   for(const p of [a,b]){
    const merged=await model(p,kind);expect(count(merged,'#')-count(before,'#')).toBe(60);expect(count(merged,'~')-count(before,'~')).toBe(60);
   }
   console.log('PASS concurrent typing through multiple native saves preserves every character');
  }
  await close(a);await close(b);console.log('PASS merged edits, checkpoint, and ordinary-file persistence:',name);
 }
 console.log('PASS all live Office collaboration checks');
}catch(error){
 for(const [i,c]of contexts.entries())for(const p of c.pages())await p.screenshot({path:join(screenshots,`failure-${i}.png`)}).catch(()=>{});
 throw error;
}finally{await browser.close();}
