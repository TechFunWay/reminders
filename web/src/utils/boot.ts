// 首屏骨架（index.html 内联）的收尾工具。
//
// 骨架的存在意义是「解析完 HTML 就有画面」：入口包下载、启动请求往返、路由
// 守卫判定这段时间里，用户看到的是应用图标与提示文案，而不是一片白。等到
// 首个路由真正渲染出来，再把骨架淡出移除。
//
// 启动异常时骨架同时承担提示作用：服务还在初始化（503）或网络不通时，把
// 原因写在骨架上，避免用户对着「正在加载…」干等。

const SPLASH_ID = 'boot-splash'
const HINT_ID = 'boot-hint'
const FADE_MS = 240

export function setBootHint(text: string): void {
  const hint = document.getElementById(HINT_ID)
  if (hint) hint.textContent = text
}

export function dismissBootSplash(): void {
  const splash = document.getElementById(SPLASH_ID)
  if (!splash) return
  splash.classList.add('boot-done')
  const remove = () => splash.remove()
  try {
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
      remove()
      return
    }
  } catch {
    remove()
    return
  }
  window.setTimeout(remove, FADE_MS + 60)
}
