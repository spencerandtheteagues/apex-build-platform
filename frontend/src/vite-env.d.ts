/// <reference types="vite/client" />

// Environment variables
interface ImportMetaEnv {
  readonly VITE_API_URL: string
  readonly VITE_WS_URL: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}

// Monaco Editor Worker Module Declarations
declare module 'monaco-editor/esm/vs/editor/editor.worker?worker' {
  const WorkerFactory: new () => Worker
  export default WorkerFactory
}

declare module 'monaco-editor/esm/vs/language/json/json.worker?worker' {
  const WorkerFactory: new () => Worker
  export default WorkerFactory
}

declare module 'monaco-editor/esm/vs/language/css/css.worker?worker' {
  const WorkerFactory: new () => Worker
  export default WorkerFactory
}

declare module 'monaco-editor/esm/vs/language/html/html.worker?worker' {
  const WorkerFactory: new () => Worker
  export default WorkerFactory
}

declare module 'monaco-editor/esm/vs/language/typescript/ts.worker?worker' {
  const WorkerFactory: new () => Worker
  export default WorkerFactory
}

// Monaco environment declaration
interface MonacoEnvironment {
  getWorker(moduleId: string, label: string): Worker
}

declare global {
  interface Window {
    MonacoEnvironment: MonacoEnvironment
  }
}
