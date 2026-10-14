import {defineConfig} from 'vite';
import vue from '@vitejs/plugin-vue';
import path from 'node:path';
export default defineConfig({root:path.resolve(import.meta.dirname,'photos'),base:'/photos-static/',plugins:[vue()],build:{outDir:path.resolve(import.meta.dirname,'../apps/photos/web'),emptyOutDir:true},resolve:{dedupe:['vue']}});
