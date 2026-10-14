<template>
  <div class="office-history" role="region" aria-label="Document version history">
    <div class="office-history__preview">
      <div class="office-history__preview-bar">
        <span>{{ selected?.name || (selected ? versionDate(selected.created) : 'Select a version') }}</span>
        <span role="status">{{ previewStatus }}</span>
        <button v-if="selected" type="button" class="button button--flat" :disabled="busy || !previewReady" @click="download">Download this version</button>
        <button v-if="selected && selected.id !== versions[0]?.id" type="button" class="button" :disabled="busy || !previewReady" @click="confirmRestore = true">Restore this version</button>
      </div>
      <iframe v-if="selected" :key="selected.id + selected.digest" ref="previewFrame" class="office-history__frame" :src="editorUrl" :title="'Version preview of ' + fileName" />
      <div v-else class="office-history__empty">Select a saved version to preview it.</div>
    </div>
    <aside class="office-history__sidebar">
      <div class="office-history__heading"><h2>Version history</h2><button class="button button--flat" type="button" aria-label="Close version history" :disabled="busy" @click="$emit('close')">Close</button></div>
      <label class="office-history__filter"><input v-model="namedOnly" type="checkbox" /> Only show named versions</label>
      <button type="button" class="button button--flat" :disabled="busy" @click="refresh">Refresh history</button>
      <p v-if="error" class="office-history__error" role="alert">{{ error }}</p>
      <p v-if="loading" role="status">Loading versions…</p>
      <ol class="office-history__timeline">
        <li v-for="version in visibleVersions" :key="version.id" :class="{ 'office-history__selected': selected?.id === version.id }">
          <button class="office-history__version" type="button" :disabled="busy" @click="select(version)">
            <strong>{{ version.name || versionDate(version.created) }}</strong>
            <small v-if="version.name">{{ versionDate(version.created) }}</small>
            <span v-if="version.id === versions[0]?.id" class="office-history__current">Current version</span>
            <span v-if="version.kind === 'initial'">Original file</span>
            <span v-else-if="version.kind === 'restore'">Restored version</span>
            <span v-else-if="version.kind === 'external'">Changed outside the editor</span>
            <span v-for="author in version.authors" :key="author" class="office-history__author"><i :style="{ background: authorColor(author) }" />{{ author }}</span>
          </button>
          <button class="button button--flat office-history__name" type="button" :disabled="busy" @click="startName(version)">{{ version.name ? 'Rename version' : 'Name this version' }}</button>
        </li>
      </ol>
      <p v-if="!loading && !visibleVersions.length" class="office-history__empty">{{ namedOnly ? 'No named versions yet.' : 'No saved versions yet.' }}</p>
      <form v-if="naming" class="office-history__dialog" aria-label="Name a version" @submit.prevent="saveName">
        <label>Version name<input v-model.trim="versionName" aria-label="Version name" maxlength="120" autofocus /></label>
        <p>Named versions are kept and no longer merged with autosaves. Leave the name empty to remove it.</p>
        <button type="submit" class="button" :disabled="busy">Save version name</button>
        <button type="button" class="button button--flat" :disabled="busy" @click="naming = null">Cancel</button>
      </form>
      <div v-if="confirmRestore" class="office-history__dialog" role="alertdialog" aria-label="Restore this version">
        <h3>Restore this version?</h3>
        <p>The current saved document will remain in history. Other open devices will reload the restored document or keep their unsaved edits as a draft.</p>
        <button class="button" type="button" :disabled="busy" @click="restore">Confirm restore</button>
        <button class="button button--flat" type="button" :disabled="busy" @click="confirmRestore = false">Cancel</button>
      </div>
    </aside>
  </div>
</template>

<script>
import { fetchURL } from '@/api/utils.ts';
import { globalVars } from '@/utils/constants.js';

export default {
  name:'OfficeVersionHistory',
  props:{controller:{type:Object,required:true},fileName:{type:String,required:true},restoring:{type:Boolean,default:false}},
  emits:['close','restore'],
  data:()=>({versions:[],selected:null,buffer:null,bridgeReady:false,previewReady:false,previewStatus:'',loading:false,working:false,error:'',namedOnly:false,naming:null,versionName:'',confirmRestore:false,selectionToken:0}),
  computed:{
    busy(){return this.working || this.restoring;},
    visibleVersions(){return this.namedOnly ? this.versions.filter(v=>v.name) : this.versions;},
    editorUrl(){const query=new URLSearchParams({embed:'1',embedOrigin:window.location.origin});return `${globalVars.baseURL||'/'}editor?${query}`;},
  },
  mounted(){window.addEventListener('message',this.onMessage);void this.refresh();},
  beforeUnmount(){this.selectionToken++;window.removeEventListener('message',this.onMessage);},
  methods:{
    versionDate(value){return new Intl.DateTimeFormat(undefined,{dateStyle:'medium',timeStyle:'short'}).format(new Date(value));},
    authorColor(name){let hash=0;for(const char of name)hash=(hash*31+char.charCodeAt(0))>>>0;return ['#1a73e8','#188038','#a142f4','#d93025','#e37400'][hash%5];},
    async refresh(){this.loading=true;this.error='';try{const result=await this.controller.request('history-list');this.versions=result.versions;const match=this.versions.find(v=>v.id===this.selected?.id);if(!this.selected && this.versions.length)await this.select(this.versions[0]);else if(match){if(match.digest!==this.selected.digest)await this.select(match);else this.selected=match;}}catch(error){this.error=error.message;}finally{this.loading=false;}},
    async select(version){
      const token=++this.selectionToken;this.selected=version;this.bridgeReady=false;this.previewReady=false;this.previewStatus='Loading read-only preview…';this.buffer=null;this.error='';
      try{const response=await fetchURL(this.controller.endpoint('history-blob',{version:version.id,digest:version.digest}),{headers:this.controller.headers()});const buffer=await response.arrayBuffer();if(token!==this.selectionToken)return;this.buffer=buffer;if(this.bridgeReady)this.openPreview();}
      catch(error){if(token===this.selectionToken){this.previewStatus='Preview unavailable';this.error=error.message;}}
    },
    openPreview(){if(!this.buffer || !this.bridgeReady)return;this.$refs.previewFrame?.contentWindow.postMessage({id:`history-${this.selectionToken}`,type:'document:open-buffer',payload:{fileName:this.fileName.replace(/\.[^.]+$/,'')+this.selected.format,buffer:this.buffer,readonly:true}},window.location.origin);},
    onMessage(event){if(event.origin!==window.location.origin||event.source!==this.$refs.previewFrame?.contentWindow)return;if(event.data?.type==='document:ready'){this.bridgeReady=true;this.openPreview();}else if(event.data?.type==='document:loaded'){this.$refs.previewFrame.contentWindow.postMessage({type:'document:set-readonly',payload:{readonly:true}},window.location.origin);this.previewReady=true;this.previewStatus='Read-only version';}else if(event.data?.type==='document:error'){this.previewStatus='Preview unavailable';this.error=event.data.payload?.message||'Could not load this version';}},
    startName(version){this.naming=version;this.versionName=version.name;},
    async saveName(){this.working=true;this.error='';try{await this.controller.request('history-name',{version:this.naming.id,digest:this.naming.digest,name:this.versionName});this.naming=null;await this.refresh();}catch(error){this.error=error.message;}finally{this.working=false;}},
    async download(){this.working=true;this.error='';try{const response=await fetchURL(this.controller.endpoint('history-blob',{version:this.selected.id,digest:this.selected.digest}),{headers:this.controller.headers()});const href=URL.createObjectURL(await response.blob());const link=document.createElement('a');link.href=href;const label=(this.selected.name||new Date(this.selected.created).toISOString()).replace(/[\\/:*?"<>|]/g,'-');link.download=this.fileName.replace(/\.[^.]+$/,'')+` (${label})`+this.selected.format;link.click();setTimeout(()=>URL.revokeObjectURL(href),1000);}catch(error){this.error=error.message;}finally{this.working=false;}},
    restore(){this.confirmRestore=false;this.$emit('restore',this.selected);},
  },
};
</script>

<style scoped>
.office-history{display:flex;flex:1;min-height:0;background:var(--background);color:var(--textPrimary)}
.office-history__preview{display:flex;flex:1;min-width:0;flex-direction:column}
.office-history__preview-bar{display:flex;align-items:center;gap:.6rem;flex-wrap:wrap;padding:.5rem .8rem;border-bottom:1px solid var(--divider);background:var(--panel-bg);font-size:.85rem}
.office-history__preview-bar>span:first-child{font-weight:600;margin-right:auto}
.office-history__frame{flex:1;width:100%;min-height:0;border:0}
.office-history__sidebar{width:19rem;flex:0 0 19rem;overflow-y:auto;border-left:1px solid var(--divider);padding:1rem;background:var(--panel-bg)}
.office-history__heading{display:flex;align-items:center;justify-content:space-between;gap:.5rem}.office-history h2{font-size:1.1rem;margin:0}
.office-history__filter{display:flex;gap:.5rem;align-items:center;margin:1rem 0;font-size:.85rem}
.office-history__timeline{padding:0;list-style:none;margin:1rem 0}.office-history__timeline>li{border-left:3px solid var(--divider);padding:.2rem .4rem .6rem .6rem;margin-bottom:.35rem;border-radius:0 .6rem .6rem 0}
.office-history__timeline>li.office-history__selected{background:var(--surface-alt,#e8f0fe);border-left-color:#1a73e8}
.office-history__version{display:flex;flex-direction:column;gap:.3rem;text-align:left;width:100%;padding:.5rem;border:0;background:none;color:inherit;cursor:pointer;font:inherit;font-size:.85rem}
.office-history__version small,.office-history__author{color:var(--textSecondary)}.office-history__author{display:flex;gap:.4rem;align-items:center}.office-history__author i{display:inline-block;width:.55rem;height:.55rem;border-radius:50%}
.office-history__current{color:#188038;font-weight:600}.office-history__name{font-size:.75rem}
.office-history__dialog{border:1px solid var(--divider);padding:1rem;border-radius:.6rem;margin-top:1rem;font-size:.85rem}.office-history__dialog label{display:flex;flex-direction:column;gap:.5rem}.office-history__dialog input{width:100%;box-sizing:border-box;padding:.6rem;border:1px solid var(--divider);border-radius:.3rem}.office-history__dialog h3{margin-top:0}.office-history__dialog button{margin-top:.5rem}
.office-history__error{color:#b3261e}.office-history__empty{padding:1rem;color:var(--textSecondary);font-size:.85rem}
@media(max-width:800px){.office-history{flex-direction:column}.office-history__preview{min-height:50vh}.office-history__sidebar{width:auto;flex:0 0 16rem;border-left:0;border-top:1px solid var(--divider)}}
</style>
