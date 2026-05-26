/* @vitest-environment jsdom */

import React from 'react'
import { render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

// Stub the heavy Monaco editor so the lazy import resolves to something harmless
// in jsdom (no canvas/workers). forwardRef because SplitPaneEditor passes a ref.
vi.mock('@/components/editor/MonacoEditor', async () => {
  const ReactMod = await import('react')
  return {
    MonacoEditor: ReactMod.forwardRef<unknown, Record<string, unknown>>(
      (_props, _ref) => ReactMod.createElement('div', { 'data-testid': 'monaco-stub' }, 'monaco'),
    ),
  }
})

import { SplitPaneEditor } from './SplitPaneEditor'
import type { PaneLayout } from '@/hooks/usePaneManager'
import type { File as ProjectFile } from '@/types'

function makeLayout(): PaneLayout {
  const file = { id: 1, name: 'index.ts' } as unknown as ProjectFile
  return {
    type: 'single',
    panes: [
      {
        id: 'pane-1',
        activeFileId: 1,
        files: [{ file, content: 'console.log(1)', hasUnsavedChanges: false }],
      },
    ],
  }
}

const noop = () => {}
const asyncNoop = async () => undefined

function renderEditor() {
  return render(
    <SplitPaneEditor
      layout={makeLayout()}
      activePaneId="pane-1"
      canSplit
      onFocusPane={noop}
      onClosePane={noop}
      onFileSelect={noop}
      onFileClose={noop}
      onFileChange={noop}
      onFileSave={noop}
      onAIRequest={asyncNoop}
      onSplitHorizontal={noop}
      onSplitVertical={noop}
    />,
  )
}

describe('SplitPaneEditor lazy editor loading', () => {
  it('shows the accessible editor skeleton while Monaco is suspended', () => {
    renderEditor()
    // On the first render pass the lazy Monaco chunk has not resolved, so the
    // Suspense fallback (our accessible skeleton) must be present.
    const status = screen.getByRole('status')
    expect(status.getAttribute('aria-busy')).toBe('true')
    expect(screen.getByText('Loading editor...')).toBeTruthy()
  })

  it('swaps the fallback for the editor once the lazy chunk resolves', async () => {
    renderEditor()
    await waitFor(() => expect(screen.getByTestId('monaco-stub')).toBeTruthy())
    // Fallback is gone once the editor mounts.
    expect(screen.queryByRole('status')).toBeNull()
  })
})
