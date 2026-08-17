<template>
  <div class="mx-auto max-w-5xl">
    <div class="mb-7">
      <p class="mb-2 text-xs font-extrabold uppercase tracking-[.18em] text-violet-500">随时触达</p>
      <h1 class="font-display text-3xl font-extrabold tracking-tight">通知方式</h1>
      <p class="mt-2 max-w-2xl text-sm leading-6 text-muted-foreground">站内消息默认可用。已开通的方式，绑定一次即可在每条提醒中自由选择。</p>
    </div>

    <div class="mb-5 rounded-2xl border border-blue-500/15 bg-blue-500/[.06] px-4 py-3 text-xs leading-5 text-blue-700 dark:text-blue-200">
      选择你想接收提醒的方式即可。邮件需要先配置“发件邮箱”，再绑定“接收邮箱”；飞书请填写接收提醒消息的工作邮箱或手机号，不需要查找 OpenID。
    </div>

    <section v-if="authStore.isAdmin" class="mb-5 flex flex-col gap-3 rounded-2xl border border-violet-500/15 bg-violet-500/[.05] p-4 sm:flex-row sm:items-end">
      <label class="min-w-0 flex-1 text-xs font-extrabold text-foreground">机器人消息来源名称
        <input v-model.trim="notificationBrand" class="bind-input mt-2" maxlength="40" placeholder="例如：我的提醒" />
      </label>
      <button class="bind-button shrink-0" :disabled="busy === 'notification-brand' || !notificationBrand" @click="saveBrand">{{ busy === 'notification-brand' ? '保存中…' : '保存名称' }}</button>
      <p class="text-[11px] leading-5 text-muted-foreground sm:max-w-48">会显示在飞书和 QQ 消息开头，例如【我的提醒】。</p>
    </section>

    <div v-if="loading" class="grid gap-4 md:grid-cols-2">
      <div v-for="n in 4" :key="n" class="h-52 animate-pulse rounded-[24px] bg-surface"></div>
    </div>

    <div v-else class="grid gap-4 md:grid-cols-2">
      <article v-for="channel in channels" :key="channel.channel" class="channel-card">
        <div class="flex items-start gap-4">
          <span class="channel-icon" :class="channel.channel" v-html="icon(channel.channel)"></span>
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
              <h2 class="text-base font-extrabold">{{ channel.label }}</h2>
              <span class="status-pill" :class="statusClass(channel)">{{ statusText(channel) }}</span>
            </div>
            <p class="mt-1 text-xs leading-5 text-muted-foreground">{{ channel.description }}</p>
          </div>
          <button v-if="channel.channel !== 'inapp' && channel.bound && channel.configured" class="toggle" :class="{ on: channel.status === 'active' }" :aria-label="channel.status === 'active' ? '关闭渠道' : '开启渠道'" @click="toggle(channel)">
            <span></span>
          </button>
        </div>

        <button v-if="authStore.isAdmin && channel.configured && ['feishu', 'qq'].includes(channel.channel)" class="mt-4 w-full rounded-xl border border-violet-500/15 bg-violet-500/[.05] px-3 py-2 text-xs font-bold text-violet-600" @click="openBotSetup(channel.channel)">
          更换机器人 App ID / App Secret
        </button>

        <div v-if="channel.channel === 'inapp'" class="mt-5 rounded-2xl bg-emerald-500/[.07] px-4 py-3 text-xs font-semibold text-emerald-600 dark:text-emerald-300">
          ✓ 已启用，提醒会保存在通知中心
        </div>

        <div v-else-if="channel.bound" class="mt-5">
          <div class="flex items-center justify-between rounded-2xl border border-border bg-muted/45 px-4 py-3">
            <div>
              <p class="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">已绑定</p>
              <p class="mt-1 text-sm font-bold">{{ channel.target_masked }}</p>
            </div>
            <span class="h-2.5 w-2.5 rounded-full" :class="channel.configured && channel.status === 'active' ? 'bg-emerald-500' : 'bg-amber-500'"></span>
          </div>
          <div class="mt-3 flex gap-2">
            <button class="channel-button primary" :disabled="busy === channel.channel || !channel.configured" @click="test(channel)">
              {{ busy === channel.channel ? '发送中…' : channel.configured ? '发送测试' : '服务端未配置' }}
            </button>
            <button class="channel-button" @click="startRebind(channel)">更换</button>
            <button class="channel-button danger" @click="remove(channel)">解绑</button>
          </div>
        </div>

        <div v-else-if="!channel.configured && channel.channel === 'feishu' && authStore.isAdmin" class="mt-5 rounded-2xl border border-cyan-500/20 bg-cyan-500/[.06] p-4">
          <p class="text-sm font-extrabold">开通飞书机器人</p>
          <p class="mt-1 text-xs leading-5 text-muted-foreground">填写飞书开放平台的应用凭证。保存后立即生效，无需重启；凭证仅加密保存在本应用中。</p>
          <form class="mt-4 space-y-3" @submit.prevent="configureFeishu">
            <input v-model.trim="feishuConfig.app_id" class="bind-input" placeholder="App ID，例如 cli_xxxxx" autocomplete="off" />
            <input v-model.trim="feishuConfig.app_secret" class="bind-input" type="password" placeholder="App Secret" autocomplete="new-password" />
            <button class="bind-button w-full" :disabled="busy === 'feishu-config' || !feishuConfig.app_id || !feishuConfig.app_secret">{{ busy === 'feishu-config' ? '开通中…' : '保存并开通飞书机器人' }}</button>
          </form>
          <p class="mt-3 text-[10px] leading-4 text-muted-foreground">还需在飞书开放平台启用机器人、设置应用可见范围，并开通“通过邮箱或手机号获取用户 ID”的通讯录权限。</p>
        </div>

        <div v-else-if="!channel.configured && channel.channel === 'email' && authStore.isAdmin" class="provider-form"><p class="text-sm font-extrabold">开通邮件提醒</p><p class="provider-help">先配置用于发出提醒的发件邮箱和授权码；配置完成后，还需在邮件卡片中绑定实际接收提醒的邮箱。</p><button class="bind-button mt-4 w-full" @click="openEmailSetup">配置邮件服务</button></div>

        <div v-else-if="!channel.configured && channel.channel === 'sms' && authStore.isAdmin" class="provider-form">
          <p class="text-sm font-extrabold">开通短信提醒</p><p class="provider-help">填写你的短信服务转发地址。系统会向该地址发送手机号、提醒标题、时间和幂等标识。</p>
          <form class="mt-4 space-y-3" @submit.prevent="configureProvider('sms')"><input v-model.trim="providerConfig.sms.webhook_url" class="bind-input" placeholder="短信服务 Webhook 地址" type="url" /><input v-model.trim="providerConfig.sms.webhook_token" class="bind-input" placeholder="可选：Webhook Token" type="password" autocomplete="new-password" /><button class="bind-button w-full" :disabled="busy === 'sms-config' || !providerConfig.sms.webhook_url">{{ busy === 'sms-config' ? '开通中…' : '保存并开通短信' }}</button></form>
        </div>

        <div v-else-if="!channel.configured && channel.channel === 'qq' && authStore.isAdmin" class="provider-form">
          <p class="text-sm font-extrabold">开通 QQ 机器人</p>
          <p class="provider-help">填写 QQ 开放平台的 App ID 和 App Secret。配置后还需完成用户身份绑定，机器人才能主动私聊。</p>
          <form class="mt-4 space-y-4" @submit.prevent="configureProvider('qq')">
            <label class="provider-field">
              <span class="provider-field-label">App ID <small class="provider-badge required">必填</small></span>
              <input v-model.trim="providerConfig.qq.app_id" class="bind-input w-full" placeholder="例如：1903753507" autocomplete="off" />
              <span class="provider-field-help">QQ 开放平台创建机器人后生成的唯一标识，用来识别你的机器人。</span>
            </label>
            <label class="provider-field">
              <span class="provider-field-label">App Secret <small class="provider-badge required">必填</small></span>
              <input v-model.trim="providerConfig.qq.app_secret" class="bind-input w-full" placeholder="请输入 QQ 开放平台的 App Secret" type="password" autocomplete="new-password" />
              <span class="provider-field-help">与 App ID 配套的密钥，只用于向 QQ 官方接口换取访问凭证，请勿分享。</span>
            </label>
            <label class="provider-field">
              <span class="provider-field-label">机器人主页 / 邀请链接 <small class="provider-badge recommended">推荐</small></span>
              <input v-model.trim="providerConfig.qq.bot_link" class="bind-input w-full" type="url" placeholder="例如：https://q.qq.com/..." />
              <span class="provider-field-help">绑定时显示“打开 QQ 机器人”按钮，方便用户进入机器人；不参与消息发送。</span>
            </label>
            <label class="provider-field">
              <span class="provider-field-label">消息 API 地址 <small class="provider-badge optional">可选</small></span>
              <input v-model.trim="providerConfig.qq.api_base" class="bind-input w-full" placeholder="留空使用 https://api.bot.qq.com" />
              <span class="provider-field-help">一般留空。仅在使用代理或兼容网关时填写；获取访问凭证仍使用 QQ 官方地址。</span>
            </label>
            <button class="bind-button w-full" :disabled="busy === 'qq-config' || !providerConfig.qq.app_id || !providerConfig.qq.app_secret">{{ busy === 'qq-config' ? '开通中…' : '保存并开通 QQ 机器人' }}</button>
          </form>
        </div>

        <div v-else-if="!channel.configured" class="mt-5 rounded-2xl border border-amber-500/15 bg-amber-500/[.06] px-4 py-3 text-xs leading-5 text-muted-foreground">
          <strong class="text-foreground">暂未开通</strong>
          <p class="mt-1">该方式尚未由应用管理员开通。</p>
        </div>

        <div v-else-if="channel.channel === 'qq'" class="mt-5 rounded-2xl border border-rose-500/15 bg-rose-500/[.04] p-4">
          <p class="text-sm font-extrabold">绑定 QQ</p>
          <template v-if="qqBindCode">
            <p class="mt-2 text-xs text-muted-foreground">先打开机器人，再发送以下消息（10 分钟内有效）：</p>
            <a v-if="channel.bot_link" class="mt-3 flex h-10 items-center justify-center rounded-xl bg-rose-500 px-3 text-xs font-bold text-white" :href="channel.bot_link" target="_blank" rel="noopener">打开 QQ 机器人</a>
            <div class="mt-3 rounded-xl bg-surface px-4 py-3 font-mono text-lg font-bold tracking-widest text-foreground">/绑定 {{ qqBindCode }}</div>
            <button class="mt-3 w-full rounded-xl border border-rose-500/15 bg-surface px-3 py-2 text-xs font-bold text-rose-500" @click="copyQQBindCommand">复制绑定消息</button>
            <p class="mt-3 text-[10px] leading-4 text-muted-foreground">发送后会静默完成绑定；机器人回复“绑定成功”即表示完成。<span v-if="!channel.bot_link">未配置机器人入口时，请向管理员索取机器人主页或二维码。</span></p>
          </template>
          <template v-else>
            <p class="mt-1 text-xs leading-5 text-muted-foreground">不需要填写 OpenID。{{ channel.bot_link ? '先打开机器人，再获取绑定码。' : '请先向管理员索取机器人主页或二维码，再获取绑定码。' }}</p>
            <a v-if="channel.bot_link" class="mt-4 flex h-11 items-center justify-center rounded-2xl bg-rose-500 px-4 text-sm font-bold text-white" :href="channel.bot_link" target="_blank" rel="noopener">打开 QQ 机器人</a>
            <button class="bind-button mt-4 w-full" :disabled="busy === 'qq-code'" @click="getQQBindCode">{{ busy === 'qq-code' ? '生成中…' : '获取 QQ 绑定码' }}</button>
          </template>
        </div>

        <div v-else-if="channel.channel === 'dingtalk'" class="mt-5 rounded-2xl border border-sky-500/15 bg-sky-500/[.05] p-4">
          <p class="text-sm font-extrabold">绑定钉钉群机器人</p>
          <p class="mt-2 rounded-xl bg-amber-500/[.08] px-3 py-2 text-xs font-semibold leading-5 text-amber-700 dark:text-amber-200">请使用 Windows 或 Mac 的钉钉电脑端操作；手机端无法创建或配置此机器人。</p>
          <ol class="mt-2 list-decimal space-y-1 pl-4 text-xs leading-5 text-muted-foreground">
            <li>在电脑端钉钉新建一个专用提醒群，或打开现有群。</li>
            <li>不希望他人看到提醒时，请只保留自己在群内：可新建个人群，或将其他成员移出。</li>
            <li>打开群设置，进入“群机器人 / 机器人”，添加“自定义机器人”。</li>
            <li>安全设置选择“自定义关键词”，填写 <strong class="text-foreground">提醒</strong>，复制 Webhook 地址。</li>
            <li>粘贴到下方并发送测试；地址仅加密保存，不会显示在页面或日志中。</li>
          </ol>
          <p class="mt-3 text-[11px] leading-5 text-sky-700 dark:text-sky-200">提醒会发送给该群的所有当前成员；群内只有你一人时，只有你能收到。</p>
          <form class="mt-4" @submit.prevent="bind(channel)">
            <label class="text-[11px] font-bold text-muted-foreground">钉钉机器人 Webhook 地址</label>
            <div class="mt-2 flex gap-2">
              <input v-model.trim="targets[channel.channel]" class="bind-input" type="url" placeholder="https://oapi.dingtalk.com/robot/send?access_token=..." autocomplete="off" />
              <button class="bind-button" :disabled="busy === channel.channel || !targets[channel.channel]?.trim()">{{ busy === channel.channel ? '绑定中' : '绑定' }}</button>
            </div>
          </form>
        </div>

        <form v-else class="mt-5" @submit.prevent="bind(channel)">
          <label class="text-[11px] font-bold text-muted-foreground">{{ inputLabel(channel.channel) }}</label>
          <div class="mt-2 flex gap-2">
            <input v-model="targets[channel.channel]" class="bind-input" :placeholder="placeholder(channel.channel)" :type="channel.channel === 'email' ? 'email' : 'text'" />
            <button class="bind-button" :disabled="busy === channel.channel || !targets[channel.channel]?.trim()">{{ busy === channel.channel ? '绑定中' : '绑定' }}</button>
          </div>
          <p v-if="channel.channel === 'email'" class="mt-2 text-[10px] leading-4 text-muted-foreground">填写提醒实际送达的接收邮箱。它可以与发件邮箱相同，也可以不同。</p>
          <p v-else-if="channel.channel === 'feishu'" class="mt-2 text-[10px] leading-4 text-muted-foreground">填写你在飞书中要接收提醒消息的工作邮箱或手机号。</p>
        </form>

        <p v-if="channel.last_error_code" class="mt-3 text-xs text-rose-500">{{ channel.last_error_code }}</p>
      </article>
    </div>

    <Toast :message="toast.message" :type="toast.type" />
    <Teleport to="body"><div v-if="confirmChannel" class="fixed inset-0 z-[80] flex items-center justify-center bg-slate-950/35 p-5 backdrop-blur-sm"><section class="w-full max-w-sm rounded-[28px] border border-white/40 bg-surface p-6 shadow-2xl"><div class="flex h-11 w-11 items-center justify-center rounded-2xl bg-rose-500/10 text-xl text-rose-500">!</div><h2 class="mt-4 text-lg font-extrabold">解绑{{ confirmChannel.label }}？</h2><p class="mt-2 text-sm leading-6 text-muted-foreground">解绑后将停止通过此方式接收提醒；之后可以随时重新绑定。</p><div class="mt-6 flex gap-3"><button class="channel-button" @click="confirmChannel = null">取消</button><button class="flex-1 rounded-xl bg-rose-500 px-3 py-2 text-xs font-bold text-white" @click="confirmRemove">确认解绑</button></div></section></div></Teleport>
    <Teleport to="body"><div v-if="emailSetupOpen" class="fixed inset-0 z-[80] flex items-center justify-center bg-slate-950/35 p-4 backdrop-blur-sm"><form class="w-full max-w-lg rounded-[28px] border border-white/40 bg-surface p-6 shadow-2xl sm:p-8" @submit.prevent="saveSimpleEmail"><div class="flex items-center justify-between gap-4"><div><p class="text-xs font-extrabold uppercase tracking-[.16em] text-violet-500">邮件提醒</p><h2 class="mt-1 text-2xl font-extrabold">邮件通知服务设置</h2></div><button type="button" class="icon-button" aria-label="关闭" @click="emailSetupOpen = false">×</button></div><p class="mt-4 rounded-2xl bg-blue-500/[.07] px-4 py-3 text-xs leading-5 text-blue-700 dark:text-blue-200">这一步设置的是发件邮箱：系统会使用它的 SMTP 服务和授权码发出提醒。设置完成后，请在邮件卡片中再填写提醒要送达的接收邮箱；两个邮箱可以相同，也可以不同。</p><label class="modal-label mt-7">服务提供商 <span>*</span><select v-model="emailSetup.provider" class="modal-field mt-2"><option value="qq">QQ 邮箱</option><option value="163">163 邮箱</option><option value="gmail">Gmail</option><option value="custom">其他 SMTP 服务</option></select></label><label class="modal-label mt-5">发件邮箱地址 <span>*</span><input v-model.trim="emailSetup.address" class="modal-field mt-2" type="email" maxlength="80" placeholder="请输入发件邮箱地址" /></label><label class="modal-label mt-5">发件人名称 <small>可选</small><input v-model.trim="emailSetup.sender_name" class="modal-field mt-2" maxlength="40" placeholder="例如：我的提醒" /></label><template v-if="emailSetup.provider === 'custom'"><label class="modal-label mt-5">SMTP 服务器 <span>*</span><input v-model.trim="emailSetup.host" class="modal-field mt-2" placeholder="例如 smtp.example.com" /></label><label class="modal-label mt-5">端口 <span>*</span><input v-model.trim="emailSetup.port" class="modal-field mt-2" inputmode="numeric" placeholder="465 或 587" /></label></template><label class="modal-label mt-5">发件邮箱授权码 <span>*</span><input v-model.trim="emailSetup.password" class="modal-field mt-2" type="password" placeholder="请输入发件邮箱对应的授权码（不是邮箱登录密码）" autocomplete="new-password" /></label><p class="mt-4 text-[11px] leading-4 text-muted-foreground">授权码需在对应邮箱服务商的设置中生成，不能填写邮箱登录密码。</p><p class="mt-4 rounded-2xl bg-blue-500/[.07] px-4 py-3 text-xs leading-5 text-blue-700 dark:text-blue-200">{{ emailProviderTip }}</p><div class="mt-7 flex justify-end gap-3"><button type="button" class="channel-button !flex-none" @click="emailSetupOpen = false">取消</button><button class="rounded-xl bg-brand-500 px-5 py-2 text-sm font-bold text-white disabled:opacity-50" :disabled="busy === 'email-config' || !emailSetup.address || !emailSetup.password || (emailSetup.provider === 'custom' && (!emailSetup.host || !emailSetup.port))">{{ busy === 'email-config' ? '保存中…' : '保存并开通' }}</button></div></form></div></Teleport>
    <Teleport to="body"><div v-if="botSetupChannel" class="fixed inset-0 z-[80] flex items-center justify-center bg-slate-950/35 p-4 backdrop-blur-sm"><form class="max-h-[90vh] w-full max-w-lg overflow-y-auto rounded-[28px] border border-white/40 bg-surface p-6 shadow-2xl sm:p-8" @submit.prevent="saveBotCredentials"><div class="flex items-center justify-between gap-4"><div><p class="text-xs font-extrabold uppercase tracking-[.16em] text-violet-500">机器人应用</p><h2 class="mt-1 text-2xl font-extrabold">更换{{ botSetupChannel === 'feishu' ? '飞书' : 'QQ' }}应用凭证</h2></div><button type="button" class="icon-button" aria-label="关闭" @click="botSetupChannel = ''">×</button></div><p class="mt-4 rounded-2xl bg-blue-500/[.07] px-4 py-3 text-xs leading-5 text-blue-700 dark:text-blue-200">保存后请使用新的机器人重新绑定当前账号：飞书填写接收提醒消息的工作邮箱或手机号；QQ 获取新的绑定码后发给机器人。</p><template v-if="botSetupChannel === 'feishu'"><label class="modal-label mt-6">App ID <span>*</span><input v-model.trim="feishuConfig.app_id" class="modal-field mt-2" placeholder="例如 cli_xxxxx" autocomplete="off" /></label><label class="modal-label mt-5">App Secret <span>*</span><input v-model.trim="feishuConfig.app_secret" class="modal-field mt-2" type="password" placeholder="请输入新的 App Secret" autocomplete="new-password" /></label></template><template v-else><label class="modal-label mt-6">QQ 机器人 App ID <span>*</span><input v-model.trim="providerConfig.qq.app_id" class="modal-field mt-2" placeholder="例如：1903753507" autocomplete="off" /></label><p class="modal-help">QQ 开放平台创建机器人后生成的唯一标识。</p><label class="modal-label mt-5">QQ 机器人 App Secret <span>*</span><input v-model.trim="providerConfig.qq.app_secret" class="modal-field mt-2" type="password" placeholder="请输入新的 App Secret" autocomplete="new-password" /></label><p class="modal-help">与 App ID 配套的密钥，只用于换取 QQ 访问凭证，请勿分享。</p><label class="modal-label mt-5">机器人主页 / 邀请链接 <small>推荐</small><input v-model.trim="providerConfig.qq.bot_link" class="modal-field mt-2" type="url" placeholder="例如：https://q.qq.com/..." /></label><p class="modal-help">用于显示“打开 QQ 机器人”按钮，不参与消息发送。</p><label class="modal-label mt-5">消息 API 地址 <small>可选</small><input v-model.trim="providerConfig.qq.api_base" class="modal-field mt-2" placeholder="留空使用 https://api.bot.qq.com" /></label><p class="modal-help">一般留空，仅代理或兼容网关场景需要填写；获取访问凭证仍使用 QQ 官方地址。</p></template><div class="mt-7 flex justify-end gap-3"><button type="button" class="channel-button !flex-none" @click="botSetupChannel = ''">取消</button><button class="rounded-xl bg-brand-500 px-5 py-2 text-sm font-bold text-white disabled:opacity-50" :disabled="botSaveDisabled">{{ busy.endsWith('-config') ? '保存中…' : '保存新的凭证' }}</button></div></form></div></Teleport>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useAuthStore } from '../stores/auth'
import Toast from '../components/Toast.vue'
import {
  bindChannel, createQQBindCode, getChannelStatuses, getNotificationBrand, saveFeishuProvider, saveNotificationBrand, saveProvider, testChannel, toggleChannel, unbindChannel,
  type ChannelStatus, type ReminderChannel
} from '../api/reminder'

const channels = ref<(ChannelStatus & { last_error_code?: string })[]>([])
const targets = reactive<Record<string, string>>({})
const feishuConfig = reactive({ app_id: '', app_secret: '' })
const qqBindCode = ref('')
const notificationBrand = ref('提醒事项')
const botSetupChannel = ref<'' | 'feishu' | 'qq'>('')
const editingProvider = ref('')
const confirmChannel = ref<ChannelStatus | null>(null)
const emailSetupOpen = ref(false)
const emailSetup = reactive({ provider: 'qq', address: '', sender_name: '', password: '', host: '', port: '587' })
const providerConfig = reactive({
  email: { host: '', port: '587', from_address: '', username: '', password: '' },
  sms: { webhook_url: '', webhook_token: '' },
  qq: { app_id: '', app_secret: '', bot_link: '', api_base: '' },
})
const busy = ref('')
const loading = ref(true)
const toast = reactive<{ message: string; type: 'success' | 'error' }>({ message: '', type: 'success' })
const authStore = useAuthStore()
const emailProviderTip = computed(() => ({ qq: 'QQ 邮箱：在“设置 → 账号”开启 POP3/SMTP 或 IMAP/SMTP 服务后生成授权码；系统将自动使用 smtp.qq.com:465。', 163: '163 邮箱：在“设置 → POP3/SMTP/IMAP”开启 SMTP 服务并生成客户端授权密码；系统将自动使用 smtp.163.com:465。', gmail: 'Gmail：请使用应用专用密码；系统将自动使用 smtp.gmail.com:465。', custom: '请向你的邮箱服务商确认 SMTP 服务器、端口和授权码。' } as Record<string, string>)[emailSetup.provider] || '')
const botSaveDisabled = computed(() => botSetupChannel.value === 'feishu'
  ? busy.value === 'feishu-config' || !feishuConfig.app_id || !feishuConfig.app_secret
  : busy.value === 'qq-config' || !providerConfig.qq.app_id || !providerConfig.qq.app_secret)

async function load(quiet = false) {
  if (!quiet) loading.value = true
  try {
    const res = await getChannelStatuses()
    channels.value = res.data.data || []
    if (channels.value.find(item => item.channel === 'qq')?.bound) qqBindCode.value = ''
  } finally { if (!quiet) loading.value = false }
}
async function loadBrand() {
  if (!authStore.isAdmin) return
  try {
    const res = await getNotificationBrand()
    notificationBrand.value = res.data?.data?.name || '提醒事项'
  } catch { /* channel setup stays usable if this optional preference is unavailable */ }
}
async function saveBrand() {
  busy.value = 'notification-brand'
  try {
    const res = await saveNotificationBrand(notificationBrand.value)
    notificationBrand.value = res.data?.data?.name || notificationBrand.value
    show('机器人消息来源名称已保存')
  } catch (err: any) { show(err.response?.data?.message || '保存失败', 'error') }
  finally { busy.value = '' }
}
async function bind(channel: ChannelStatus) {
  busy.value = channel.channel
  try {
    await bindChannel(channel.channel, targets[channel.channel] || '')
    targets[channel.channel] = ''
    await load()
    show(`${channel.label}已绑定`)
  } catch (err: any) { show(err.response?.data?.message || '绑定失败', 'error') }
  finally { busy.value = '' }
}
async function configureFeishu() {
  busy.value = 'feishu-config'
  try {
    await saveFeishuProvider(feishuConfig)
    feishuConfig.app_secret = ''
    await load()
    botSetupChannel.value = ''
    show('飞书机器人已开通，请填写接收提醒消息的飞书工作邮箱或手机号完成绑定')
  } catch (err: any) { show(err.response?.data?.message || '飞书开通失败', 'error') }
  finally { busy.value = '' }
}
async function configureProvider(provider: 'email' | 'sms' | 'qq') {
  busy.value = `${provider}-config`
  try {
    await saveProvider(provider, providerConfig[provider])
    if (provider === 'email') providerConfig.email.password = ''
    if (provider === 'sms') providerConfig.sms.webhook_token = ''
    if (provider === 'qq') providerConfig.qq.app_secret = ''
    await load()
    editingProvider.value = ''
    if (provider === 'qq') botSetupChannel.value = ''
    show(`${({ email: '邮件', sms: '短信', qq: 'QQ 机器人' } as Record<string, string>)[provider]}已开通，请绑定接收账号后发送测试`)
    return true
  } catch (err: any) { show(err.response?.data?.message || '开通失败', 'error'); return false }
  finally { busy.value = '' }
}
async function saveBotCredentials() {
  if (botSetupChannel.value === 'feishu') await configureFeishu()
  else if (botSetupChannel.value === 'qq') await configureProvider('qq')
}
function openBotSetup(channel: ReminderChannel) {
  if (channel === 'feishu' || channel === 'qq') botSetupChannel.value = channel
}
function openEmailSetup() { emailSetupOpen.value = true }
async function saveSimpleEmail() {
  const presets: Record<string, { host: string; port: string }> = { qq: { host: 'smtp.qq.com', port: '465' }, '163': { host: 'smtp.163.com', port: '465' }, gmail: { host: 'smtp.gmail.com', port: '465' } }
  const server = presets[emailSetup.provider] || { host: emailSetup.host, port: emailSetup.port }
  Object.assign(providerConfig.email, { host: server.host, port: server.port, from_address: emailSetup.address, from_name: emailSetup.sender_name, username: emailSetup.address, password: emailSetup.password })
  if (await configureProvider('email')) emailSetupOpen.value = false
}
async function getQQBindCode() {
  busy.value = 'qq-code'
  try {
    const res = await createQQBindCode()
    qqBindCode.value = res.data?.data?.code || ''
    show('请将绑定消息发送给 QQ 机器人')
  } catch (err: any) { show(err.response?.data?.message || '生成绑定码失败', 'error') }
  finally { busy.value = '' }
}
async function copyQQBindCommand() {
  const command = `/绑定 ${qqBindCode.value}`
  try {
    await navigator.clipboard.writeText(command)
    show('绑定消息已复制，去 QQ 发送即可')
  } catch { show('复制失败，请手动复制绑定消息', 'error') }
}
async function test(channel: ChannelStatus) {
  busy.value = channel.channel
  try {
    await testChannel(channel.channel)
    show(`测试${channel.label}已发送`)
  } catch (err: any) { show(err.response?.data?.message || '测试发送失败', 'error') }
  finally { busy.value = '' }
}
async function toggle(channel: ChannelStatus) {
  try {
    await toggleChannel(channel.channel, channel.status !== 'active')
    await load()
  } catch (err: any) { show(err.response?.data?.message || '更新失败', 'error') }
}
async function remove(channel: ChannelStatus) {
  confirmChannel.value = channel
}
async function confirmRemove() {
  const channel = confirmChannel.value
  if (!channel) return
  confirmChannel.value = null
  try {
    await unbindChannel(channel.channel)
    await load()
    show(`${channel.label}已解绑`)
  } catch (err: any) { show(err.response?.data?.message || '解绑失败', 'error') }
}
async function startRebind(channel: ChannelStatus) {
  await unbindChannel(channel.channel)
  await load()
}
function show(message: string, type: 'success' | 'error' = 'success') { toast.message = ''; setTimeout(() => Object.assign(toast, { message, type }), 0) }
function statusText(channel: ChannelStatus) {
  if (channel.channel === 'inapp') return '始终可用'
  if (!channel.configured) return '待配置'
  if (!channel.bound) return '未绑定'
  return channel.status === 'active' ? '已连接' : channel.status === 'disabled' ? '已停用' : '异常'
}
function statusClass(channel: ChannelStatus) {
  if ((channel.channel === 'inapp') || (channel.configured && channel.bound && channel.status === 'active')) return 'success'
  if (!channel.configured) return 'warning'
  return 'muted'
}
function inputLabel(channel: ReminderChannel) { return ({ email: '接收邮箱（提醒送达）', sms: '中国大陆手机号', feishu: '接收提醒的飞书账号', qq: 'QQ 用户 OpenID', dingtalk: '钉钉机器人 Webhook 地址' } as Record<string, string>)[channel] || '接收目标' }
function placeholder(channel: ReminderChannel) { return ({ email: '请输入提醒接收邮箱', sms: '13800138000', feishu: '接收提醒的工作邮箱或手机号', qq: '机器人单聊事件中的 user_openid', dingtalk: 'https://oapi.dingtalk.com/robot/send?access_token=...' } as Record<string, string>)[channel] || '' }
function icon(channel: ReminderChannel) {
  const icons: Record<string, string> = {
    inapp: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor"><path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9ZM10 21h4" stroke-width="1.8" stroke-linecap="round"/></svg>',
    email: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor"><rect x="3" y="5" width="18" height="14" rx="3" stroke-width="1.8"/><path d="m4 7 8 6 8-6" stroke-width="1.8" stroke-linejoin="round"/></svg>',
    sms: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor"><path d="M19 4H5a2 2 0 0 0-2 2v10a2 2 0 0 0 2 2h4l3 3 3-3h4a2 2 0 0 0 2-2V6a2 2 0 0 0-2-2Z" stroke-width="1.8"/><path d="M7 9h10M7 13h6" stroke-width="1.8" stroke-linecap="round"/></svg>',
	    feishu: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor"><path d="M7 5.5 12 3l5 2.5v5L12 13 7 10.5v-5ZM7 13.5l5 2.5 5-2.5v5L12 21l-5-2.5v-5Z" stroke-width="1.7" stroke-linejoin="round"/></svg>',
	    qq: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor"><path d="M8 9c0-4 1.8-6 4-6s4 2 4 6c0 1.5.5 3 1.5 4.5.8 1.2 1.2 2.4.5 3.5-.4.6-1 .8-1.8.7-.7 2-2.2 3.3-4.2 3.3s-3.5-1.3-4.2-3.3c-.8.1-1.4-.1-1.8-.7-.7-1.1-.3-2.3.5-3.5C7.5 12 8 10.5 8 9Z" stroke-width="1.7"/></svg>',
	    dingtalk: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor"><path d="M5 5h14v10H9l-4 4V5Z" stroke-width="1.8" stroke-linejoin="round"/><path d="M8 9h8M8 12h5" stroke-width="1.8" stroke-linecap="round"/></svg>',
  }
  return icons[channel]
}
let qqRefreshTimer: number | undefined
onMounted(async () => { await load(); await loadBrand(); qqRefreshTimer = window.setInterval(() => { if (qqBindCode.value) load(true) }, 3000) })
onBeforeUnmount(() => { if (qqRefreshTimer) window.clearInterval(qqRefreshTimer) })
</script>

<style scoped>
.channel-card { @apply rounded-[24px] border border-border/70 bg-surface/85 p-5 shadow-[0_16px_44px_-34px_rgba(15,23,42,.55)] backdrop-blur-xl transition hover:-translate-y-0.5 hover:border-brand-500/15; }
.channel-icon { @apply flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl; }
.channel-icon :deep(svg) { @apply h-6 w-6; }
.channel-icon.inapp { @apply bg-blue-500/10 text-blue-500; }.channel-icon.email { @apply bg-violet-500/10 text-violet-500; }.channel-icon.sms { @apply bg-amber-500/10 text-amber-500; }.channel-icon.feishu { @apply bg-cyan-500/10 text-cyan-500; }.channel-icon.qq { @apply bg-rose-500/10 text-rose-500; }.channel-icon.dingtalk { @apply bg-sky-500/10 text-sky-500; }
.status-pill { @apply rounded-full px-2 py-0.5 text-[9px] font-extrabold; }.status-pill.success { @apply bg-emerald-500/10 text-emerald-500; }.status-pill.warning { @apply bg-amber-500/10 text-amber-500; }.status-pill.muted { @apply bg-muted text-muted-foreground; }
.toggle { @apply relative h-7 w-12 shrink-0 rounded-full bg-muted transition; }.toggle span { @apply absolute left-1 top-1 h-5 w-5 rounded-full bg-white shadow transition; }.toggle.on { @apply bg-emerald-500; }.toggle.on span { transform: translateX(20px); }
.bind-input { @apply h-11 min-w-0 flex-1 rounded-2xl border border-border bg-surface px-3.5 text-sm outline-none focus:border-brand-500/40 focus:ring-4 focus:ring-brand-500/10; }
.bind-button { @apply h-11 rounded-2xl bg-brand-500 px-4 text-sm font-bold text-white disabled:opacity-50; }
.channel-button { @apply flex-1 rounded-xl border border-border bg-surface px-3 py-2 text-xs font-bold text-foreground transition hover:bg-muted disabled:opacity-45; }.channel-button.primary { @apply border-brand-500/20 bg-brand-500/10 text-brand-600 dark:text-brand-300; }.channel-button.danger { @apply text-rose-500; }
.provider-form { @apply mt-5 rounded-2xl border border-violet-500/15 bg-violet-500/[.05] p-4; }.provider-help { @apply mt-1 text-xs leading-5 text-muted-foreground; }
.provider-field { @apply block; }.provider-field-label { @apply flex items-center gap-2 text-xs font-bold text-foreground; }.provider-badge { @apply rounded-full px-1.5 py-0.5 text-[9px] font-bold; }.provider-badge.required { @apply bg-rose-500/10 text-rose-500; }.provider-badge.recommended { @apply bg-emerald-500/10 text-emerald-600 dark:text-emerald-300; }.provider-badge.optional { @apply bg-muted text-muted-foreground; }.provider-field-help { @apply mt-1 text-[11px] leading-4 text-muted-foreground; }
.modal-label { @apply block text-sm font-bold text-foreground; }.modal-label span { @apply text-rose-500; }.modal-label small { @apply ml-1 text-xs font-medium text-muted-foreground; }.modal-field { @apply h-12 w-full rounded-2xl border border-border bg-surface px-4 text-sm font-normal outline-none focus:border-brand-500 focus:ring-4 focus:ring-brand-500/10; }
.modal-help { @apply mt-1 text-[11px] leading-4 text-muted-foreground; }
</style>
