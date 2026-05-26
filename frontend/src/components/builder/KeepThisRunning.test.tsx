/** @vitest-environment jsdom */
import React from 'react'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { KeepThisRunning } from './KeepThisRunning'
import * as apiModule from '@/services/api'

const getAlwaysOnStatusSpy = vi.spyOn(apiModule.apiService, 'getAlwaysOnStatus')
const setAlwaysOnSpy = vi.spyOn(apiModule.apiService, 'setAlwaysOn')

const paidProps = () => ({
  projectId: 42,
  deploymentId: 'dep-123',
  isPaid: true,
  buildCompleted: true,
  className: undefined as string | undefined,
})

const freeProps = () => ({
  projectId: 42,
  deploymentId: 'dep-123',
  isPaid: false,
  buildCompleted: true,
  className: undefined as string | undefined,
})

const defaultStatus = () => ({
  always_on: false,
  always_on_enabled: null,
  last_keep_alive: null,
  keep_alive_interval: 60,
  sleep_after_minutes: 30,
  restart_count: 0,
  max_restarts: 5,
  container_status: 'stopped' as const,
  uptime_seconds: 0,
})

beforeEach(() => {
  getAlwaysOnStatusSpy.mockReset()
  setAlwaysOnSpy.mockReset()
  getAlwaysOnStatusSpy.mockResolvedValue(defaultStatus())
  setAlwaysOnSpy.mockResolvedValue({ success: true, always_on: true, message: 'enabled' })
})

describe('KeepThisRunning — free user', () => {
  it('renders upgrade prompt for free users without calling always-on status', async () => {
    render(<KeepThisRunning {...freeProps()} />)
    expect(screen.getByText(/Keep this app running/i)).toBeTruthy()
    expect(screen.getByRole('button', { name: /Upgrade to enable/i })).toBeTruthy()
    await new Promise((resolve) => setTimeout(resolve, 0))
    expect(getAlwaysOnStatusSpy).not.toHaveBeenCalled()
  })
})

describe('KeepThisRunning — paid user without deploymentId', () => {
  it('shows deploy-first prompt and no toggle', () => {
    render(<KeepThisRunning {...paidProps()} deploymentId={null} />)
    expect(screen.getByText(/Deploy or publish first to enable always-on/i)).toBeTruthy()
    expect(screen.queryByRole('button', { name: /disable always-on|enable always-on/i })).toBeNull()
    expect(getAlwaysOnStatusSpy).not.toHaveBeenCalled()
  })
})

describe('KeepThisRunning — paid user with deploymentId', () => {
  it('loads status via API and renders running pill when healthy', async () => {
    getAlwaysOnStatusSpy.mockResolvedValueOnce({
      always_on: true,
      always_on_enabled: '2026-05-26T00:00:00Z',
      last_keep_alive: '2026-05-26T01:00:00Z',
      keep_alive_interval: 60,
      sleep_after_minutes: 30,
      restart_count: 0,
      max_restarts: 5,
      container_status: 'healthy',
      uptime_seconds: 3661,
    })

    render(<KeepThisRunning {...paidProps()} />)
    await waitFor(() => {
      expect(screen.getByText('Running')).toBeTruthy()
    })
    expect(getAlwaysOnStatusSpy).toHaveBeenCalledWith(42, 'dep-123')
  })

  it('renders warm pill when container is starting', async () => {
    getAlwaysOnStatusSpy.mockResolvedValueOnce({
      always_on: true,
      always_on_enabled: '2026-05-26T00:00:00Z',
      last_keep_alive: '2026-05-26T01:00:00Z',
      keep_alive_interval: 60,
      sleep_after_minutes: 30,
      restart_count: 1,
      max_restarts: 5,
      container_status: 'starting',
      uptime_seconds: 120,
    })

    render(<KeepThisRunning {...paidProps()} />)
    await waitFor(() => {
      expect(screen.getByText('Warm')).toBeTruthy()
    })
  })

  it('renders cold pill when always-on is false', async () => {
    getAlwaysOnStatusSpy.mockResolvedValueOnce({
      always_on: false,
      always_on_enabled: null,
      last_keep_alive: null,
      keep_alive_interval: 60,
      sleep_after_minutes: 30,
      restart_count: 0,
      max_restarts: 5,
      container_status: 'stopped',
      uptime_seconds: 0,
    })

    render(<KeepThisRunning {...paidProps()} />)
    await waitFor(() => {
      expect(screen.getByText('Cold')).toBeTruthy()
    })
  })

  it('toggles always-on via API when switch clicked', async () => {
    getAlwaysOnStatusSpy.mockResolvedValueOnce({
      always_on: false,
      always_on_enabled: null,
      last_keep_alive: null,
      keep_alive_interval: 60,
      sleep_after_minutes: 30,
      restart_count: 0,
      max_restarts: 5,
      container_status: 'stopped',
      uptime_seconds: 0,
    })
    setAlwaysOnSpy.mockResolvedValueOnce({ success: true, always_on: true, message: 'enabled' })
    getAlwaysOnStatusSpy.mockResolvedValueOnce({
      always_on: true,
      always_on_enabled: '2026-05-26T00:00:00Z',
      last_keep_alive: '2026-05-26T01:00:00Z',
      keep_alive_interval: 60,
      sleep_after_minutes: 30,
      restart_count: 0,
      max_restarts: 5,
      container_status: 'healthy',
      uptime_seconds: 60,
    })

    render(<KeepThisRunning {...paidProps()} />)

    const toggle = await screen.findByRole('button', { name: /Enable always-on/i })
    fireEvent.click(toggle)

    await waitFor(() => {
      expect(setAlwaysOnSpy).toHaveBeenCalledWith(42, 'dep-123', true, 60)
    })
  })

  it('shows error on API failure', async () => {
    getAlwaysOnStatusSpy.mockRejectedValueOnce({ response: { data: { error: 'Network error' } } })

    render(<KeepThisRunning {...paidProps()} />)
    await waitFor(() => {
      expect(screen.getByText(/Network error/i)).toBeTruthy()
    })
  })

  it('opens and closes details breakdown', async () => {
    getAlwaysOnStatusSpy.mockResolvedValueOnce({
      always_on: true,
      always_on_enabled: '2026-05-26T00:00:00Z',
      last_keep_alive: '2026-05-26T01:00:00Z',
      keep_alive_interval: 60,
      sleep_after_minutes: 30,
      restart_count: 2,
      max_restarts: 5,
      container_status: 'healthy',
      uptime_seconds: 7200,
    })

    render(<KeepThisRunning {...paidProps()} />)
    await waitFor(() => screen.getByText('Running'))

    const detailsBtn = screen.getByRole('button', { name: /details/i })
    fireEvent.click(detailsBtn)
    expect(screen.getByText(/Always-On Status/i)).toBeTruthy()
    expect(screen.getByText(/2h 0m/i)).toBeTruthy()
    expect(screen.getByText(/Restarts/i)).toBeTruthy()

    fireEvent.click(detailsBtn)
    await waitFor(() => {
      expect(screen.queryByText(/Always-On Status/i)).toBeNull()
    })
  })

  it('gracefully handles 404 from API as not available', async () => {
    getAlwaysOnStatusSpy.mockRejectedValueOnce({ response: { status: 404 } })

    render(<KeepThisRunning {...paidProps()} />)
    await waitFor(() => {
      expect(screen.queryByText(/Failed to load status/i)).toBeNull()
    })
    expect(getAlwaysOnStatusSpy).toHaveBeenCalledTimes(1)
  })
})
