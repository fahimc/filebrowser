<template><img v-if="name==='logo'" :src="photosLogo" alt="" aria-hidden="true" /><svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path :d="paths[name]||paths.photo"/></svg></template>
<script setup>
import photosLogo from "../../assets/branding/my_photos.svg";
defineProps({name:{type:String,default:'photo'}});
const paths={photo:'M4 4h16v16H4Z M4 16l5-5 4 4 3-3 4 4 M15 8h.01',search:'M21 21l-5-5 M18 10a8 8 0 1 1-16 0 8 8 0 0 1 16 0',star:'m12 3 2.8 5.7 6.2.9-4.5 4.4 1.1 6.2-5.6-3-5.6 3 1.1-6.2L3 9.6l6.2-.9Z',folder:'M3 7V5h7l2 2h9v13H3Z',upload:'M12 16V3 m-5 5 5-5 5 5 M4 16v5h16v-5',close:'M6 6l12 12 M18 6 6 18',left:'m15 5-7 7 7 7',right:'m9 5 7 7-7 7',info:'M12 11v6 M12 7h.01 M22 12a10 10 0 1 1-20 0 10 10 0 0 1 20 0',download:'M12 3v12 m-5-5 5 5 5-5 M4 18v3h16v-3',refresh:'M20 7V3l-4 4 M20 7a9 9 0 1 0 1 7',video:'m9 7 8 5-8 5Z M3 3h18v18H3Z',grid:'M3 3h7v7H3Z M14 3h7v7h-7Z M3 14h7v7H3Z M14 14h7v7h-7Z',drive:'M5 19h14l3-5-7-12H9L2 14Z M9 2l7 12 M2 14h20 M5 19l7-12'};
</script>
<style scoped>svg,img{width:24px;height:24px;flex:none}</style>
