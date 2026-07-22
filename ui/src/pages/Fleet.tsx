import { Plus, Rocket, ServerCrash } from 'lucide-react'
import { useState } from 'react'
import { Button } from '../components/Button'
import { EmptyState } from '../components/EmptyState'
import { ProjectCard } from '../components/ProjectCard'
import { RegisterProjectDialog } from '../components/RegisterProjectDialog'
import { useProjects } from '../hooks/useProjects'

/** The Fleet screen (US-1, docs/DESIGN.md §4.1) — the landing view: every
 * registered project as a card, plus register/remove. */
export function Fleet() {
  const { projects, loading, error, refresh, register, remove } = useProjects()
  const [dialogOpen, setDialogOpen] = useState(false)

  return (
    <div className="flex flex-col gap-6 p-6">
      <header className="flex items-center justify-between gap-4">
        <div>
          <h1 className="font-display text-xl font-semibold text-text">Fleet</h1>
          <p className="text-sm text-text-mut">Every registered project, at a glance.</p>
        </div>
        <Button onClick={() => setDialogOpen(true)}>
          <Plus className="h-4 w-4" />
          Register project
        </Button>
      </header>

      {error ? (
        <EmptyState
          icon={ServerCrash}
          title="Couldn't load projects"
          description={error}
          action={
            <Button variant="ghost" onClick={refresh}>
              Try again
            </Button>
          }
        />
      ) : loading ? (
        <p className="py-16 text-center text-sm text-text-mut" aria-live="polite">
          Loading projects…
        </p>
      ) : projects.length === 0 ? (
        <EmptyState
          icon={Rocket}
          title="No projects registered yet"
          description="Register a project's local repository to start driving its ticket queue from here."
        />
      ) : (
        <div className="grid grid-cols-[repeat(auto-fill,minmax(260px,1fr))] gap-4">
          {projects.map((project) => (
            <ProjectCard key={project.id} project={project} onRemove={() => remove(project.id)} />
          ))}
        </div>
      )}

      {dialogOpen && (
        <RegisterProjectDialog
          onClose={() => setDialogOpen(false)}
          onRegister={async (body) => {
            await register(body)
            setDialogOpen(false)
          }}
        />
      )}
    </div>
  )
}
