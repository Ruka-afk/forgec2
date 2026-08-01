"use client"

import { useCallback, useEffect, useRef } from "react"

const COLORS = ["#ff6b6b", "#4ecdc4", "#45b7d1", "#f9ca24", "#6c5ce7", "#fd79a8", "#00cec9", "#e17055"]

export function useConfetti() {
  const timersRef = useRef<ReturnType<typeof setTimeout>[]>([]);

  const burst = useCallback((x: number, y: number, count = 40) => {
    const container = document.createElement("div")
    container.style.cssText = "position:fixed;inset:0;pointer-events:none;z-index:9999"
    document.body.appendChild(container)

    for (let i = 0; i < count; i++) {
      const el = document.createElement("div")
      const color = COLORS[Math.floor(Math.random() * COLORS.length)]
      const size = Math.random() * 8 + 4
      const angle = Math.random() * Math.PI * 2
      const velocity = Math.random() * 200 + 100
      const dx = Math.cos(angle) * velocity
      const dy = Math.sin(angle) * velocity - 100
      const rotation = Math.random() * 720 - 360

      el.style.cssText = `
        position:absolute;left:${x}px;top:${y}px;width:${size}px;height:${size}px;
        background:${color};border-radius:${Math.random() > 0.5 ? "50%" : "2px"};
        pointer-events:none;opacity:1;
        transition:all ${0.8 + Math.random() * 0.4}s cubic-bezier(0.25,0.46,0.45,0.94);
        transform:translate(0,0) rotate(0deg);
      `
      container.appendChild(el)

      requestAnimationFrame(() => {
        el.style.transform = `translate(${dx}px, ${dy + 300}px) rotate(${rotation}deg)`
        el.style.opacity = "0"
      })
    }

    const timer = setTimeout(() => container.remove(), 2000)
    timersRef.current.push(timer)
  }, [])

  useEffect(() => {
    const timers = timersRef.current
    return () => { timers.forEach(clearTimeout); }
  }, [])

  return burst
}
