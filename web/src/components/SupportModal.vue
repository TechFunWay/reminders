<template>
  <Teleport to="body">
    <div v-if="support.show">
      <!-- 主弹窗：赞赏说明 + 收款码 + 暂不支持/已支持按钮。外层可滚动：内容高于
           小屏（尤其手机）时弹窗内部滚动；点击遮罩或右上角关闭。 -->
      <div class="fixed inset-0 z-[60] overflow-y-auto overscroll-contain bg-black/50" @click="support.dismiss()">
        <div class="flex min-h-full items-center justify-center p-4">
          <div role="dialog" aria-modal="true" aria-labelledby="support-modal-title" class="surface-modal relative my-auto w-full max-w-sm overflow-hidden rounded-2xl text-center" @click.stop>
          <button
            class="absolute right-3 top-3 z-10 rounded-lg p-1.5 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
            aria-label="关闭"
            @click="support.dismiss()"
          >
            <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" /></svg>
          </button>

          <div class="px-4 pb-5 pt-5 sm:px-6 sm:pb-6 sm:pt-7">
            <div class="mx-auto mb-3 flex h-14 w-14 items-center justify-center rounded-2xl bg-brand-gradient shadow-glow">
              <span class="text-2xl">☕</span>
            </div>
            <h3 id="support-modal-title" class="mb-2 text-lg font-bold text-foreground">请作者喝杯咖啡</h3>

            <p class="mb-1 text-sm leading-relaxed text-muted-foreground">
              这个应用免费、无广告，数据完全保存在你自己的设备上。
            </p>
            <p class="mb-3 text-sm leading-relaxed text-muted-foreground sm:mb-4">
              如果它帮到了你，欢迎请作者喝杯咖啡——<strong class="text-foreground">金额随意，1 元也是心意</strong>。
              <br />
              <span class="text-xs">不赞赏也完全没有问题，<strong>不支付不影响任何功能</strong>。</span>
            </p>

            <div class="my-3 flex justify-center sm:my-4">
              <div>
                <!-- 明确宽高属性：加载前就占好位，避免图片解码后弹窗高度跳动；
                     max-h 限制高度，矮屏下弹窗不被收款码撑出屏幕 -->
                <img
                  :src="donateQr"
                  alt="微信赞赏码"
                  width="352"
                  height="480"
                  decoding="async"
                  class="mx-auto h-auto max-h-[32vh] w-[190px] max-w-full rounded-xl border border-border bg-white object-contain p-1"
                />
                <span class="mt-2 block text-xs text-muted-foreground">微信扫码赞赏</span>
              </div>
            </div>

            <p v-if="support.errorText" class="mt-2 text-xs text-destructive">{{ support.errorText }}</p>

            <div class="mt-4 flex justify-center gap-3">
              <button class="btn-ghost flex-1" :disabled="support.sending" @click="support.dismiss()">
                暂不支持
              </button>
              <button class="btn-brand flex-1" :disabled="support.sending" @click="openAmount">
                <svg v-if="!support.sending" class="h-4 w-4" fill="currentColor" viewBox="0 0 24 24"><path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/></svg>
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
        <div class="surface relative w-full max-w-xs rounded-2xl p-5 text-center">
          <h4 class="text-base font-bold text-foreground">填写赞赏金额</h4>
          <p class="mt-1 text-xs text-muted-foreground">金额随意，1 元也是心意；仅用于接收端统计，不做支付核验</p>
          <div class="mt-3 flex flex-wrap justify-center gap-2">
            <button
              v-for="p in presets" :key="p"
              class="rounded-full border border-border bg-muted/40 px-3 py-1 text-sm font-medium text-foreground transition-colors hover:bg-muted"
              :class="{ '!border-brand-500 !bg-brand-500/10 !text-brand-600 dark:!text-brand-300': amount === p }"
              @click="amount = p"
            >
              {{ p }} 元
            </button>
          </div>
          <div class="mt-3 flex items-center justify-center gap-1.5">
            <span class="text-sm text-muted-foreground">或输入</span>
            <input
              v-model.number="customAmount"
              type="number"
              min="0"
              step="0.01"
              placeholder="金额（元）"
              class="input-field w-28 text-center"
              @input="amount = Number(customAmount)"
            />
          </div>
          <p v-if="amount === 0" class="mt-2 text-xs text-amber-500">未填写金额将以 0 元上报</p>
          <div class="mt-4 flex gap-3">
            <button class="btn-ghost flex-1" @click="closeAmount">取消</button>
            <button class="btn-brand flex-1" :disabled="support.sending" @click="confirmAmount">发送</button>
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
