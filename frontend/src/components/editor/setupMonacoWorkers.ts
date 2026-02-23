import editorWorker from 'monaco-editor/esm/vs/editor/editor.worker?worker'
import jsonWorker from 'monaco-editor/esm/vs/language/json/json.worker?worker'
import tsWorker from 'monaco-editor/esm/vs/language/typescript/ts.worker?worker'

let workersInitialized = false

export function ensureMonacoWorkersInitialized(): Promise<void> {
  if (workersInitialized || typeof window === 'undefined') return Promise.resolve()

  window.MonacoEnvironment = {
    getWorker(_, label) {
      if (label === 'json') return new jsonWorker()
      if (label === 'typescript' || label === 'javascript') return new tsWorker()
      return new editorWorker()
    },
  }

  workersInitialized = true
  return Promise.resolve()
}
