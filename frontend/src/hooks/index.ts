// APEX.BUILD Hooks
// Export all custom hooks

// Store hooks
export { useStore } from './useStore'
export {
  useAuth,
  useProjectsState,
  useFilesState,
  useEditor,
  useCollaboration as useStoreCollaboration,
  useAI,
  useUI,
  useUser,
  useIsAuthenticated,
  useCurrentProject,
  useProjects,
  useFiles,
  useOpenFiles,
  useActiveFileId,
  useActiveFile,
  useCursorPosition,
  useCurrentTheme,
  useSidebarOpen,
  useTerminalOpen,
  useAiPanelOpen,
  useNotifications,
  useTerminals,
  useActiveTerminalId,
  useIsConnected,
  useCollaborationUsers,
  useConnectedUsers,
} from './useStore'

// Collaboration hook
export { useCollaboration, default as useCollaborationHook } from './useCollaboration'
export type { RemoteCursor, UseCollaborationOptions } from './useCollaboration'

// Mobile detection hooks
export {
  useBreakpoint,
  useMobile,
  useIsMobile,
  useTouch,
  usePinchZoom,
  useKeyboardHeight,
  useSafeArea,
  useOrientationChange,
  useVirtualKeyboard,
  BREAKPOINTS,
} from './useMobile'
export type { Breakpoint } from './useMobile'

// Pane manager hook
export { usePaneManager } from './usePaneManager'
export type {
  PaneFile,
  EditorPane,
  PaneLayout,
  UsePaneManagerReturn,
} from './usePaneManager'

// Code review hook
export { useCodeReview } from './useCodeReview'

// Project search hook
export { useProjectSearch } from './useProjectSearch'

// Git integration hook
export { useGitIntegration } from './useGitIntegration'
