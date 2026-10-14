// Run against disposable four-format fixtures. URL, files directory, screenshots directory.
import {chromium,expect} from '../frontend/node_modules/@playwright/test/index.mjs';
import {waitForEditorReady,typeIntoDocument} from '../third_party/ranuts-document/test/e2e/actions/editor.ts';
import {readFile,mkdir} from 'node:fs/promises';
import {join} from 'node:path';
import {createHash} from 'node:crypto';
import {inflateRawSync} from 'node:zlib';
const [base,files,screenshots]=process.argv.slice(2);
if(!base||!files||!screenshots)throw Error('Pass URL, disposable files folder and screenshots folder');
await mkdir(screenshots,{recursive:true});
const browser=await chromium.launch({executablePath:process.env.OFFICE_BROWSER||'C:/Program Files/Google/Chrome/Application/chrome.exe',headless:true});
const contexts=[];
async function device(){const c=await browser.newContext({viewport:{width:1440,height:960}});contexts.push(c);
 if(process.env.OFFICE_TEST_PASSWORD_FILE){const password=(await readFile(process.env.OFFICE_TEST_PASSWORD_FILE,'utf8')).trim();expect((await c.request.post(base+'/api/auth/login?username='+encodeURIComponent(process.env.OFFICE_TEST_USER||'tester'),{headers:{'X-Password':encodeURIComponent(password)}})).status()).toBe(200);}
 const p=await c.newPage();p.on('pageerror',e=>console.error('PAGE ERROR',e.message));return p;}
async function open(p,name){await p.goto(base+'/files/My%20Drive/'+encodeURIComponent(name));await waitForEditorReady(p);await expect(p.locator('.document-editor__presence')).toContainText('Live',{timeout:30000});}
const saved=p=>expect(p.locator('.document-editor__status')).toHaveText('All changes saved',{timeout:60000});
function docxText(bytes){const end=bytes.lastIndexOf(Buffer.from([80,75,5,6]));let at=bytes.readUInt32LE(end+16);for(let i=0;i<bytes.readUInt16LE(end+10);i++){const nameLength=bytes.readUInt16LE(at+28),extra=bytes.readUInt16LE(at+30),comment=bytes.readUInt16LE(at+32),size=bytes.readUInt32LE(at+20),local=bytes.readUInt32LE(at+42);if(bytes.subarray(at+46,at+46+nameLength).toString()==='word/document.xml'){const start=local+30+bytes.readUInt16LE(local+26)+bytes.readUInt16LE(local+28),data=bytes.subarray(start,start+size);return(bytes.readUInt16LE(at+10)===0?data:inflateRawSync(data)).toString().replace(/<[^>]+>/g,'');}at+=46+nameLength+extra+comment;}throw Error('Missing document XML');}
async function model(p,kind,history=false){return p.evaluate(({kind,history})=>{
 const visit=w=>{if(w.Asc?.editor)return w;for(let i=0;i<w.frames.length;i++){const f=visit(w.frames[i]);if(f)return f;}};
 const w=visit(document.querySelector(history?'.office-history__frame':'.document-editor__frame').contentWindow),api=w?.Asc?.editor;
 if(!api?.isDocumentLoadComplete||!api.isLoadFullApi)return null;
 if(kind==='cell')return Array.from({length:20},(_,r)=>Array.from({length:6},(_,c)=>api.wbModel.aWorksheets[0].getCell3(r,c).getValue())).flat().join('|');
 if(kind==='slide')return api.WordControl.m_oLogicDocument.Slides.map(s=>s.cSld.spTree.map(shape=>(shape.txBody?.content?.Content||[]).map(p=>p.GetText?.()||'').join('')).join('|')).join('\n');
 if(kind==='pdf')return api.getPDFDoc().annots.length;
 const ps=[];api.WordControl.m_oLogicDocument.GetAllParagraphs({All:true},ps);return ps.map(p=>p.GetText()).join('\n');
 },{kind,history});}
async function edit(p,kind,text){
 if(kind==='pdf')await p.evaluate(()=>{const visit=w=>{if(w.Asc?.editor)return w;for(let i=0;i<w.frames.length;i++){const f=visit(w.frames[i]);if(f)return f;}};visit(document.querySelector('.document-editor__frame').contentWindow).Asc.editor.AddFreeTextAnnot(0);});
 else {await p.frames().find(f=>/\/(document|spreadsheet|presentation)editor\//.test(f.url())).locator('#editor_sdk').click({position:{x:300,y:250}});await typeIntoDocument(p,kind,text);}
 await saved(p);
}
async function history(p){await p.getByRole('button',{name:'Version history',exact:true}).click();await expect(p.locator('.office-history__preview-bar')).toContainText('Read-only version',{timeout:120000});}
async function select(p,label){await p.locator('.office-history__version').filter({hasText:label}).first().click();await expect(p.locator('.office-history__preview-bar')).toContainText('Read-only version',{timeout:120000});}
async function nameCurrent(p,label){await p.locator('.office-history__timeline>li').first().getByRole('button',{name:'Name this version',exact:true}).click();await p.getByRole('textbox',{name:'Version name',exact:true}).fill(label);await p.getByRole('button',{name:'Save version name',exact:true}).click();await expect(p.locator('.office-history__version').first()).toContainText(label);}
async function closeHistory(p){await p.getByRole('button',{name:'Close version history',exact:true}).click();await expect(p.locator('.office-history')).toHaveCount(0);}
async function restore(p){await p.getByRole('button',{name:'Restore this version',exact:true}).click();await p.getByRole('button',{name:'Confirm restore',exact:true}).click();await expect(p.locator('.office-history')).toHaveCount(0,{timeout:60000});await waitForEditorReady(p);}
async function list(p,name){const r=await p.request.post(base+'/api/office/history-list?source=My%20Drive&path='+encodeURIComponent('/'+name),{data:{}});expect(r.status()).toBe(200);return (await r.json()).versions;}
async function close(p){await p.locator('.document-editor__toolbar').getByRole('button',{name:'Close',exact:true}).click();await expect(p.locator('.document-editor')).toHaveCount(0,{timeout:60000});await p.close();}
try{
 for(const [file,kind]of (process.env.OFFICE_HISTORY_API_ONLY==='1'?[]:[['Quarterly report.docx','word'],['Team budget.xlsx','cell'],['Team update.pptx','slide'],['Annual report.pdf','pdf']].filter(([,kind])=>!process.env.OFFICE_HISTORY_KIND||process.env.OFFICE_HISTORY_KIND===kind))){
  console.log('History:',file);const a=await device(),b=await device();await open(a,file);await open(b,file);
  const originalVersion=(await list(a,file)).find(v=>v.kind==='initial');const first='Approved '+kind+' '+Date.now();
  await edit(a,kind,'First'+Date.now());await expect.poll(()=>model(b,kind),{timeout:30000}).toEqual(await model(a,kind));
  const approvedModel=await model(a,kind);await history(a);await nameCurrent(a,first);
  // A peer can complete a further checkpoint before the name is pinned,
  // particularly PDF's incremental export. Verify the named snapshot itself.
  const approvedVersion=(await list(a,file)).find(v=>v.name===first);
  const snapshot=await a.request.get(base+'/api/office/history-blob?source=My%20Drive&path='+encodeURIComponent('/'+file)+'&version='+approvedVersion.id+'&digest='+approvedVersion.digest);
  expect(snapshot.status()).toBe(200);const approvedBytes=await snapshot.body();expect(createHash('sha256').update(approvedBytes).digest('hex')).toBe(approvedVersion.digest);await closeHistory(a);
  await edit(b,kind,'Second'+Date.now());await expect.poll(()=>model(a,kind),{timeout:30000}).toEqual(await model(b,kind));const secondModel=await model(b,kind);
  expect(secondModel).not.toEqual(approvedModel);await history(a);await select(a,first);
  await expect.poll(()=>model(a,kind,true),{timeout:120000}).toEqual(approvedModel);
  if(kind==='word'){
   // There are two native editors; target the snapshot's own descendant.
   await a.evaluate(()=>{const visit=w=>{if(w.Asc?.editor){w.document.getElementById('area_id')?.focus();return true;}for(let i=0;i<w.frames.length;i++)if(visit(w.frames[i]))return true;};visit(document.querySelector('.office-history__frame').contentWindow);});
   await a.keyboard.type('ForbiddenSnapshotEdit',{delay:20});expect(await model(a,kind,true)).toEqual(approvedModel);
  }
  const download=a.waitForEvent('download');await a.getByRole('button',{name:'Download this version',exact:true}).click();expect((await readFile(await (await download).path())).equals(approvedBytes)).toBe(true);
  await a.screenshot({path:join(screenshots,kind+'-history.png')});await restore(a);
  await expect.poll(()=>model(a,kind),{timeout:60000}).toEqual(approvedModel);await expect.poll(()=>model(b,kind),{timeout:120000}).toEqual(approvedModel);
  expect((await readFile(join(files,file))).equals(approvedBytes)).toBe(true);
  const versions=await list(a,file);expect(versions.some(v=>v.name===first)).toBe(true);expect(versions.some(v=>v.kind==='restore')).toBe(true);expect(versions.some(v=>v.id===originalVersion.id&&v.digest===originalVersion.digest)).toBe(true);
  console.log('PASS named savepoint, preview, exact download, restore and clean peer reload:',kind);
  if(kind==='word'){
   await b.context().setOffline(true);const draft='UnsavedDraft'+Date.now();await typeIntoDocument(b,kind,draft);await expect.poll(()=>model(b,kind)).toContain(draft);
   await edit(a,kind,'NewCurrent'+Date.now());await history(a);await select(a,first);await restore(a);await b.context().setOffline(false);
   await expect(b.locator('.document-editor__status')).toContainText('Download your draft',{timeout:30000});
   expect(await model(b,kind)).toContain(draft);const d=b.waitForEvent('download',{timeout:120000});await b.getByRole('button',{name:'Download copy',exact:true}).click();const bytes=await readFile(await (await d).path());expect(docxText(bytes)).toContain(draft);
   await b.getByRole('button',{name:'Reload current version',exact:true}).click();await b.getByRole('button',{name:'Discard draft and reload',exact:true}).click();await waitForEditorReady(b);await expect.poll(()=>model(b,kind),{timeout:60000}).toEqual(approvedModel);
   expect((await readFile(join(files,file))).equals(approvedBytes)).toBe(true);console.log('PASS restored offline device retains downloadable draft and cannot overwrite restored file');
  }
  await close(a);await close(b);
 }
 const p=await device(),req=p.request,stamp=Date.now(),file=`History API ${stamp}.docx`,renamed=`Renamed history ${stamp}.docx`,folder=`History folder ${stamp}`,movedFolder=`Moved history ${stamp}`;
 const endpoint=(path,extra='')=>base+'/api/resources?source=My%20Drive&path='+encodeURIComponent('/'+path)+extra;
 const bytes=await readFile(join(files,'Quarterly report.docx'));
 expect((await req.post(endpoint(file),{data:bytes})).status()).toBe(200);
 const created=await list(p,file),wordVersions=await list(p,'Quarterly report.docx'),initial=wordVersions.find(v=>v.kind==='initial');
 const historic=await req.get(base+'/api/office/history-blob?source=My%20Drive&path='+encodeURIComponent('/Quarterly report.docx')+'&version='+initial.id+'&digest='+initial.digest);
 const previousBytes=await historic.body();expect((await req.put(endpoint(file),{data:previousBytes})).status()).toBe(200);
 const changed=await list(p,file);expect(changed.length).toBe(2);expect(changed.some(v=>v.id===created[0].id)).toBe(true);
 expect((await req.post(base+'/api/office/history-name?source=My%20Drive&path='+encodeURIComponent('/'+file),{data:{version:changed[0].id,digest:changed[0].digest,name:'Uploaded version'}})).status()).toBe(200);
 const move=async(from,to)=>{const r=await req.patch(base+'/api/resources',{data:{action:'move',overwrite:false,rename:false,items:[{fromSource:'My Drive',fromPath:'/'+from,toSource:'My Drive',toPath:'/'+to}]}});expect(r.status(),await r.text()).toBe(200);};
 await move(file,renamed);expect((await list(p,renamed))[0].id).toBe(changed[0].id);
 expect((await req.post(endpoint(folder,'&isDir=true'))).status()).toBe(200);await move(renamed,folder+'/'+renamed);await move(folder,movedFolder);
 const currentPath=movedFolder+'/'+renamed;expect((await list(p,currentPath))[0].name).toBe('Uploaded version');console.log('PASS ordinary upload/PUT and file/folder moves preserve version identity');
 const chunkName=`Chunk history ${stamp}.docx`,chunkHeaders={'X-File-Upload-Session':`history-${stamp}`,'X-File-Chunk-Offset':'0','X-File-Total-Size':String(bytes.length),'X-File-Size':String(bytes.length)};
 expect((await req.post(endpoint(chunkName),{data:bytes,headers:chunkHeaders})).status()).toBe(200);const oldChunk=await list(p,chunkName);
 expect((await req.delete(endpoint(chunkName))).status()).toBe(200);expect((await req.post(endpoint(chunkName),{data:bytes,headers:chunkHeaders})).status()).toBe(200);
 const newChunk=await list(p,chunkName);expect(newChunk).toHaveLength(1);expect(newChunk[0].id).not.toBe(oldChunk[0].id);console.log('PASS chunk upload and reused filename receive fresh document history');
 if(!process.env.OFFICE_TEST_PASSWORD_FILE){
  for(const allowModify of [true,false]){
   const r=await req.post(base+'/api/share',{data:{source:'My Drive',path:'/',title:'History verification',shareType:'normal',disableAnonymous:false,allowModify,allowCreate:allowModify,disableThumbnails:true}});expect(r.status()).toBe(200);const share=await r.json();
   try{const u=base+'/public/api/office/',q='?hash='+share.hash+'&path='+encodeURIComponent('/'+currentPath),v=(await list(p,currentPath))[0];
    expect((await req.post(u+'history-list'+q,{data:{}})).status()).toBe(allowModify?200:403);
    expect((await req.get(u+'history-blob'+q+'&version='+v.id+'&digest='+v.digest)).status()).toBe(allowModify?200:403);
    if(!allowModify){for(const action of ['history-name','history-restore'])expect((await req.post(u+action+q,{data:{}})).status()).toBe(403);
     await p.goto(base+'/public/share/'+share.hash+'/'+encodeURIComponent('Quarterly report.docx'));await waitForEditorReady(p);await expect(p.getByRole('button',{name:'Version history',exact:true})).toHaveCount(0);await close(p);
    }
   }finally{await req.delete(base+'/api/share?hash='+share.hash);}
  }
  console.log('PASS share history permissions and no history controls for viewers');
 }
 await req.delete(endpoint(chunkName));await req.delete(endpoint(movedFolder));
 console.log('PASS all Office version history browser checks');
}catch(e){for(const [i,c]of contexts.entries())for(const p of c.pages())await p.screenshot({path:join(screenshots,'history-failure-'+i+'.png')}).catch(()=>{});throw e;}finally{await browser.close();}
