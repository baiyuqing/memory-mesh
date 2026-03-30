import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@examples': path.resolve(__dirname, '../deploy/examples'),
    },
  },
  server: {
    fs: {
      allow: ['.', '../deploy/examples'],
    },
    proxy: {
      '/v1': 'http://localhost:8080',
    },
  },
})
