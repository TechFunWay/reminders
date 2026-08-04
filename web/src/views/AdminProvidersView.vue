<template>
  <div class="mx-auto max-w-2xl">
    <div class="mb-7">
      <p class="mb-2 text-xs font-extrabold uppercase tracking-[.18em] text-violet-500">管理员</p>
      <h1 class="font-display text-3xl font-extrabold tracking-tight">通知服务</h1>
      <p class="mt-2 text-sm leading-6 text-muted-foreground">配置一次，所有用户即可自行绑定。凭证会加密保存，保存后立即生效。</p>
    </div>
    <section class="rounded-[24px] border border-border/70 bg-surface/85 p-6 shadow-[0_16px_44px_-34px_rgba(15,23,42,.55)]">
      <div class="flex items-center justify-between"><div><h2 class="text-lg font-extrabold">飞书机器人</h2><p class="mt-1 text-sm text-muted-foreground">用户后续只需填写工作邮箱或手机号。</p></div><span class="rounded-full px-2.5 py-1 text-xs font-bold" :class="configured ? 'bg-emerald-500/10 text-emerald-600' : 'bg-amber-500/10 text-amber-600'">{{ configured ? '已开通' : '未开通' }}</span></div>
      <form class="mt-6 space-y-4" @submit.prevent="save">
        <label class="block text-sm font-bold">App ID<input v-model.trim="appID" class="field mt-2" placeholder="cli_xxxxxxxxx" autocomplete="off" /></label>
        <label class="block text-sm font-bold">App Secret<input v-model.trim="appSecret" class="field mt-2" type="password" placeholder="请输入 App Secret" autocomplete="new-password" /></label>
        <p class="text-xs leading-5 text-muted-foreground">在飞书开放平台启用机器人，并为应用授予通讯录查询权限及设置应用可见范围。</p>
        <button class="rounded-2xl bg-brand-500 px-5 py-3 text-sm font-bold text-white disabled:opacity-50" :disabled="saving || !appID || !appSecret">{{ saving ? '保存中…' : configured ? '更新配置' : '保存并开通' }}</button>
      </form>
    </section>
    <p v-if="message" class="mt-4 text-sm" :class="error ? 'text-rose-500' : 'text-emerald-600'">{{ message }}</p>
  </div>
</template>
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { getProviderStatuses, saveFeishuProvider } from '../api/reminder'
const configured = ref(false), appID = ref(''), appSecret = ref(''), saving = ref(false), message = ref(''), error = ref(false)
async function load() { const r = await getProviderStatuses(); configured.value = !!r.data?.data?.feishu?.configured }
async function save() { saving.value = true; message.value = ''; error.value = false; try { await saveFeishuProvider({ app_id: appID.value, app_secret: appSecret.value }); configured.value = true; appSecret.value = ''; message.value = '飞书服务已开通，现在用户可以直接绑定。' } catch (e: any) { error.value = true; message.value = e.response?.data?.message || '保存失败' } finally { saving.value = false } }
onMounted(() => load().catch(() => { error.value = true; message.value = '读取配置失败' }))
</script>
<style scoped>.field { @apply h-11 w-full rounded-2xl border border-border bg-surface px-3.5 text-sm font-normal outline-none focus:border-brand-500/40 focus:ring-4 focus:ring-brand-500/10; }</style>
