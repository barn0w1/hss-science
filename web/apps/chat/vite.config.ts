import { defineConfig } from 'vite'
import { fileURLToPath, URL } from 'node:url'
import react from '@vitejs/plugin-react-swc'
import tailwindcss from '@tailwindcss/vite'
import { VitePWA } from 'vite-plugin-pwa';

// https://vite.dev/config/
export default defineConfig({
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  plugins: [
    react(),
    tailwindcss(),
    VitePWA({
      registerType: 'autoUpdate', // 自動で最新版に更新する
      injectRegister: 'auto', // サービスワーカーを自動登録
      manifest: {
        name: 'Chat',
        short_name: 'Chat',
        description: 'Chat application for HSS Science',
        theme_color: '#ffffff', // アプリのヘッダーなどのテーマカラー
        background_color: '#ffffff', // 起動時の背景色
        display: 'standalone', // ブラウザのUIを消してネイティブアプリのように見せる
        icons: [
          {
            src: '/pwa-256x256.png',
            sizes: '256x256',
            type: 'image/png'
          },
          {
            src: '/pwa-512x512.png',
            sizes: '512x512',
            type: 'image/png'
          },
          {
            src: '/pwa-512x512.png',
            sizes: '512x512',
            type: 'image/png',
            purpose: 'any maskable' // Androidのアイコンを綺麗に切り抜くため
          }
        ]
      }
    }),
  ],
})