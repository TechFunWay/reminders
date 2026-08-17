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
  type: 'connected' | 'notification.created' | string
  notification?: RealtimeNotification
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
        const response = await fetch(new URL('api/reminder/events', document.baseURI), {
          headers: {
            Accept: 'text/event-stream',
            Authorization: `Bearer ${token}`,
          },
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
