<template>
  <section class="document-editor" :class="{ 'document-editor--prompt': isPrompt }">
    <div class="document-editor__toolbar">
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
      <a class="document-editor__legal" href="https://github.com/fahimc/filebrowser/blob/drive-reskin/third_party/NOTICES.md" target="_blank" rel="noopener noreferrer">Source &amp; licenses</a>
      <button
        type="button"
        class="button button--flat document-editor__save"
        :disabled="!canEdit || (!dirty && !isNewDocument) || saving || !editorReady"
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
    <iframe
      ref="editorFrame"
      class="document-editor__frame"
      :src="editorUrl"
      :title="fileName"
      allow="clipboard-read; clipboard-write"
    />
  </section>
</template>

<script>
import { resourcesApi } from "@/api";
import { fetchURL } from "@/api/utils.ts";
import { ensureViewToken } from "@/api/viewToken.js";
import { getters, mutations, state } from "@/store";
import { globalVars } from "@/utils/constants.js";
import { url } from "@/utils";
import { removeLastDir } from "@/utils/url.js";
import { notify } from "@/notify";
import { router } from "@/router";
import { createBlankPDF } from "@/utils/office.js";

const SAVE_DELAY_MS = 1800;
const OFFICE_TYPES = new Set(["docx", "xlsx", "pptx", "pdf"]);

export default {
  name: "documentEditor",
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
    };
  },
  computed: {
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
      if (this.isNewDocument && this.newType !== 'pdf') params.set("new", this.newType.toLowerCase());
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
      return;
    }
    this.openExistingFile().catch((error) => this.showError(error));
  },
  beforeUnmount() {
    window.removeEventListener("message", this.onEditorMessage);
    window.removeEventListener("beforeunload", this.beforeUnload);
    this.removeNavigationGuard?.();
    clearTimeout(this.saveTimer);
    clearTimeout(this.saveTimeout);
  },
  methods: {
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
      const sharePath = this.file.path;
      const source = this.saveSource;
      const viewToken = this.file.viewToken || await ensureViewToken(source);
      const viewURL = this.isShare
        ? resourcesApi.getRawViewURLPublic(this.shareContext, [sharePath], viewToken)
        : resourcesApi.getRawViewURL(source, this.file.path, viewToken);
      const headers = {};
      if (this.isShare) {
        const password = localStorage.getItem(`sharepass:${this.shareContext.hash}`);
        if (password) headers["X-SHARE-PASSWORD"] = encodeURIComponent(password);
      }
      const response = await fetchURL(viewURL, { headers });
      const buffer = await response.arrayBuffer();
      this.documentBuffer = buffer;
      this.documentOpened = false;
      if (this.bridgeReady) this.openEditorBuffer();
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
        this.statusText = this.canEdit ? "Ready" : "View only";
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
        if (this.pendingCommandId && id === this.pendingCommandId) {
          this.persistDocument(payload).catch((error) => this.showError(error));
        }
      } else if (type === "document:error") {
        this.showError(new Error(payload.message || "Could not export document"));
      }
    },
    scheduleSave() {
      clearTimeout(this.saveTimer);
      this.saveTimer = setTimeout(() => this.requestSave(), SAVE_DELAY_MS);
    },
    requestSave() {
      if (!this.canEdit || (!this.dirty && !this.isNewDocument) || this.saving || !this.editorReady) return;
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
      this.pendingCommandId = this.sendCommand("document:save");
      if (!this.pendingCommandId) {
        this.showError(new Error("Editor is not ready"));
      }
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
      if (this.isNewDocument && !this.createdInStorage) {
        if (this.isShare) {
          await resourcesApi.postPublic(this.shareContext.hash, path, file, false);
        } else {
          await resourcesApi.post(this.saveSource, path, file, false);
        }
        this.createdInStorage = true;
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
    },
    onFileNameChange() {
      if (!this.createdInStorage) this.targetPath = "";
      if (this.dirty && this.canEdit) this.scheduleSave();
    },
    showError(error) {
      const message = error?.message || String(error);
      this.statusText = message;
      if (this.saving) this.dirty = true;
      this.saving = false;
      clearTimeout(this.saveTimeout);
      this.completeSave?.(false);
      this.completeSave = null;
      this.pendingCommandId = "";
      notify.showError(message);
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
