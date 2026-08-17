<template>
  <div class="auth-root">
    <!-- Immersive background layers -->
    <div class="bg-base"></div>
    <div class="aurora aurora-1"></div>
    <div class="aurora aurora-2"></div>
    <div class="aurora aurora-3"></div>
    <div class="grid-overlay"></div>
    <div class="grain"></div>
    <div class="vignette"></div>

    <!-- Content -->
    <div class="auth-content relative z-10 flex flex-col items-center justify-center px-6 py-12">
      <div class="w-full max-w-md">
        <!-- Brand -->
        <div class="flex flex-col items-center text-center mb-7 animate-fade-in">
          <div class="logo-badge">
            <span class="font-display font-extrabold text-xl text-white">{{ siteInitial }}</span>
          </div>
          <h1 class="mt-4 font-display text-lg font-bold tracking-tight text-white/90">{{ authStore.siteTitle }}</h1>
        </div>

        <!-- Glass card -->
        <div class="card-wrap animate-scale-in">
          <div class="card-glow"></div>
          <div class="auth-card">
            <div class="relative z-10">
              <h2 class="font-display text-[1.75rem] leading-tight font-extrabold tracking-tight text-white">{{ title }}</h2>
              <p v-if="subtitle" class="mt-2 text-sm text-white/50">{{ subtitle }}</p>
              <div class="mt-7">
                <slot />
              </div>
            </div>
          </div>
        </div>

        <!-- Footer -->
        <div v-if="$slots.footer" class="mt-7 text-center text-sm text-white/50">
          <slot name="footer" />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useAuthStore } from '../../stores/auth'

defineProps<{ title: string; subtitle?: string }>()

const authStore = useAuthStore()
const siteInitial = computed(() => (authStore.siteTitle || 'S').charAt(0).toUpperCase())
</script>

<style scoped>
@property --bd-angle {
  syntax: '<angle>';
  initial-value: 0deg;
  inherits: false;
}

.auth-root {
  position: relative;
  min-height: 100vh;
  min-height: 100dvh;
  overflow-x: hidden;
  overflow-y: auto;
  background: #05050b;
  color: #fff;
}

.auth-content {
  min-height: 100vh;
  min-height: 100dvh;
}

/* base radial wash */
.bg-base {
  position: fixed;
  inset: 0;
  background:
    radial-gradient(125% 125% at 50% -10%, #14142a 0%, #0a0a16 45%, #05050b 100%);
}

/* drifting aurora orbs */
.aurora {
  position: fixed;
  border-radius: 9999px;
  filter: blur(90px);
  will-change: transform;
}
.aurora-1 {
  width: 42rem;
  height: 42rem;
  top: -12rem;
  left: -8rem;
  background: radial-gradient(circle at 30% 30%, rgba(99, 102, 241, 0.55), transparent 70%);
  animation: float-1 20s ease-in-out infinite;
}
.aurora-2 {
  width: 38rem;
  height: 38rem;
  bottom: -14rem;
  right: -10rem;
  background: radial-gradient(circle at 70% 70%, rgba(168, 85, 247, 0.5), transparent 70%);
  animation: float-2 24s ease-in-out infinite;
}
.aurora-3 {
  width: 30rem;
  height: 30rem;
  top: 40%;
  left: 45%;
  background: radial-gradient(circle at 50% 50%, rgba(34, 211, 238, 0.35), transparent 70%);
  animation: float-3 28s ease-in-out infinite;
}
@keyframes float-1 {
  0%, 100% { transform: translate(0, 0) scale(1); }
  50% { transform: translate(5rem, 4rem) scale(1.12); }
}
@keyframes float-2 {
  0%, 100% { transform: translate(0, 0) scale(1); }
  50% { transform: translate(-6rem, -3rem) scale(1.06); }
}
@keyframes float-3 {
  0%, 100% { transform: translate(-50%, -50%) scale(1); }
  50% { transform: translate(-58%, -42%) scale(1.18); }
}

/* faint grid that fades toward the edges */
.grid-overlay {
  position: fixed;
  inset: 0;
  background-image:
    linear-gradient(rgba(255, 255, 255, 0.035) 1px, transparent 1px),
    linear-gradient(90deg, rgba(255, 255, 255, 0.035) 1px, transparent 1px);
  background-size: 58px 58px;
  -webkit-mask: radial-gradient(circle at 50% 42%, #000 0%, transparent 72%);
  mask: radial-gradient(circle at 50% 42%, #000 0%, transparent 72%);
}

/* fine film grain */
.grain {
  position: fixed;
  inset: 0;
  opacity: 0.16;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='160' height='160'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.85' numOctaves='3' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23n)'/%3E%3C/svg%3E");
}

/* edge darkening */
.vignette {
  position: fixed;
  inset: 0;
  pointer-events: none;
  background: radial-gradient(125% 110% at 50% 50%, transparent 48%, rgba(0, 0, 0, 0.55) 100%);
}

/* card + glow */
.card-wrap {
  position: relative;
}
.card-glow {
  position: absolute;
  inset: -28px;
  z-index: 0;
  border-radius: 40px;
  background: radial-gradient(60% 55% at 50% 0%, rgba(124, 92, 255, 0.45), transparent 72%);
  filter: blur(46px);
}
.auth-card {
  position: relative;
  z-index: 1;
  border-radius: 26px;
  padding: 2.25rem;
  background: rgba(18, 18, 30, 0.55);
  backdrop-filter: blur(26px) saturate(140%);
  -webkit-backdrop-filter: blur(26px) saturate(140%);
  border: 1px solid rgba(255, 255, 255, 0.07);
  box-shadow:
    0 30px 80px -24px rgba(0, 0, 0, 0.75),
    inset 0 1px 0 rgba(255, 255, 255, 0.06);
  overflow: hidden;
}
/* rotating conic flow border */
.auth-card::before {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: inherit;
  padding: 1px;
  background: conic-gradient(
    from var(--bd-angle),
    transparent 0%,
    rgba(99, 102, 241, 0) 18%,
    #6366f1 32%,
    #a855f7 44%,
    #22d3ee 56%,
    rgba(34, 211, 238, 0) 70%,
    transparent 100%
  );
  -webkit-mask: linear-gradient(#000 0 0) content-box, linear-gradient(#000 0 0);
  -webkit-mask-composite: xor;
  mask-composite: exclude;
  animation: border-rotate 7s linear infinite;
  pointer-events: none;
}
@keyframes border-rotate {
  to { --bd-angle: 360deg; }
}

/* logo */
.logo-badge {
  width: 3.5rem;
  height: 3.5rem;
  border-radius: 1.1rem;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #6366f1 0%, #8b5cf6 50%, #a855f7 100%);
  box-shadow:
    0 0 0 1px rgba(255, 255, 255, 0.12),
    0 12px 36px -8px rgba(124, 92, 255, 0.7);
}

/* Embedded mobile WebViews repaint fixed blur layers while the keyboard
   animates the visual viewport. Keep the desktop treatment, but use a stable
   static composition on touch devices so focusing an input stays responsive. */
@media (max-width: 767px), (hover: none) and (pointer: coarse) {
  .auth-content {
    min-height: 100dvh;
    justify-content: flex-start;
    padding-top: max(2rem, env(safe-area-inset-top));
    padding-bottom: max(2rem, env(safe-area-inset-bottom));
  }

  .bg-base,
  .vignette {
    position: absolute;
  }

  .aurora,
  .grid-overlay,
  .grain,
  .card-glow {
    display: none;
  }

  .auth-card {
    background: rgba(18, 18, 30, 0.96);
    backdrop-filter: none;
    -webkit-backdrop-filter: none;
  }

  .auth-card::before {
    display: none;
  }

  .animate-fade-in,
  .animate-scale-in {
    animation: none;
  }
}

@media (prefers-reduced-motion: reduce) {
  .aurora { animation: none; }
  .auth-card::before { animation: none; }
  .animate-fade-in, .animate-scale-in { animation: none; }
}

@media (prefers-reduced-transparency: reduce) {
  .auth-card {
    background: rgba(18, 18, 30, 0.96);
    backdrop-filter: none;
    -webkit-backdrop-filter: none;
  }
}
</style>
