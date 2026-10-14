import { fetchURL } from '@/api/utils.ts';
import { globalVars } from '@/utils/constants.js';

const delay = (ms) => new Promise(resolve => setTimeout(resolve, ms));

// Native ONLYOFFICE changes carry object positions and transforms. They are
// merged by the SDK, rather than replacing the other device's entire file.
export class OfficeCollaboration {
  constructor({ frame, source, path, share, edit, onState, onChange }) {
    Object.assign(this, { frame, source, path, share, edit, onState, onChange });
    this.session = null;
    this.sequence = 0;
    this.revision = 0;
    this.saved = 0;
    this.outbox = null;
    this.busy = false;
    this.stopped = false;
    this.changedAt = 0;
    this.lastInput = 0;
    this.sending = false;
    this.applying = false;
    this.lease = '';
    this.tasks = Promise.resolve();
    this.participants = [];
  }
  endpoint(action, extra = {}) {
    const query = new URLSearchParams({ path: this.path, ...(this.share ? {hash:this.share.hash} : {source:this.source}), ...extra });
    return `${globalVars.baseURL || '/'}${this.share ? 'public/' : ''}api/office/${action}?${query}`;
  }
  headers() {
    const password=this.share ? localStorage.getItem(`sharepass:${this.share.hash}`) : '';
    return password ? {'X-SHARE-PASSWORD':encodeURIComponent(password)} : {};
  }
  async request(action, data = {}, signal) {
    const response = await fetchURL(this.endpoint(action), {
      method:'POST', headers:{...this.headers(),'Content-Type':'application/json'},
      body:JSON.stringify({...this.session,...data}), signal:signal || AbortSignal.timeout(10000),
    });
    return response.json();
  }
  async join() {
    const state = await this.request('join');
    this.session = { room:state.room, client:state.client };
    const response = await fetchURL(this.endpoint('base', this.session), {headers:this.headers()});
    return response.arrayBuffer();
  }
  sdk() {
    const visit = (win) => {
      if (win.Asc?.editor) return win;
      for(let i=0;i<win.frames.length;i++){const found=visit(win.frames[i]);if(found)return found;}
    };
    return visit(this.frame.contentWindow);
  }
  async attach() {
    this.win = this.sdk();
    if (!this.win) throw new Error('Collaboration editor is unavailable');
    const api = this.api = this.win.Asc.editor;
    const co = api.CoAuthoringApi;
    const h = this.win.AscCommon.History;
    this.coll = api.collaborativeEditing || this.win.AscCommon.CollaborativeEditing;
    this.cell = Boolean(api.collaborativeEditing);
    this.slide = /presentationeditor/.test(this.win.location.pathname);
    if(this.cell){
      const haveChanges=api._haveChanges.bind(api);
      api._haveChanges=()=>haveChanges() || Boolean(api.wbModel.aCollaborativeActions?.some(items=>items.length));
    }
    this.original = Object.fromEntries(['saveChanges','askSaveChanges','unSaveLock','unLockDocument','sendCursor','getUserConnectionId'].map(k=>[k,co[k]]));
    co.onSetIndexUser(this.session.client);
    co.getUserConnectionId = () => this.session.client;
    api.DocInfo.get_UserId=()=>this.session.client;
    co.getParticipantName = id => this.participants.find(p=>p.id===id)?.name || 'Device';
    api.startCollaborationEditing();
    api.asc_SetFastCollaborative?.(true);
    api.autoSaveGap = api.autoSaveGapFast = api.autoSaveGapRealTime = 0;
    this.lastFlush=Date.now();
    this.inputListener = () => {
      const now=Date.now();
      if(now-this.lastInput>250)this.lastFlush=now;
      this.lastInput=now;
    };
    this.win.document.addEventListener('keydown', this.inputListener, true);
    this.win.document.addEventListener('input', this.inputListener, true);
    co.askSaveChanges = callback => {
      this.tasks = this.tasks.then(async()=>{
        try {
          while(this.checkpointing && !this.stopped)await delay(100);
          await this.sync();
          this.lease = (await this.request('acquire',{revision:this.revision})).lease;
          callback({saveLock:false});
        } catch(error) {
          callback({saveLock:true});
          if(error.status!==409)this.connectionError(error);
        }
      });
    };
    co.saveChanges = (changes, deleteIndex, extra) => {
      // Fast collaboration clears sent history; retain the exact batch until
      // the server confirms it, including across timeout/retry responses.
      if(this.outbox) throw new Error('An Office change batch is already pending');
      if(!changes.length){unlock();return;}
      this.outbox={batch:crypto.randomUUID(),lease:this.lease,changes,extra};
      this.changedAt=Date.now();
      this.onChange();
      // The SDK sets this flag after saveChanges returns and clears its sent
      // history. HTTP acknowledgement can take seconds: new input belongs in
      // fresh history while the exact sent batch stays in our outbox.
      queueMicrotask(()=>{h.CanNotAddChanges=false;});
      this.sendOutbox();
    };
    const unlock = () => {
      if(!this.checkpointing){
        if(this.lease) this.request('release',{lease:this.lease}).catch(()=>{});
        this.lease='';
      }
      h.CanNotAddChanges=false;
      co.onUnSaveLock?.();
      api.canSave=true;
    };
    co.unSaveLock=unlock;
    co.unLockDocument=unlock;
    co.sendCursor = cursor => {
      if(Date.now()-(this.lastCursor||0)<600)return;
      this.lastCursor=Date.now();
      this.request('cursor',{cursor}).catch(()=>{});
    };
    co.askLock=(blocks,callback)=>{
      if(!this.cell && !this.slide && !/pdfeditor/.test(this.win.location.pathname)){
        // Fast text collaboration uses the native positional merge rather
        // than a network lock before each paragraph/input action.
        queueMicrotask(()=>{
          callback?.({lock:blocks[0]});
          for(const block of blocks)co.onLocksAcquired?.({state:2,block,user:this.session.client});
        });
        return;
      }
      this.request('locks',{blocks}).then(()=>{
        callback?.({lock:blocks[0]});
        for(const block of blocks)co.onLocksAcquired?.({state:2,block,user:this.session.client});
      }).catch(()=>callback?.({error:'This part is being edited on another device. Try again after it saves.'}));
    };
    await this.sync();
    this.timer=setInterval(()=>this.tick(),300);
  }
  async apply(state) {
    this.applyChain=(this.applyChain || Promise.resolve()).catch(()=>{}).then(()=>this.applyState(state));
    return this.applyChain;
  }
  async applyState(state) {
    // Applying foreign operations can briefly block SDK input while fonts and
    // layout settle. Keep them queued through the current typing/composition
    // burst, including when synchronization was requested by native Save.
    if(state.events.some(e=>e.seq>this.sequence && e.type==='changes' && e.client!==this.session.client)){
      while(!this.stopped && Date.now()-this.lastInput<250)await delay(25);
    }
    for(const participant of this.participants){
      if(participant.id!==this.session.client && !state.participants.some(p=>p.id===participant.id))this.api.CoAuthoringApi.onCursor?.([{cursor:'',user:participant.id,useridoriginal:participant.id}]);
    }
    this.participants=state.participants;
    for(const event of state.events){
      if(event.seq<=this.sequence)continue;
      if(event.type==='changes' && event.client!==this.session.client){
        for(const change of event.changes) this.api.CoAuthoringApi.onSaveChanges(change,event.client,this.revision===0);
        if(event.extra) this.api.CoAuthoringApi.onRecalcLocks?.(JSON.stringify(event.extra));
      } else if(event.type==='cursor' && event.client!==this.session.client){
        this.api.CoAuthoringApi.onCursor?.([{cursor:event.cursor,user:event.client,useridoriginal:event.client}]);
      }else if((event.type==='locks' || event.type==='unlocks') && event.client!==this.session.client){
        for(const block of event.blocks){
          const lock={state:event.type==='locks'?3:0,block:typeof block==='object' ? (block.guid || block) : block,blockValue:block,user:event.client,time:Date.now()};
          if(event.type==='locks')this.api.CoAuthoringApi.onLocksAcquired?.(lock);
          else this.api.CoAuthoringApi.onLocksReleased?.(lock,true);
        }
      }
      if(event.type==='changes')this.changeSequence=event.seq;
      this.sequence=event.seq;
    }
    if(this.cell){
      this.coll.applyChanges();
      await this.settled();
    }else if(this.slide){
      this.coll.Apply_Changes();
      await this.settled();
    }else{
      await new Promise(resolve=>this.coll.Apply_Changes(resolve));
    }
    const changed=state.revision>this.revision;
    this.revision=Math.max(this.revision,state.revision);
    this.saved=Math.max(this.saved,state.saved);
    if(changed && this.revision>this.saved){this.changedAt=Date.now();this.onChange();}
    this.onState({connected:true,participants:this.participants,revision:this.revision,saved:this.saved});
  }
  async settled() {
    const deadline=Date.now()+15000;
    while(this.api.isLongAction?.() && Date.now()<deadline)await delay(30);
    if(this.api.isLongAction?.())throw new Error('Editor is still applying remote changes');
  }
  hasLocalChanges() {
    return this.win?.AscCommon.History.Have_Changes(true) ||
      Boolean(this.cell && this.api.wbModel.aCollaborativeActions?.some(items=>items.length));
  }
  async sync() {
    if(this.applying)return;
    this.applying=true;
    try{
      let state;
      try{state=await this.request('sync',{after:this.sequence});}
      catch(error){
        if(error.status!==403 && error.status!==409)throw error;
        this.sequence=this.changeSequence || 0;
        state=await this.request('resume',{after:this.sequence});
      }
      await this.apply(state);
    }finally{this.applying=false;}
  }
  async sendOutbox() {
    if(this.sending || !this.outbox)return;
    this.sending=true;
    try {
      const state=await this.request('changes',this.outbox);
      this.outbox=null;
      this.lease='';
      this.win.AscCommon.History.CanNotAddChanges=false;
      this.api.CoAuthoringApi.onUnSaveLock?.();
      this.api.canSave=true;
      await this.apply(state);
    }catch(error){this.connectionError(error);}finally{this.sending=false;}
  }
  connectionError(error) {
    this.onState({connected:false,error:error.message,participants:this.participants,revision:this.revision,saved:this.saved});
  }
  async tick() {
    if(this.busy || this.stopped || this.checkpointing)return;
    this.busy=true;
    try {
      if(this.outbox){await this.sendOutbox();return;}
      // Allow the current typing/composition action to settle before the SDK
      // applies foreign changes and collects its own operation history.
      if(Date.now()-this.lastInput<250){
        // Keep the session alive while a cell/composition is still being edited.
        if(Date.now()-(this.heartbeat||0)>10000){this.heartbeat=Date.now();await this.request('sync',{after:this.sequence});}
        if(!this.cell && !this.slide && Date.now()-(this.lastFlush||0)>1000 && this.edit && this.api.canSave && !this.api.isLongAction?.() && this.hasLocalChanges()){
          this.lastFlush=Date.now();this.api.asc_Save(true);
        }
        return;
      }
      await this.sync();
      if(this.edit && !this.api.isLongAction?.() && this.api.canSave && this.hasLocalChanges())this.api.asc_Save(true);
      if(this.edit && !this.cell && !this.api.isLongAction?.())this.api.WordControl?.m_oLogicDocument?.Continue_FastCollaborativeEditing?.();
      if(this.edit && this.cell && !this.api.isLongAction?.())this.api.wb?.sendCursor?.();
    }catch(error){this.connectionError(error);}finally{this.busy=false;}
  }
  async prepareCheckpoint() {
    for(let attempt=0;attempt<100;attempt++){
      await this.tick();
      await this.tasks;
      if(this.outbox || !this.api.canSave || this.applying || this.hasLocalChanges()){await delay(100);continue;}
      await this.sync();
      try{
        this.lease=(await this.request('acquire',{revision:this.revision})).lease;
        this.checkpointRevision=this.revision;
        this.checkpointing=true;
        this.renewTimer=setInterval(()=>this.request('acquire',{revision:this.checkpointRevision}).catch(()=>{}),5000);
        return;
      }catch(error){if(error.status!==409)throw error;await delay(150);}
    }
    throw new Error('Changes are still synchronizing. Keep this document open and try Save again.');
  }
  async checkpoint(file) {
    const extra={...this.session,lease:this.lease,revision:String(this.checkpointRevision)};
    try{
      await fetchURL(this.endpoint('checkpoint',extra),{method:'POST',headers:this.headers(),body:file});
      this.saved=this.checkpointRevision;
      await this.sync();
    }finally{
      this.checkpointing=false;
      clearInterval(this.renewTimer);
      await this.request('release',{lease:this.lease}).catch(()=>{});
      this.lease='';
    }
  }
  get pending() {return Boolean(this.outbox || this.revision>this.saved);}
  cancelCheckpoint() {
    this.checkpointing=false;clearInterval(this.renewTimer);
    if(this.lease)this.request('release',{lease:this.lease}).catch(()=>{});
    this.lease='';
  }
  async stop() {
    this.stopped=true;clearInterval(this.timer);
    this.cancelCheckpoint();
    this.win?.document.removeEventListener('keydown',this.inputListener,true);
    this.win?.document.removeEventListener('input',this.inputListener,true);
    if(this.session)await this.request('leave').catch(()=>{});
  }
}
