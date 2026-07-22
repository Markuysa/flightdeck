import { useCallback, useEffect, useState } from 'react'
import { createProject, deleteProject, listProjects } from '../lib/api'
import type { CreateProjectRequest, ProjectSummary } from '../lib/types'

export interface UseProjectsResult {
  projects: ProjectSummary[]
  /** True only until the first list request settles. */
  loading: boolean
  /** Message from a failed list request; cleared on the next successful one. */
  error: string | null
  refresh: () => Promise<void>
  /** Registers a project, then refreshes the list. Rejects (without
   * refreshing) on a validation/conflict error — callers show that inline. */
  register: (body: CreateProjectRequest) => Promise<void>
  /** Unregisters a project, then refreshes the list. */
  remove: (id: string) => Promise<void>
}

/** Loads the Fleet's project list once and exposes register/remove actions
 * that keep it in sync (GET /api/projects, US-1/US-7). Shared so other
 * screens that need the registered-project list can reuse it. */
export function useProjects(): UseProjectsResult {
  const [projects, setProjects] = useState<ProjectSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    try {
      const next = await listProjects()
      setProjects(next)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load projects.')
    } finally {
      setLoading(false)
    }
  }, [])

  // Load once on mount. Deferred a microtask via .then() (rather than
  // `refresh()` called bare) so the eslint-plugin-react-hooks
  // set-state-in-effect rule sees this as the async-fetch pattern it is,
  // not a synchronous setState-during-render smell — refresh() itself
  // already awaits before touching state.
  useEffect(() => {
    Promise.resolve()
      .then(() => refresh())
      .catch(() => {
        // refresh() already turns a failure into `error` state.
      })
  }, [refresh])

  const register = useCallback(
    async (body: CreateProjectRequest) => {
      await createProject(body)
      await refresh()
    },
    [refresh],
  )

  const remove = useCallback(
    async (id: string) => {
      await deleteProject(id)
      await refresh()
    },
    [refresh],
  )

  return { projects, loading, error, refresh, register, remove }
}
