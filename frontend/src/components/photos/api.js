// No router/store dependencies: the same client serves the integrated view,
// the Windows companion, and a future mobile client.
export function createPhotosApi({base='/',onUnauthorized=()=>{},filebrowserURL=''}={}) {
 let profile=null,modern=null;
 const scans=new Map(),grants=new Map();
 const endpoint=(route,params={})=>base.replace(/\/$/,'')+'/'+route+'?'+new URLSearchParams(params);
 async function request(route,params={},options={}) {
  const response=await fetch(endpoint(route,params),{credentials:'same-origin',...options});
  if(!response.ok){if(response.status===401)onUnauthorized();const message=await response.text();const error=new Error(message||`Request failed (${response.status})`);error.status=response.status;throw error;}
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
   let completed=0;for(const file of files){const target=fullPath(folder,file.name),session=crypto.randomUUID();let offset=0;
    do{const chunk=file.slice(offset,offset+5*1024*1024);const response=await fetch(endpoint('api/resources',{source,path:target}),{method:'POST',credentials:'same-origin',headers:{'Content-Type':'application/octet-stream','X-File-Chunk-Offset':String(offset),'X-File-Total-Size':String(file.size),'X-File-Size':String(file.size),'X-File-Upload-Session':session},body:chunk});
     if(!response.ok){const error=new Error(`${file.name}: ${await response.text()}`);error.status=response.status;throw error;}
     const result=await response.json();offset+=chunk.size;onProgress({name:file.name,completed,total:files.length,percent:file.size?Math.round(offset/file.size*100):100});if(result.complete===false&&offset===file.size)throw new Error('Upload was not finalized');
    }while(offset<file.size);completed++;onProgress({name:file.name,completed,total:files.length,percent:100});
   }scans.clear();
  },
 };
}
