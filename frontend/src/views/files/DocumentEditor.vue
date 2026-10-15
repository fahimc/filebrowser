<template>
  <section class="document-editor" :class="{ 'document-editor--prompt': isPrompt }">
    <div class="document-editor__toolbar">
      <img :src="officeLogo" alt="My Office" width="32" height="32" style="flex:none" />
      <input
        v-if="isNewDocument"
        v-model.trim="newFileName"
        class="document-editor__filename"
        type="text"
        aria-label="New document filename"
        :disabled="createdInStorage || saving"
        @input="onFileNameChange"
      />
      <strong v-else class="document-editor__title">{{ fileName }}</strong>
      <span class="document-editor__status" role="status">{{ statusText }}</span>
      <span v-if="collaboration" class="document-editor__presence" :title="participants.map(p => p.name).join(', ')">{{ connected ? 'Live' : 'Reconnecting…' }} · {{ participants.length }} {{ participants.length === 1 ? 'device' : 'devices' }}</span>
      <button v-if="collaboration && canEdit" type="button" class="button button--flat" :disabled="!editorReady || saving || restorePending || restoringVersion" @click="openHistory">Version history</button>
      <button v-if="restorePending" type="button" class="button" :disabled="Boolean(copyCommandId)" @click="confirmReload = true">Reload current version</button>
      <button v-if="collaboration && dirty" type="button" class="button button--flat" :disabled="!editorReady || saving || Boolean(copyCommandId)" @click="downloadCopy">Download copy</button>
      <a class="document-editor__legal" href="https://github.com/fahimc/filebrowser/blob/drive-reskin/third_party/NOTICES.md" target="_blank" rel="noopener noreferrer">Source &amp; licenses</a>
      <button
        type="button"
        class="button button--flat document-editor__save"
        :disabled="!canEdit || restorePending || (!dirty && !isNewDocument) || saving || !editorReady"
        @click="requestSave"
      >
        {{ $t('general.save') }}
      </button>
      <button
        type="button"
        class="button button--flat"
        :aria-label="$t('general.close')"
        @click="close"
      >
        {{ $t('general.close') }}
      </button>
    </div>
    <div v-if="confirmReload" class="document-editor__restore-confirm" role="alertdialog" aria-label="Reload restored document">
      <p>Reloading replaces this tab's unsaved draft with the current saved document. Download a copy first to keep your edits.</p>
      <button type="button" class="button button--flat" :disabled="Boolean(copyCommandId)" @click="downloadCopy">Download copy</button>
      <button type="button" class="button" :disabled="Boolean(copyCommandId)" @click="reloadRestored">Discard draft and reload</button>
      <button type="button" class="button button--flat" @click="confirmReload = false">Cancel</button>
    </div>
    <iframe
      v-show="!historyVisible"
      ref="editorFrame"
      class="document-editor__frame"
      :src="editorUrl"
      :title="fileName"
      allow="clipboard-read; clipboard-write"
      :inert="restorePending"
    />
    <OfficeVersionHistory v-if="historyVisible" :controller="collaboration" :file-name="fileName" :restoring="restoringVersion" @close="closeHistory" @restore="restoreVersion" />
  </section>
</template>

<script>
import officeLogo from "@/assets/branding/my_office.svg";
import { resourcesApi } from "@/api";
import { getters, mutations, state } from "@/store";
import { globalVars } from "@/utils/constants.js";
import { url } from "@/utils";
import { removeLastDir } from "@/utils/url.js";
import { notify } from "@/notify";
import { router } from "@/router";
import { createBlankPDF } from "@/utils/office.js";
import { OfficeCollaboration } from "@/utils/officeCollaboration.js";
import { markRaw } from "vue";
import OfficeVersionHistory from '@/components/office/OfficeVersionHistory.vue';

const SAVE_DELAY_MS = 1800;
const OFFICE_TYPES = new Set(["docx", "xlsx", "pptx", "pdf"]);

export default {
  name: "documentEditor",
  components:{OfficeVersionHistory},
  props: {
    fbdata: { type: Object, default: null },
    promptId: { type: [String, Number], default: null },
    newType: { type: String, default: "" },
    source: { type: String, default: "" },
    parentPath: { type: String, default: "" },
  },
  data() {
    return {
      editorReady: false,
      bridgeReady: false,
      documentOpened: false,
      dirty: false,
      saving: false,
      statusText: "",
      newFileName: this.newType ? `Untitled.${this.newType}` : "",
      targetPath: "",
      pendingCommandId: "",
      saveTimer: null,
      documentBuffer: null,
      createdInStorage: false,
      dirtyRevision: 0,
      saveRequestedAt: 0,
      editorDirty: false,
      originalFile: { ...(this.fbdata || state.req || {}) },
      shareContext: getters.isShare() ? { ...state.shareInfo } : null,
      saveCompletion: null,
      completeSave: null,
      saveTimeout: null,
      removeNavigationGuard: null,
      collaboration: null,
      participants: [],
      connected: false,
      collaborationAttaching: false,
      activateAfterSave: false,
      copyCommandId: '',
      destroyed: false,
      historyVisible: false,
      restoringVersion: false,
      restorePending: false,
      confirmReload: false,
    };
  },
  computed: {
    officeLogo: () => officeLogo,
    isPrompt() {
      return this.promptId !== null && this.promptId !== undefined;
    },
    isNewDocument() {
      return OFFICE_TYPES.has(String(this.newType).toLowerCase());
    },
    file() {
      return this.originalFile;
    },
    fileName() {
      return this.isNewDocument ? this.newFileName : (this.file.name || "Office document");
    },
    editorUrl() {
      const params = new URLSearchParams({ embed: "1", embedOrigin: window.location.origin });
      if (this.isNewDocument && !['pdf','pptx','docx'].includes(this.newType)) params.set("new", this.newType.toLowerCase());
      return `${globalVars.baseURL || "/"}editor?${params.toString()}`;
    },
    isShare() {
      return Boolean(this.shareContext);
    },
    saveSource() {
      return this.source || this.file.source || state.sources.current;
    },
    canEdit() {
      if (this.isNewDocument) {
        return this.isShare
          ? Boolean(this.createdInStorage ? this.shareContext.allowModify : this.shareContext.allowCreate)
          : Boolean(getters.sourcePermissions(this.saveSource)[this.createdInStorage ? 'modify' : 'create']);
      }
      return this.isShare
        ? Boolean(this.shareContext.allowModify)
        : Boolean(getters.sourcePermissions(this.file.source).modify);
    },
  },
  mounted() {
    window.addEventListener("message", this.onEditorMessage);
    window.addEventListener("beforeunload", this.beforeUnload);
    this.removeNavigationGuard = router.beforeEach(() => this.flushSave());
    if (this.isNewDocument) {
      this.statusText = "New document";
      if (this.newType === 'pdf') this.documentBuffer = createBlankPDF();
      else if (['pptx','docx'].includes(this.newType)) this.loadNewTemplate().catch(error => this.showError(error));
      return;
    }
    this.openExistingFile().catch((error) => this.showError(error));
  },
  beforeUnmount() {
    this.destroyed=true;
    window.removeEventListener("message", this.onEditorMessage);
    window.removeEventListener("beforeunload", this.beforeUnload);
    this.removeNavigationGuard?.();
    clearTimeout(this.saveTimer);
    clearTimeout(this.saveTimeout);
    void this.collaboration?.stop();
  },
  methods: {
    async loadNewTemplate() {
      const response = await fetch(`${globalVars.baseURL || '/'}public/static/templates/blank-en.${this.newType}`);
      if (!response.ok) throw new Error('Could not load the new document template. Please retry.');
      this.documentBuffer = await response.arrayBuffer();
      if (this.destroyed) return;
      if (this.bridgeReady) this.openEditorBuffer();
    },
    editorOrigin() {
      return window.location.origin;
    },
    sendCommand(type, payload = {}) {
      const frame = this.$refs.editorFrame;
      if (!frame?.contentWindow) return "";
      const id = `${Date.now()}-${Math.random().toString(16).slice(2)}`;
      frame.contentWindow.postMessage({ id, type, payload }, this.editorOrigin());
      return id;
    },
    async openExistingFile() {
      await this.startCollaboration(this.file.path);
      return;
    },
    async startCollaboration(path) {
      this.editorReady=false;
      this.statusText='Loading document…';
      if(this.bridgeReady)this.sendCommand('document:set-readonly',{readonly:true});
      await this.collaboration?.stop();
      this.collaboration = markRaw(new OfficeCollaboration({
        frame: this.$refs.editorFrame, source:this.saveSource, path,
        share:this.shareContext, edit:this.canEdit,
        onState:({connected,participants,revision,saved,error})=>{
          const reconnected=connected && !this.connected;
          this.connected=connected;this.participants=participants;
          if(this.restorePending)return;
          if(!connected){clearTimeout(this.saveTimer);this.statusText=`Reconnecting… ${error || ''}`;return;}
          if(reconnected && this.dirty && !this.saving)this.scheduleSave();
          if(!this.saving && !this.dirty && revision<=saved)this.statusText=this.canEdit ? (saved>0 || this.createdInStorage ? 'All changes saved' : 'Ready') : 'View only';
          if(!this.saving && this.dirty && revision<=saved && !this.collaboration.outbox && !this.collaboration.hasLocalChanges()){
            this.dirty=false;this.statusText='All changes saved';
          }
        },
        onChange:()=>{if(this.canEdit){this.dirty=true;this.dirtyRevision++;if(!this.saving)this.scheduleSave();}},
        onRestored:()=>this.onRemoteRestore(),
      }));
      this.documentBuffer=await this.collaboration.join();
      if(this.destroyed){await this.collaboration.stop();return;}
      this.documentOpened=false;
      this.editorReady=false;
      if(this.bridgeReady)this.openEditorBuffer();
    },
    openEditorBuffer() {
      if (!this.documentBuffer || this.documentOpened) return;
      this.documentOpened = true;
      this.sendCommand("document:open-buffer", {
        fileName: this.fileName,
        buffer: this.documentBuffer,
        readonly: !this.canEdit,
      });
      this.statusText = "Loading document…";
    },
    onEditorMessage(event) {
      if (event.origin !== this.editorOrigin() || event.source !== this.$refs.editorFrame?.contentWindow) return;
      const { id, type, payload = {} } = event.data || {};
      if (typeof type !== "string" || !type.startsWith("document:")) return;
      if (type === "document:ready") {
        this.bridgeReady = true;
        this.openEditorBuffer();
      } else if (type === "document:opened") {
        this.documentOpened = true;
        if (!this.editorReady) this.statusText = "Loading document…";
      } else if (type === "document:loaded") {
        this.editorReady = true;
        this.statusText = this.canEdit ? (this.createdInStorage ? "All changes saved" : "Ready") : "View only";
        if(this.collaboration && !this.collaboration.win && !this.collaborationAttaching){
          this.collaborationAttaching=true;
          this.collaboration.attach().catch(error=>this.showError(error)).finally(()=>{this.collaborationAttaching=false;});
        }
        if (this.dirty && this.canEdit) this.scheduleSave();
      } else if (type === "document:dirty-changed") {
        this.editorDirty = Boolean(payload.dirty);
        if (this.editorDirty && this.canEdit) {
          this.dirty = true;
          this.dirtyRevision += 1;
          this.statusText = "Unsaved changes";
          this.scheduleSave();
        }
      } else if (type === "document:saved") {
        if(this.copyCommandId && id===this.copyCommandId){
          this.copyCommandId='';
          if(this.restorePending)this.sendCommand('document:set-readonly',{readonly:true});
          const href=URL.createObjectURL(payload.file);
          const link=document.createElement('a');link.href=href;link.download=this.fileName;link.click();
          setTimeout(()=>URL.revokeObjectURL(href),1000);
        } else if (this.pendingCommandId && id === this.pendingCommandId) {
          this.persistDocument(payload).catch((error) => this.showError(error));
        }
      } else if (type === "document:error") {
        if(this.copyCommandId && id===this.copyCommandId){this.copyCommandId='';if(this.restorePending)this.sendCommand('document:set-readonly',{readonly:true});}
        this.showError(new Error(payload.message || "Could not export document"));
      }
    },
    scheduleSave() {
      clearTimeout(this.saveTimer);
      if(this.restorePending)return;
      this.saveTimer = setTimeout(() => this.requestSave(), SAVE_DELAY_MS);
    },
    async openHistory(){
      if(!await this.flushSave())return;
      this.sendCommand('document:set-readonly',{readonly:true});
      this.historyVisible=true;
    },
    closeHistory(){this.historyVisible=false;if(!this.restorePending)this.sendCommand('document:set-readonly',{readonly:!this.canEdit});},
    async restoreVersion(version){
      this.restoringVersion=true;
      try{
        if(!await this.flushSave())throw new Error('Save the current edits before restoring a version.');
        await this.collaboration.prepareCheckpoint();
        await this.collaboration.request('history-restore',{version:version.id,digest:version.digest,lease:this.collaboration.lease,revision:this.collaboration.checkpointRevision});
        this.collaboration.cancelCheckpoint();this.historyVisible=false;
        await this.reloadRestored();
        notify.showSuccess('Version restored');
      }catch(error){this.showError(error);}finally{this.restoringVersion=false;}
    },
    onRemoteRestore(){
      clearTimeout(this.saveTimer);
      this.closeHistory();
      if(this.dirty || this.collaboration.outbox || this.collaboration.hasLocalChanges() || this.saving){
        this.restorePending=true;this.dirty=true;this.saving=false;this.statusText='A version was restored on another device. Download your draft before reloading.';
        this.sendCommand('document:set-readonly',{readonly:true});
        this.collaboration.cancelCheckpoint();this.completeSave?.(false);this.completeSave=null;
      }else{void this.reloadRestored().catch(error=>this.showError(error));}
    },
    async reloadRestored(){
      this.confirmReload=false;this.restorePending=false;this.dirty=false;this.editorDirty=false;this.saving=false;
      clearTimeout(this.saveTimer);clearTimeout(this.saveTimeout);
      await this.startCollaboration(this.isNewDocument ? this.targetPath : this.file.path);
    },
    downloadCopy() {
      clearTimeout(this.saveTimer);
      // The vendor exporter refuses a restricted model. Keep the retired
      // session paused and the frame inert while exporting its private draft.
      if(this.restorePending)this.sendCommand('document:set-readonly',{readonly:false});
      this.copyCommandId=this.sendCommand('document:save',{preserveChanges:true});
    },
    requestSave() {
      if (!this.canEdit || this.restorePending || (!this.dirty && !this.isNewDocument) || this.saving || this.copyCommandId || !this.editorReady) return;
      if (this.isNewDocument && !this.targetPath) {
        const name = this.newFileName.trim();
        if (!name || /[\\/<>:"|?*]/.test(name) || !name.toLowerCase().endsWith(`.${this.newType.toLowerCase()}`)) {
          this.statusText = `Enter a filename ending in .${this.newType}`;
          return;
        }
        const basePath = this.parentPath || state.req?.path || "/";
        this.targetPath = url.joinPath(basePath, name);
      }
      clearTimeout(this.saveTimer);
      this.saving = true;
      this.saveCompletion = new Promise((resolve) => { this.completeSave = resolve; });
      this.saveTimeout = setTimeout(() => this.showError(new Error("Saving timed out. Your changes are still open; try Save again.")), 60000);
      this.saveRequestedAt = this.dirtyRevision;
      this.statusText = "Preparing save…";
      const prepare=this.collaboration ? this.collaboration.prepareCheckpoint() : Promise.resolve();
      prepare.then(()=>{
        this.saveRequestedAt=this.dirtyRevision;
        this.pendingCommandId=this.sendCommand("document:save",{preserveChanges:Boolean(this.collaboration)});
        if(!this.pendingCommandId)this.showError(new Error("Editor is not ready"));
      }).catch(error=>this.showError(error));
    },
    async persistDocument(payload) {
      clearTimeout(this.saveTimeout);
      const exported = payload.file;
      if (!(exported instanceof Blob)) throw new Error("The editor returned no document data");
      const name = this.isNewDocument
        ? this.targetPath.split("/").pop()
        : (this.file.name || payload.fileName);
      const file = new File([exported], name, { type: exported.type || "application/octet-stream" });
      const path = this.isNewDocument
        ? this.targetPath
        : this.file.path;

      this.statusText = "Saving to FileBrowser…";
      if(this.collaboration){
        await this.collaboration.checkpoint(file);
      } else if (this.isNewDocument && !this.createdInStorage) {
        if (this.isShare) {
          await resourcesApi.postPublic(this.shareContext.hash, path, file, false);
        } else {
          await resourcesApi.post(this.saveSource, path, file, false);
        }
        this.createdInStorage = true;
        this.activateAfterSave=true;
        if (!this.canEdit) this.sendCommand("document:set-readonly", { readonly: true });
        mutations.setReload(true);
      } else if (this.isShare) {
        await resourcesApi.putPublic(this.shareContext.hash, path, file);
      } else {
        await resourcesApi.put(this.saveSource, path, file);
      }

      this.pendingCommandId = "";
      this.saving = false;
      clearTimeout(this.saveTimeout);
      this.dirty = Boolean(payload.dirty) || this.editorDirty || this.dirtyRevision > this.saveRequestedAt;
      this.statusText = this.dirty ? "Saving recent edits…" : "All changes saved";
      if (this.dirty) this.scheduleSave();
      this.completeSave?.(true);
      this.completeSave = null;
      if(this.activateAfterSave && !this.dirty){
        this.activateAfterSave=false;
        clearTimeout(this.saveTimer);
        await this.startCollaboration(this.targetPath);
      }
    },
    onFileNameChange() {
      if (!this.createdInStorage) this.targetPath = "";
      if (this.dirty && this.canEdit) this.scheduleSave();
    },
    showError(error) {
      this.copyCommandId='';
      this.collaboration?.cancelCheckpoint();
      const message = error?.message || String(error);
      this.statusText = message;
      if (this.saving) this.dirty = true;
      this.saving = false;
      clearTimeout(this.saveTimeout);
      this.completeSave?.(false);
      this.completeSave = null;
      this.pendingCommandId = "";
      notify.showError(message);
      if(this.collaboration && this.canEdit && !/outside this session|expired|limit reached|403|401/.test(message))this.scheduleSave();
    },
    beforeUnload(event) {
      if (this.dirty || this.saving) {
        event.preventDefault();
        event.returnValue = "";
      }
    },
    async flushSave() {
      while (this.dirty || this.saving) {
        if (!this.saving) this.requestSave();
        if (!this.saving || !this.saveCompletion) return false;
        if (!await this.saveCompletion) return false;
      }
      return true;
    },
    async close() {
      if (await this.flushSave()) this.finishClose();
    },
    finishClose() {
      if (this.isPrompt) {
        mutations.closeTopPrompt(this.promptId);
        return;
      }
      const filename = this.file.name || "";
      const listingPath = `${removeLastDir(state.route.path)}/`;
      void router.push({ path: listingPath, hash: filename ? `#${filename}` : "" });
    },
  },
};
</script>

<style scoped>
.document-editor {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: calc(100dvh - 4rem);
  min-height: 24rem;
  background: var(--background);
}

.document-editor--prompt {
  height: 100%;
  min-height: 0;
}

.document-editor__toolbar {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  min-height: 3rem;
  padding: 0.35rem 0.75rem;
  border-bottom: 1px solid var(--divider);
  background: var(--panel-bg);
}

.document-editor__filename,
.document-editor__title {
  flex: 1 1 auto;
  min-width: 5rem;
  overflow: hidden;
  color: var(--textPrimary);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.document-editor__filename {
  border: 0;
  border-radius: 0.4rem;
  padding: 0.4rem 0.6rem;
  background: var(--input-bg, var(--panel-bg));
}

.document-editor__status {
  color: var(--textSecondary);
  font-size: 0.85rem;
  white-space: nowrap;
}

.document-editor__presence {
  color: var(--textSecondary);
  font-size: 0.8rem;
  white-space: nowrap;
}

.document-editor__restore-confirm{padding:1rem;background:var(--panel-bg);border-bottom:1px solid var(--divider);font-size:.9rem}
.document-editor__restore-confirm button{margin-right:.5rem}

.document-editor__legal {
  color: var(--textSecondary);
  font-size: 0.75rem;
  white-space: nowrap;
}

.document-editor__frame {
  width: 100%;
  flex: 1 1 auto;
  min-height: 0;
  border: 0;
  background: #fff;
}

@media (max-width: 720px) {
  .document-editor {
    height: calc(100dvh - 4rem);
  }

  .document-editor__toolbar {
    flex-wrap: wrap;
  }

  .document-editor__status {
    flex: 1 1 100%;
    order: 1;
  }
}
</style>
