import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'
import { visualizer } from 'rollup-plugin-visualizer'
import viteCompression from 'vite-plugin-compression'

// https://vitejs.dev/config/
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const isAnalyze = mode === 'analyze'

  return {
    plugins: [
      react(),
      // Gzip compression for production builds
      viteCompression({
        algorithm: 'gzip',
        ext: '.gz',
        threshold: 10240, // Only compress files > 10KB
      }),
      // Brotli compression (better compression ratio)
      viteCompression({
        algorithm: 'brotliCompress',
        ext: '.br',
        threshold: 10240,
      }),
      // Bundle analyzer (only in analyze mode)
      isAnalyze && visualizer({
        filename: 'dist/stats.html',
        open: true,
        gzipSize: true,
        brotliSize: true,
        template: 'treemap', // 'sunburst', 'treemap', 'network'
      }),
    ].filter(Boolean),

    server: {
      port: 5180,
      proxy: {
        '/api': {
          target: 'http://localhost:8080',
          changeOrigin: true,
        },
        '/health': {
          target: 'http://localhost:8080',
          changeOrigin: true,
        },
        '/ws': {
          target: 'ws://localhost:8080',
          ws: true,
          changeOrigin: true,
        },
      },
    },

    resolve: {
      alias: {
        '@': path.resolve(__dirname, './src'),
        '@components': path.resolve(__dirname, './src/components'),
        '@hooks': path.resolve(__dirname, './src/hooks'),
        '@services': path.resolve(__dirname, './src/services'),
        '@types': path.resolve(__dirname, './src/types'),
        '@utils': path.resolve(__dirname, './src/utils'),
        '@styles': path.resolve(__dirname, './src/styles'),
      },
    },

    build: {
      // Enable source maps only in development
      sourcemap: mode !== 'production',

      // Target modern browsers for smaller bundles
      target: 'es2020',

      // Increase chunk warning limit slightly
      chunkSizeWarningLimit: 600,

      // Minification settings
      minify: 'terser',
      terserOptions: {
        compress: {
          drop_console: mode === 'production',
          drop_debugger: mode === 'production',
          pure_funcs: mode === 'production' ? ['console.log', 'console.info'] : [],
        },
        format: {
          comments: false,
        },
      },

      rollupOptions: {
        output: {
          // Optimize chunk naming for better caching
          chunkFileNames: 'assets/js/[name]-[hash].js',
          entryFileNames: 'assets/js/[name]-[hash].js',
          assetFileNames: 'assets/[ext]/[name]-[hash].[ext]',

          // Advanced manual chunks for optimal code splitting
          manualChunks: (id) => {
            // React core - rarely changes, cache long
            if (id.includes('node_modules/react/') ||
                id.includes('node_modules/react-dom/') ||
                id.includes('node_modules/scheduler/')) {
              return 'react-core'
            }

            // React Router - separate chunk
            if (id.includes('node_modules/react-router')) {
              return 'react-router'
            }

            // Monaco Editor - very large, load separately
            if (id.includes('node_modules/monaco-editor') ||
                id.includes('node_modules/@monaco-editor')) {
              return 'monaco'
            }

            // xterm - terminal emulator, load on demand
            if (id.includes('node_modules/xterm') ||
                id.includes('node_modules/@xterm')) {
              return 'terminal'
            }

            // Framer Motion - animations
            if (id.includes('node_modules/framer-motion')) {
              return 'animations'
            }

            // Lucide icons - tree-shake well but group together
            if (id.includes('node_modules/lucide-react')) {
              return 'icons'
            }

            // Zustand + Immer - state management
            if (id.includes('node_modules/zustand') ||
                id.includes('node_modules/immer')) {
              return 'state'
            }

            // Socket.io - real-time communication
            if (id.includes('node_modules/socket.io')) {
              return 'realtime'
            }

            // Axios - HTTP client
            if (id.includes('node_modules/axios')) {
              return 'http'
            }

            // Syntax highlighter - code display
            if (id.includes('node_modules/react-syntax-highlighter') ||
                id.includes('node_modules/prismjs') ||
                id.includes('node_modules/refractor')) {
              return 'syntax-highlight'
            }

            // Date utilities
            if (id.includes('node_modules/date-fns')) {
              return 'date-utils'
            }

            // UI utilities (clsx, tailwind-merge, class-variance-authority)
            if (id.includes('node_modules/clsx') ||
                id.includes('node_modules/tailwind-merge') ||
                id.includes('node_modules/class-variance-authority')) {
              return 'ui-utils'
            }

            // Other vendor code
            if (id.includes('node_modules')) {
              return 'vendor'
            }

            // IDE components - lazy loaded
            if (id.includes('/components/ide/')) {
              return 'ide'
            }

            // Builder components - lazy loaded
            if (id.includes('/components/builder/')) {
              return 'builder'
            }

            // Admin components - lazy loaded
            if (id.includes('/components/admin/')) {
              return 'admin'
            }

            // AI components
            if (id.includes('/components/ai/')) {
              return 'ai'
            }

            // Editor components
            if (id.includes('/components/editor/')) {
              return 'editor'
            }
          },
        },
      },
    },

    // Optimize dependency pre-bundling
    optimizeDeps: {
      include: [
        'react',
        'react-dom',
        'react-router-dom',
        'zustand',
        'immer',
        'axios',
        'clsx',
        'tailwind-merge',
        'class-variance-authority',
        'lucide-react',
        'framer-motion',
        'date-fns',
        'uuid',
      ],
      // Exclude heavy deps that should be loaded on demand
      exclude: [
        'monaco-editor',
        '@monaco-editor/react',
        'xterm',
      ],
    },

    // CSS optimization
    css: {
      devSourcemap: true,
      modules: {
        localsConvention: 'camelCase',
      },
    },

    // Worker configuration for Monaco
    worker: {
      format: 'es',
    },

    // Enable experimental features for better tree-shaking
    esbuild: {
      // Remove pure annotations for better dead code elimination
      legalComments: 'none',
      treeShaking: true,
    },
  }
})
