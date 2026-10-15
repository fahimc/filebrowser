import {httpErrorMessage} from '../../utils/httpError.js';
function duplicateName(name,attempt){
 const dot=name.lastIndexOf('.'),extension=dot>0?name.slice(dot):'',stem=dot>0?name.slice(0,dot):name;
 const match=stem.match(/^(.*) \((\d+)\)$/),number=match?Number(match[2]):0;
 const increment=match&&Number.isSafeInteger(number)&&number<Number.MAX_SAFE_INTEGER-1000;
 return `${increment?match[1]:stem} (${(increment?number:0)+attempt})${extension}`;
}
// No router/store dependencies: the same client serves the integrated view,
// the Windows companion, and a future mobile client.
export function createPhotosApi({base='/',onUnauthorized=()=>{},filebrowserURL=''}={}) {
 let profile=null,modern=null;
 const scans=new Map(),grants=new Map(),uploads=new WeakMap();
 const endpoint=(route,params={})=>base.replace(/\/$/,'')+'/'+route+'?'+new URLSearchParams(params);
 async function request(route,params={},options={}) {
  const response=await fetch(endpoint(route,params),{credentials:'same-origin',...options});
  if(!response.ok){if(response.status===401)onUnauthorized();const error=new Error(httpErrorMessage(response.status,await response.text(),response.headers.get('Content-Type')||''));error.status=response.status;throw error;}
  return response.status===204?null:response.json();
 }
 const id=(source,path)=>btoa(String.fromCharCode(...new TextEncoder().encode(source+'\0'+path)));
 const fullPath=(folder,name)=>(folder.replace(/\/$/,'')+'/'+name).replace(/\/+/g,'/');
 async function metadata(item){return request('api/resources',{source:item.source,path:item.path});}
 function scopes(){return profile?.scopes||[];}
 function permissions(source){return scopes().find(s=>s.name===source)?.permissions||profile?.permissions||{};}
 async function fallbackPage(params){
  const key=JSON.stringify([params.source,params.path||'/']);let scan=scans.get(key);
  if(!scan||!params.cursor){scan={queue:[params.path||'/'],seen:new Set(),items:[],done:false};scans.set(key,scan);}
  const matches=x=>(!params.kind||x.kind===params.kind)&&(!params.q||x.name.toLowerCase().includes(params.q.toLowerCase()))&&(!params.favorites||x.favorite);
  const offset=Number(params.cursor?.replace('legacy:','')||0),limit=100;
  let visits=0;
  while(!scan.done&&scan.items.filter(matches).length<offset+limit&&visits++<40){
   const folder=scan.queue.shift();if(!folder){scan.done=true;break;}if(scan.seen.has(folder))continue;scan.seen.add(folder);
   let listing;try{listing=await request('api/resources',{source:params.source,path:folder});}catch(error){if([403,404].includes(error.status))continue;throw error;}
   for(const dir of listing.folders||[])scan.queue.push(fullPath(folder,dir.name));
   for(const file of listing.files||[]){const kind=file.type?.startsWith('video/')?'video':file.type?.startsWith('image/')||/\.(heic|heif)$/i.test(file.name)?'image':null;if(!kind)continue;
    const path=fullPath(folder,file.name);scan.items.push({...file,id:id(params.source,path),source:params.source,path,kind,favorite:(listing.pinnedItems||[]).includes(file.name),permissions:permissions(params.source)});
   }
   if(!scan.queue.length)scan.done=true;
  }
  const found=scan.items.filter(matches).sort((a,b)=>(Date.parse(b.modified)||0)-(Date.parse(a.modified)||0)||a.path.localeCompare(b.path));
  // Offset is used only once the folder scan completes. Until then the UI
  // deduplicates the current prefix by stable ID, so newly found dates cannot
  // make older items disappear between compatibility pages.
  return {items:scan.done?found.slice(offset,offset+limit):found,nextCursor:scan.done?(found.length>offset+limit?'legacy:'+(offset+limit):''):'legacy:0',dateBasis:'modified',compatibility:true};
 }
 return {
  endpoint,filebrowserURL,
  async profile(){profile=await request('api/users',{username:'self'});return profile;},
  async sources(){const data=await request('api/settings/sources');return Object.keys(data||{});},
  async page(params){
   if(modern===null){try{await request('api/photos/capabilities');modern=true;}catch(error){if(error.status!==404)throw error;modern=false;}}
   if(modern){try{return await request('api/photos/library',{...params,favorites:params.favorites?'true':'false'});}catch(error){if(error.status!==501)throw error;modern=false;}}
   return fallbackPage(params);
  },
  thumbnail(item){return endpoint('api/resources/preview',{source:item.source,path:item.path,size:'small',v:item.modified||'',account:profile?.id||''});},
  async media(item){
   if(modern)return endpoint('api/photos/media',{source:item.source,path:item.path});
   const key=item.id+item.modified;let grant=grants.get(key);if(!grant||grant.expires<Date.now()){const info=await metadata(item);grant={token:info.viewToken,expires:Date.now()+600000};grants.set(key,grant);}
   return endpoint(item.kind==='video'?'api/media/stream':'api/resources/view',{source:item.source,file:item.path,viewToken:grant.token});
  },
  async favorite(item,value){const parts=item.path.split('/'),name=parts.pop(),folder=parts.join('/')||'/';await request('api/users/pinned-items',{action:value?'add':'remove'},{method:'PATCH',headers:{'Content-Type':'application/json'},body:JSON.stringify({source:item.source,path:folder,name})});item.favorite=value;profile=await request('api/users',{username:'self'});},
  folders(source,path='/'){return request('api/resources/items',{source,path,only:'folders'});},
  download(item){return endpoint('api/resources/download',{source:item.source,file:item.path});},
  browse(item){return filebrowserURL.replace(/\/$/,'')+'/files/'+encodeURIComponent(item.source)+item.path.split('/').map(encodeURIComponent).join('/');},
  async upload(source,folder,files,onProgress){
   let completed=0;const saved=[];
   try{for(const file of files){
    let progress=uploads.get(file);
    if(!progress||progress.source!==source||progress.folder!==folder){progress={source,folder,path:fullPath(folder,file.name),name:file.name,session:crypto.randomUUID(),offset:0,complete:false,checked:false,copies:0};uploads.set(file,progress);}
    if(progress.complete){completed++;continue;}
    const rename=()=>{
     if(++progress.copies>1000){const error=new Error('There are too many files with this name. Choose a different name and upload it again.');error.status=409;throw error;}
     progress.name=duplicateName(file.name,progress.copies);progress.path=fullPath(folder,progress.name);progress.session=crypto.randomUUID();progress.offset=0;progress.checked=false;
     onProgress({name:progress.name,originalName:file.name,completed,total:files.length,phase:'renaming',percent:0});
    };
    while(!progress.complete){
     // Check names before sending bytes; POST still checks conflicts atomically
     // and the 409 branch below handles another uploader choosing that name.
     if(!progress.checked){
      let exists=true;
      try{await request('api/resources',{source,path:progress.path});}catch(error){if(error.status===404)exists=false;else throw error;}
      if(exists){rename();continue;}
      progress.checked=true;
     }
     const offset=progress.offset,chunk=file.slice(offset,offset+5*1024*1024);
     try{await new Promise((resolve,reject)=>{
      const xhr=new XMLHttpRequest();xhr.open('POST',endpoint('api/resources',{source,path:progress.path,_chunkSeq:offset}));xhr.withCredentials=true;xhr.timeout=120000;
      for(const [key,value]of Object.entries({'Content-Type':'application/octet-stream','X-File-Chunk-Offset':String(offset),'X-File-Total-Size':String(file.size),'X-File-Size':String(file.size),'X-File-Upload-Session':progress.session}))xhr.setRequestHeader(key,value);
      const report=(loaded,phase)=>onProgress({name:progress.name,originalName:file.name,completed,total:files.length,phase,percent:file.size?Math.min(99,Math.round((offset+loaded)/file.size*100)):0});
      report(0,'uploading');xhr.upload.onprogress=e=>{if(e.lengthComputable)report(e.loaded,e.loaded===e.total?'saving':'uploading');};
      const fail=(status,message)=>{const error=new Error(message);error.status=status;reject(error);};
      xhr.onload=()=>{
       if(xhr.status<200||xhr.status>=300){fail(xhr.status,httpErrorMessage(xhr.status,xhr.responseText,xhr.getResponseHeader('Content-Type')||''));return;}
       try{const value=JSON.parse(xhr.responseText);if(value.offset!==offset+chunk.size||value.total!==file.size||value.complete!==(offset+chunk.size===file.size))throw Error('unconfirmed');resolve(value);}catch{fail(502,'The server did not confirm this upload. Refresh the library before retrying. No existing file will be overwritten.');}
      };
      xhr.onerror=()=>fail(0,httpErrorMessage(0));xhr.ontimeout=()=>fail(504,httpErrorMessage(504));xhr.onabort=()=>fail(0,'The upload was interrupted. Keep this page open to retry.');xhr.send(chunk);
     });}catch(error){if(error.status===409){rename();continue;}throw error;}
     progress.offset+=chunk.size;
     progress.complete=progress.offset===file.size;
     onProgress({name:progress.name,originalName:file.name,completed,total:files.length,phase:'saved',percent:file.size?Math.round(progress.offset/file.size*100):100});
    }
    saved.push({name:progress.name,path:progress.path,renamed:progress.name!==file.name});
    completed++;onProgress({name:progress.name,originalName:file.name,completed,total:files.length,phase:'saved',percent:100});
   }}catch(error){error.remainingFiles=files.slice(completed);error.completed=completed;scans.clear();throw error;}
   scans.clear();return saved;
  },
 };
}
