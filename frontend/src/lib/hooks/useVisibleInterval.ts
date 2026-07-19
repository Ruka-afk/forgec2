import { useEffect, useRef } from "react"

/**
 * Fires `callback` every `delay` ms, but pauses when the browser tab is hidden.
 * When the tab becomes visible again, it fires an immediate catch-up tick.
 */
export function useVisibleInterval(callback: () => void, delay: number) {
  const savedCallback = useRef(callback)
  const remainingRef = useRef(delay)
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null)

  useEffect(() => {
    savedCallback.current = callback
  }, [callback])

  useEffect(() => {
    if (delay <= 0) return

    const tick = () => {
      remainingRef.current = delay
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

    start()
    document.addEventListener("visibilitychange", handleVisibility)

    return () => {
      if (timerRef.current) clearInterval(timerRef.current)
      document.removeEventListener("visibilitychange", handleVisibility)
    }
  }, [delay])
}
