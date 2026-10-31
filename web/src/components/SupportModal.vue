<template>
  <Teleport to="body">
    <div v-if="support.show">
      <!-- 主弹窗：精简为一屏展示（mobile-ui 规范 2026-10-01） -->
      <div class="fixed inset-0 z-[60] overflow-hidden overscroll-contain bg-black/50" @click="support.dismiss()">
        <div class="flex min-h-full items-center justify-center p-4">
          <div role="dialog" aria-modal="true" aria-labelledby="support-modal-title" class="surface-modal relative w-full max-w-[320px] overflow-hidden rounded-2xl text-center" @click.stop>
          <button
            class="absolute right-2.5 top-2.5 z-10 rounded-lg p-1.5 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
            aria-label="关闭"
            @click="support.dismiss()"
          >
            <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" /></svg>
          </button>

          <div class="px-5 pb-4 pt-5">
            <div class="mx-auto mb-2 flex h-11 w-11 items-center justify-center rounded-xl bg-brand-gradient shadow-glow">
              <span class="text-xl">☕</span>
            </div>
            <h3 id="support-modal-title" class="mb-1.5 text-base font-bold text-foreground">请作者喝杯咖啡</h3>

            <p class="mb-3 text-xs leading-relaxed text-muted-foreground">
              应用免费无广告、数据保存在本机。若对你有帮助，欢迎赞赏——<strong class="text-foreground">金额随意，1 元也是心意</strong>。
            </p>

            <div class="my-2 flex justify-center">
              <div>
                <img
                  :src="donateQr"
                  alt="微信赞赏码"
                  width="352"
                  height="480"
                  decoding="async"
                  class="mx-auto h-auto w-[150px] max-w-full rounded-lg border border-border bg-white object-contain p-1"
                />
                <span class="mt-1 block text-[11px] text-muted-foreground">微信扫码赞赏</span>
              </div>
            </div>

            <p v-if="support.errorText" class="mt-1.5 text-xs text-destructive">{{ support.errorText }}</p>

            <div class="mt-3 flex justify-center gap-2.5">
              <button class="btn-ghost flex-1 py-2 text-sm" :disabled="support.sending" @click="support.dismiss()">
                暂不支持
              </button>
              <button class="btn-brand flex-1 py-2 text-sm" :disabled="support.sending" @click="openAmount">
                <svg v-if="!support.sending" class="h-3.5 w-3.5" fill="currentColor" viewBox="0 0 24 24"><path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/></svg>
                {{ support.sending ? '发送中…' : '已支持' }}
              </button>
            </div>
          </div>
          </div>
        </div>
      </div>

      <!-- 金额输入弹窗（点击「已支持」后出现，浮在主弹窗之上） -->
      <div v-if="amountDialog" class="fixed inset-0 z-[70] flex items-center justify-center p-4" @click.self="closeAmount">
        <div class="absolute inset-0 bg-black/50"></div>
        <div class="surface relative w-full max-w-[300px] rounded-2xl p-4 text-center">
          <h4 class="text-sm font-bold text-foreground">填写赞赏金额</h4>
          <p class="mt-1 text-[11px] text-muted-foreground">仅用于统计，不做支付核验</p>
          <div class="mt-2.5 flex flex-wrap justify-center gap-1.5">
            <button
              v-for="p in presets" :key="p"
              class="rounded-full border border-border bg-muted/40 px-2.5 py-1 text-xs font-medium text-foreground transition-colors hover:bg-muted"
              :class="{ '!border-brand-500 !bg-brand-500/10 !text-brand-600 dark:!text-brand-300': amount === p }"
              @click="amount = p"
            >
              {{ p }} 元
            </button>
          </div>
          <div class="mt-2.5 flex items-center justify-center gap-1.5">
            <span class="text-xs text-muted-foreground">或输入</span>
            <input
              v-model.number="customAmount"
              type="number"
              min="0"
              step="0.01"
              placeholder="金额（元）"
              class="input-field w-24 text-center text-sm"
              @input="amount = Number(customAmount)"
            />
          </div>
          <p v-if="amount === 0" class="mt-1.5 text-[11px] text-amber-500">未填写将以 0 元上报</p>
          <div class="mt-3 flex gap-2">
            <button class="btn-ghost flex-1 py-1.5 text-sm" @click="closeAmount">取消</button>
            <button class="btn-brand flex-1 py-1.5 text-sm" :disabled="support.sending" @click="confirmAmount">发送</button>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useSupportStore } from '../stores/support'
import donateQr from '../assets/donate-wechat.png'

const support = useSupportStore()

// 金额输入弹窗状态
const amountDialog = ref(false)
const presets = [1, 5, 10, 50]
const customAmount = ref<number | ''>('')
const amount = ref<number>(0)

function openAmount() {
  amountDialog.value = true
  customAmount.value = ''
  amount.value = 0
}

function closeAmount() {
  amountDialog.value = false
  customAmount.value = ''
  amount.value = 0
}

function confirmAmount() {
  if (support.sending) return
  amountDialog.value = false
  void support.confirmSupported(amount.value || 0)
}
</script>
