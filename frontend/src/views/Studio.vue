<template><section class="studio-host"><iframe :key="editorURL" :src="editorURL" title="Studio image editor" allow="clipboard-read; clipboard-write"/></section></template>
<script setup>
import {computed} from 'vue';
import {useRoute} from 'vue-router';
import {globalVars} from '@/utils/constants.js';
const route=useRoute();
const editorURL=computed(()=>`${globalVars.baseURL||'/'}studio/editor/?${new URLSearchParams({...(route.query.source?{source:String(route.query.source),path:String(route.query.path||'/')}:{} )})}`);
</script>
<style scoped>.studio-host{height:calc(100dvh - 72px);padding:0;background:#202020}.studio-host iframe{width:100%;height:100%;border:0;display:block}</style>
