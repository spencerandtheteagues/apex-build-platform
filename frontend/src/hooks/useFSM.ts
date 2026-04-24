// FSM selectors — thin hooks that select from the zustand fsmSlice
import { useStore } from '@/hooks/useStore'
import type { FSMBuildState } from '@/store/fsmSlice'

export const useFSMBuildState = (buildID: string): FSMBuildState | undefined =>
  useStore((state) => state.fsmStates.get(buildID))

export const useFSMActiveBuilds = (): string[] =>
  useStore((state) => state.fsmActiveBuilds)

export const useFSMLoading = (): boolean =>
  useStore((state) => state.fsmLoading)

export const useFSMError = (): string | null =>
  useStore((state) => state.fsmError)
