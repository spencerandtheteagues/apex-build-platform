// FSM State Slice — WebSocket FSM event consumer for Zustand store
// Maps backend FSM bridge events (build:fsm:*) into reactive frontend state

import { enableMapSet } from 'immer'
import { FSMState, FSMEvent, FSMWSMessageType } from '@/types'

// Required so Immer can draft the Map used in this slice's state.
enableMapSet()

export interface FSMBuildState {
  buildID: string
  currentState: FSMState
  previousState: FSMState | null
  currentEvent: FSMEvent | null
  progress: number
  elapsedMs: number
  retryCount: number
  stepID: string
  checkpointID: string | null
  errorMessage: string | null
  metadata: string
  timestamp: string
}

export interface FSMStateSlice {
  fsmStates: Map<string, FSMBuildState>
  fsmActiveBuilds: string[]
  fsmLoading: boolean
  fsmError: string | null
}

export interface FSMActions {
  // Core FSM event handlers (wired from WebSocket service)
  handleFSMStarted: (buildID: string, data: any) => void
  handleFSMInitialized: (buildID: string, data: any) => void
  handleFSMPlanReady: (buildID: string, data: any) => void
  handleFSMStepComplete: (buildID: string, data: any) => void
  handleFSMAllStepsComplete: (buildID: string, data: any) => void
  handleFSMValidationPass: (buildID: string, data: any) => void
  handleFSMValidationFail: (buildID: string, data: any) => void
  handleFSMRetryExhausted: (buildID: string, data: any) => void
  handleFSMRollbackComplete: (buildID: string, data: any) => void
  handleFSMRollbackFailed: (buildID: string, data: any) => void
  handleFSMPaused: (buildID: string, data: any) => void
  handleFSMResumed: (buildID: string, data: any) => void
  handleFSMCancelled: (buildID: string, data: any) => void
  handleFSMFatalError: (buildID: string, data: any) => void
  handleFSMCheckpointCreated: (buildID: string, data: any) => void
  handleFSMRollback: (buildID: string, data: any) => void

  // Generic handler for any FSM event
  handleFSMEvent: (msgType: FSMWSMessageType, buildID: string, data: any) => void

  // State management
  getFSMState: (buildID: string) => FSMBuildState | undefined
  clearFSMState: (buildID: string) => void
  clearAllFSMStates: () => void
}

export const createFSMSlice = (set: any, get: any): FSMStateSlice & FSMActions => ({
  // State
  fsmStates: new Map(),
  fsmActiveBuilds: [],
  fsmLoading: false,
  fsmError: null,

  // Helper to update or create FSM state for a build
  handleFSMEvent: (msgType: FSMWSMessageType, buildID: string, data: any) => {
    set((state: any) => {
      const existing = state.fsmStates.get(buildID)
      const fsmState: FSMBuildState = {
        buildID,
        currentState: data.to_state || data.fsm_state || existing?.currentState || 'idle',
        previousState: data.from_state || existing?.currentState || null,
        currentEvent: data.event || null,
        progress: data.progress ?? existing?.progress ?? 0,
        elapsedMs: data.elapsed_ms ?? existing?.elapsedMs ?? 0,
        retryCount: data.retry_count ?? existing?.retryCount ?? 0,
        stepID: data.step_id || existing?.stepID || '',
        checkpointID: data.checkpoint_id || existing?.checkpointID || null,
        errorMessage: data.error || existing?.errorMessage || null,
        metadata: data.metadata || existing?.metadata || '',
        timestamp: data.timestamp || new Date().toISOString(),
      }

      state.fsmStates.set(buildID, fsmState)

      // Track active builds
      if (!state.fsmActiveBuilds.includes(buildID)) {
        state.fsmActiveBuilds.push(buildID)
      }

      // Auto-clear completed/error builds from active list after 5 min
      if (['completed', 'failed', 'cancelled'].includes(fsmState.currentState)) {
        setTimeout(() => {
          get().clearFSMState(buildID)
        }, 300000)
      }
    })
  },

  // Specific event handlers — all delegate to generic handler
  handleFSMStarted: (buildID, data) => get().handleFSMEvent('build:fsm:started', buildID, data),
  handleFSMInitialized: (buildID, data) => get().handleFSMEvent('build:fsm:initialized', buildID, data),
  handleFSMPlanReady: (buildID, data) => get().handleFSMEvent('build:fsm:plan_ready', buildID, data),
  handleFSMStepComplete: (buildID, data) => get().handleFSMEvent('build:fsm:step_complete', buildID, data),
  handleFSMAllStepsComplete: (buildID, data) => get().handleFSMEvent('build:fsm:all_steps_complete', buildID, data),
  handleFSMValidationPass: (buildID, data) => get().handleFSMEvent('build:fsm:validation_pass', buildID, data),
  handleFSMValidationFail: (buildID, data) => get().handleFSMEvent('build:fsm:validation_fail', buildID, data),
  handleFSMRetryExhausted: (buildID, data) => get().handleFSMEvent('build:fsm:retry_exhausted', buildID, data),
  handleFSMRollbackComplete: (buildID, data) => get().handleFSMEvent('build:fsm:rollback_complete', buildID, data),
  handleFSMRollbackFailed: (buildID, data) => get().handleFSMEvent('build:fsm:rollback_failed', buildID, data),
  handleFSMPaused: (buildID, data) => get().handleFSMEvent('build:fsm:paused', buildID, data),
  handleFSMResumed: (buildID, data) => get().handleFSMEvent('build:fsm:resumed', buildID, data),
  handleFSMCancelled: (buildID, data) => get().handleFSMEvent('build:fsm:cancelled', buildID, data),
  handleFSMFatalError: (buildID, data) => get().handleFSMEvent('build:fsm:fatal_error', buildID, data),
  handleFSMCheckpointCreated: (buildID, data) => get().handleFSMEvent('build:fsm:checkpoint_created', buildID, data),
  handleFSMRollback: (buildID, data) => get().handleFSMEvent('build:fsm:rollback', buildID, data),

  // Getters
  getFSMState: (buildID: string) => {
    return get().fsmStates.get(buildID)
  },

  clearFSMState: (buildID: string) => {
    set((state: any) => {
      state.fsmStates.delete(buildID)
      state.fsmActiveBuilds = state.fsmActiveBuilds.filter((id: string) => id !== buildID)
    })
  },

  clearAllFSMStates: () => {
    set((state: any) => {
      state.fsmStates.clear()
      state.fsmActiveBuilds = []
    })
  },
})
