import { resolve } from 'node:path'
import { writeFileSync } from 'node:fs'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
    {
      // dist di-embed Go (//go:embed all:dist): pertahankan .gitkeep agar embed tidak gagal pada checkout baru.
      name: 'keep-dist',
      closeBundle() {
        writeFileSync(resolve(__dirname, 'dist/.gitkeep'), '')
      },
    },
  ],
  build: {
    outDir: 'dist',
    rollupOptions: {
      input: {
        backoffice: resolve(__dirname, 'backoffice/index.html'),
        pos: resolve(__dirname, 'pos/index.html'),
      },
    },
  },
  server: { proxy: { '/api': 'http://localhost:8080' } },
})
