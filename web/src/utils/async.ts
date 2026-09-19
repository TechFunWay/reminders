// 启动阶段的请求必须有界：任何一种（弱网、网关挂起、服务未就绪）都不该
// 让首屏无限等待而停在白屏。超时后按 fallback 继续，后台请求本身不取消，
// 其成功结果仍可被后续调用采用。
export function withTimeout<T>(promise: Promise<T>, ms: number, fallback: T): Promise<T> {
  return new Promise<T>((resolve) => {
    const timer = setTimeout(() => resolve(fallback), ms)
    promise.then(
      (value) => {
        clearTimeout(timer)
        resolve(value)
      },
      () => {
        clearTimeout(timer)
        resolve(fallback)
      },
    )
  })
}
