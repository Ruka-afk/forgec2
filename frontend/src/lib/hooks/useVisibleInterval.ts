import { useEffect, useRef } from "react"

/**
 * Fires `callback` every `delay` ms, but pauses when the browser tab is hidden.
 * When the tab becomes visible again, it fires an immediate catch-up tick.
 */
export function useVisibleInterval(callback: () => void, delay: number) {
  const savedCallback = useRef(callback)
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null)

  useEffect(() => {
    savedCallback.current = callback
  }, [callback])

  useEffect(() => {
    if (delay <= 0) return

    const tick = () => {
      savedCallback.current()
    }

    const start = () => {
      if (timerRef.current) clearInterval(timerRef.current)
      timerRef.current = setInterval(tick, delay)
    }

    const handleVisibility = () => {
      if (document.hidden) {
        if (timerRef.current) clearInterval(timerRef.current)
        timerRef.current = null
      } else {
        tick()
        start()
      }
    }

    // A page can be mounted in a background tab. Do not create a timer until
    // it becomes visible; otherwise the first visibilitychange arrives only
    // after needless background wakeups have already occurred.
    if (!document.hidden) start()
    document.addEventListener("visibilitychange", handleVisibility)

    return () => {
      if (timerRef.current) clearInterval(timerRef.current)
      document.removeEventListener("visibilitychange", handleVisibility)
    }
  }, [delay])
}
