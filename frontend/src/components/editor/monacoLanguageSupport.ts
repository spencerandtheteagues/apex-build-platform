const loadedLanguages = new Set<string>()

const languageLoaders: Record<string, () => Promise<unknown>> = {
  javascript: () => import('monaco-editor/esm/vs/basic-languages/javascript/javascript.contribution'),
  typescript: () => import('monaco-editor/esm/vs/basic-languages/typescript/typescript.contribution'),
  python: () => import('monaco-editor/esm/vs/basic-languages/python/python.contribution'),
  go: () => import('monaco-editor/esm/vs/basic-languages/go/go.contribution'),
  rust: () => import('monaco-editor/esm/vs/basic-languages/rust/rust.contribution'),
  java: () => import('monaco-editor/esm/vs/basic-languages/java/java.contribution'),
  cpp: () => import('monaco-editor/esm/vs/basic-languages/cpp/cpp.contribution'),
  html: () => import('monaco-editor/esm/vs/basic-languages/html/html.contribution'),
  css: () => import('monaco-editor/esm/vs/basic-languages/css/css.contribution'),
  scss: () => import('monaco-editor/esm/vs/basic-languages/scss/scss.contribution'),
  markdown: () => import('monaco-editor/esm/vs/basic-languages/markdown/markdown.contribution'),
  yaml: () => import('monaco-editor/esm/vs/basic-languages/yaml/yaml.contribution'),
  xml: () => import('monaco-editor/esm/vs/basic-languages/xml/xml.contribution'),
  sql: () => import('monaco-editor/esm/vs/basic-languages/sql/sql.contribution'),
  json: () => import('monaco-editor/esm/vs/language/json/monaco.contribution'),
}

export async function ensureMonacoLanguageSupport(language: string): Promise<void> {
  const normalized = language.trim().toLowerCase()
  if (!normalized || loadedLanguages.has(normalized)) return

  const load = languageLoaders[normalized]
  if (!load) return

  await load()
  loadedLanguages.add(normalized)
}
