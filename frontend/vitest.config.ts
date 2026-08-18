import { defineConfig } from 'vitest/config'
import path from 'path'

export default defineConfig({
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: [],
    include: ['src/**/*.test.{ts,tsx}'],
    exclude: ['node_modules', 'e2e', 'out'],
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src'),
      'next/link': path.resolve(__dirname, 'src/lib/next/link.tsx'),
      'next/navigation': path.resolve(__dirname, 'src/lib/next/navigation.ts'),
      'next/dynamic': path.resolve(__dirname, 'src/lib/next/dynamic.tsx'),
    },
  },
})
