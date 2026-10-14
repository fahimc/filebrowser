<template><PhotosLibrary :api="api" @unauthorized="signIn"/></template>
<script setup>
import {useRouter} from 'vue-router';
import PhotosLibrary from '@/components/photos/PhotosLibrary.vue';
import {createPhotosApi} from '@/components/photos/api.js';
import {globalVars} from '@/utils/constants.js';
const router=useRouter();const signIn=()=>router.push('/login?redirect=/photos');
const api=createPhotosApi({base:globalVars.baseURL||'/',onUnauthorized:signIn,filebrowserURL:globalVars.baseURL||'/'});
</script>
