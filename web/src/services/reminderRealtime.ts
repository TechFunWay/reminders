import { onFnOSGatewayOrigin } from '../utils/gateway'
import { clientID } from '../utils/client-id'

export interface RealtimeNotification {
  id: number
  reminder_id?: number
  type: string
  title: string
  body: string
  read_at?: string
  created_at: string
}

export interface ReminderRealtimeEvent {
  /** notification.created（站内通知）/ reminders.changed（数据变更）/ connected */
  type: 'connected' | 'notification.created' | 'reminders.changed' | string
  notification?: RealtimeNotification
  /** reminders.changed：哪类数据变了（reminder / list / notification / channel）。 */
  scope?: string
  /** reminders.changed：发生了什么（created / updated / deleted / completed…）。 */
  action?: string
  target_id?: number
  /** 该用户的变更修订号，单调递增。 */
  revision?: number
  sent_at: string
}

const wait = (milliseconds: number) => new Promise(resolve => setTimeout(resolve, milliseconds))

export function connectReminderEvents(
  token: string,
  onEvent: (event: ReminderRealtimeEvent) => void,
): () => void {
  let stopped = false
  let controller: AbortController | undefined

  async function run() {
    let retryDelay = 1000
    while (!stopped) {
      controller = new AbortController()
      try {
        // 网关域不带应用自己的 Authorization（见 utils/gateway.ts）：那条路上
        // 的登录态由服务端用网关注入的 X-Trim-* 身份解析。
        const headers: Record<string, string> = { Accept: 'text/event-stream' }
        if (token && !onFnOSGatewayOrigin()) headers.Authorization = `Bearer ${token}`
        // client_id 让服务端广播数据变更时跳过本标签页（它已经在本地改过了）。
        const url = new URL('api/reminder/events', document.baseURI)
        const id = clientID()
        if (id) url.searchParams.set('client_id', id)
        const response = await fetch(url, {
          headers,
          cache: 'no-store',
          signal: controller.signal,
        })
        if (!response.ok || !response.body) {
          throw new Error(`SSE connection failed: ${response.status}`)
        }

        retryDelay = 1000
        const reader = response.body.getReader()
        const decoder = new TextDecoder()
        let buffer = ''

        while (!stopped) {
          const { done, value } = await reader.read()
          if (done) break
          buffer += decoder.decode(value, { stream: true }).replace(/\r\n/g, '\n')

          let boundary = buffer.indexOf('\n\n')
          while (boundary >= 0) {
            const block = buffer.slice(0, boundary)
            buffer = buffer.slice(boundary + 2)
            const data = block
              .split('\n')
              .filter(line => line.startsWith('data:'))
              .map(line => line.slice(5).trimStart())
              .join('\n')
            if (data) {
              try {
                onEvent(JSON.parse(data) as ReminderRealtimeEvent)
              } catch {
                // Ignore malformed events and keep the long connection alive.
              }
            }
            boundary = buffer.indexOf('\n\n')
          }
        }
      } catch (error) {
        if (stopped || (error instanceof DOMException && error.name === 'AbortError')) return
      }

      if (!stopped) {
        await wait(retryDelay)
        retryDelay = Math.min(retryDelay * 2, 15_000)
      }
    }
  }

  void run()
  return () => {
    stopped = true
    controller?.abort()
  }
}
