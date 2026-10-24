import path from 'path'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./vitest.setup.ts'],
    coverage: {
      provider: 'v8',
      // SonarQube Cloud が読む lcov と、手元で見る text
      reporter: ['lcov', 'text-summary'],
      reportsDirectory: 'coverage',
      include: ['{app,features,shared,widgets}/**/*.{ts,tsx}'],
      exclude: ['**/*.test.{ts,tsx}', 'generate/**'],
    },
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, '.'),
    },
  },
})
