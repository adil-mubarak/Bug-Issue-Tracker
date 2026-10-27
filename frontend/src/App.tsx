import { FormEvent, useEffect, useRef, useState } from 'react'
import { Issue, Project, ProjectBoard } from './ProjectBoard'

const api = async (path: string, options: RequestInit = {}) => {
  const token = localStorage.getItem('token')
  const response = await fetch(`/api${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...options.headers,
    },
  })
  if (!response.ok) throw new Error((await response.text()) || 'Request failed')
  if (response.status === 204) return null
  return response.json()
}

type AuthMode = 'login' | 'register' | 'forgot'

export default function App() {
  const [token, setToken] = useState(localStorage.getItem('token') || '')
  const [mode, setMode] = useState<AuthMode>('login')
  const [showPassword, setShowPassword] = useState(false)
  const [projects, setProjects] = useState<Project[]>([])
  const [selected, setSelected] = useState<number | null>(null)
  const [issues, setIssues] = useState<Issue[]>([])
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [query, setQuery] = useState(() => new URLSearchParams(location.search).get('q') || '')
  const [statusFilter, setStatusFilter] = useState(() => new URLSearchParams(location.search).get('status') || '')
  const [priorityFilter, setPriorityFilter] = useState(() => new URLSearchParams(location.search).get('priority') || '')
  const [selectedIssue, setSelectedIssue] = useState<Issue | null>(null)
  const [loadingIssues, setLoadingIssues] = useState(false)
  const [newIssueStatus, setNewIssueStatus] = useState('todo')
  const [createIssueOpen, setCreateIssueOpen] = useState(false)
  const [screen, setScreen] = useState<'board' | 'projects' | 'components'>('board')
  const projectRequestId = useRef(0)
  const issueRequestId = useRef(0)
  const selectedProjectRef = useRef<number | null>(selected)
  selectedProjectRef.current = selected

  const selectedProject = projects.find(project => project.id === selected)

  async function loadProjects() {
    const requestId = ++projectRequestId.current
    let data: Project[]
    try {
      data = await api('/projects') as Project[]
    } catch (cause) {
      if (requestId !== projectRequestId.current) return
      throw cause
    }
    if (requestId !== projectRequestId.current) return
    setProjects(data)
    if (!selected && data.length) {
      selectedProjectRef.current = data[0].id
      setSelected(data[0].id)
    }
  }

  async function loadIssues(projectId: number) {
    const requestId = ++issueRequestId.current
    setLoadingIssues(true)
    try {
      const params = new URLSearchParams({ project_id: String(projectId) })
      if (query) params.set('q', query)
      if (statusFilter) params.set('status', statusFilter)
      if (priorityFilter) params.set('priority', priorityFilter)
      const result = await api(`/issues?${params}`) as Issue[]
      if (requestId === issueRequestId.current) {
        setIssues(result)
        setError('')
      }
    } catch (cause) {
      if (requestId === issueRequestId.current) {
        setError(cause instanceof Error ? cause.message : String(cause))
      }
    } finally {
      if (requestId === issueRequestId.current) setLoadingIssues(false)
    }
  }

  useEffect(() => {
    if (token) loadProjects().catch(cause => setError(cause.message))
  }, [token])

  useEffect(() => {
    const params = new URLSearchParams(location.search)
    query ? params.set('q', query) : params.delete('q')
    statusFilter ? params.set('status', statusFilter) : params.delete('status')
    priorityFilter ? params.set('priority', priorityFilter) : params.delete('priority')
    const search = params.toString()
    history.replaceState(null, '', `${location.pathname}${search ? `?${search}` : ''}`)
  }, [query, statusFilter, priorityFilter])

  useEffect(() => {
    if (token && selected) void loadIssues(selected)
  }, [token, selected, query, statusFilter, priorityFilter])

  async function authenticate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setBusy(true)
    setError('')
    const form = new FormData(event.currentTarget)
    try {
      const data = await api(`/auth/${mode}`, {
        method: 'POST',
        body: JSON.stringify({ email: form.get('email'), password: form.get('password') }),
      })
      if (mode === 'register') {
        setMode('login')
        return
      }
      localStorage.setItem('email', String(form.get('email') || ''))
      localStorage.setItem('token', data.access_token)
      setToken(data.access_token)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Could not sign in')
    } finally {
      setBusy(false)
    }
  }

  async function createProject(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const target = event.currentTarget
    const form = new FormData(target)
    try {
      await api('/projects', {
        method: 'POST',
        body: JSON.stringify({ name: form.get('name'), description: form.get('description') || '' }),
      })
      target.reset()
      await loadProjects()
    } catch (cause) {
      setError(String(cause))
    }
  }

  async function createIssue(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const target = event.currentTarget
    const form = new FormData(target)
    const projectId = Number(form.get('project_id')) || selected
    if (!projectId) return
    const labelNames = String(form.get('labels') || '').split(',').map(name => name.trim()).filter(Boolean)
    try {
      const created = await api('/issues', {
        method: 'POST',
        body: JSON.stringify({
          project_id: projectId,
          title: form.get('title'),
          description: form.get('description') || '',
          priority: form.get('priority'),
          status: form.get('status'),
          assignee_id: String(form.get('assignee_id') || '').trim() ? Number(form.get('assignee_id')) : null,
        }),
      }) as Issue
      const labelResults = await Promise.allSettled(labelNames.map(name => api(`/issues/${created.id}/labels`, {
        method: 'POST',
        body: JSON.stringify({ name }),
      })))
      target.reset()
      setNewIssueStatus('todo')
      setCreateIssueOpen(false)
      if (projectId === selectedProjectRef.current) await loadIssues(projectId)
      else selectProject(projectId)
      if (labelResults.some(result => result.status === 'rejected')) setError('Issue created, but one or more labels could not be added.')
    } catch (cause) {
      setError(String(cause))
    }
  }

  async function setStatus(issue: Issue, status: string) {
    try {
      await api(`/issues/${issue.id}`, {
        method: 'PATCH',
        body: JSON.stringify({
          title: issue.title,
          description: issue.description,
          priority: issue.priority,
          status,
          assignee_id: issue.assignee_id,
        }),
      })
      const currentProjectId = selectedProjectRef.current
      if (currentProjectId) await loadIssues(currentProjectId)
    } catch (cause) {
      setError(String(cause))
    }
  }

  function addProjectFocus() {
    setScreen('board')
    window.setTimeout(() => document.getElementById('project-name')?.focus(), 0)
  }

  function selectProject(projectId: number) {
    selectedProjectRef.current = projectId
    setSelected(projectId)
  }

  function addIssueFocus(status = 'todo') {
    setError('')
    setNewIssueStatus(status)
    setCreateIssueOpen(true)
  }

  function logout() {
    projectRequestId.current++
    issueRequestId.current++
    localStorage.removeItem('token')
    localStorage.removeItem('email')
    selectedProjectRef.current = null
    setSelected(null)
    setProjects([])
    setIssues([])
    setSelectedIssue(null)
    setCreateIssueOpen(false)
    setScreen('board')
    setQuery('')
    setStatusFilter('')
    setPriorityFilter('')
    setError('')
    setToken('')
  }

  if (!token) {
    return (
      <main className="auth-shell">
        <section className="auth-promo">
          <a className="auth-brand" href="#home"><span className="brand-icon material-symbols-outlined" aria-hidden="true">view_kanban</span><span>Trackline</span></a>
          <div className="auth-promo-copy">
            <p className="eyebrow">Engineering Workspace</p>
            <h1>Issue tracking built for high-velocity engineering teams.</h1>
            <p>Keep project work and issue details in one focused workspace, with clear status, priority, and ownership at a glance.</p>
          </div>
          <div className="auth-preview-card">
            <div className="auth-preview-heading"><span>Sprint 24 Board</span><span className="preview-live"><i />32 active issues</span></div>
            <div className="auth-preview-issue"><span className="issue-id">TRK-122</span><span className="priority-badge high">High</span><strong>Optimize canvas rasterization for 60fps graph rendering</strong><small>Refactor virtualized DOM nodes within board column viewport container.</small></div>
            <div className="auth-preview-footer"><span>In progress</span><span className="preview-avatars"><i>A</i><i>M</i><i>+3</i></span></div>
          </div>
          <blockquote>“Trackline cut our sprint triage time in half. It feels as fast as a native app with zero spreadsheet overhead.”<cite>— Engineering team, Trackline customer</cite></blockquote>
          <span className="auth-promo-foot">Thoughtful work starts here.</span>
        </section>
        <section className="auth-panel">
          <div className="auth-form-wrap">
            <div className="auth-panel-heading">
              <p className="eyebrow">Your workspace</p>
              <h2>{mode === 'login' ? 'Welcome back' : mode === 'register' ? 'Create your account' : 'Forgot password?'}</h2>
              <p className="muted">{mode === 'login' ? 'Enter your credentials to access your workspaces.' : mode === 'register' ? 'Create an account to start working with your team.' : 'Password recovery is managed by your workspace administrator.'}</p>
            </div>
            {mode !== 'forgot' && <div className="auth-tabs" role="tablist" aria-label="Authentication mode">
              <button type="button" role="tab" aria-selected={mode === 'login'} className={mode === 'login' ? 'selected' : ''} onClick={() => { setMode('login'); setError('') }}>Sign in</button>
              <button type="button" role="tab" aria-selected={mode === 'register'} className={mode === 'register' ? 'selected' : ''} onClick={() => { setMode('register'); setError('') }}>Sign up</button>
            </div>}
            {mode === 'forgot' ? (
              <>
                <p className="recovery-note">Please contact your workspace administrator to reset your password.</p>
                <button className="primary-button auth-submit" onClick={() => { setMode('login'); setError('') }}>Back to sign in</button>
              </>
            ) : (
              <form onSubmit={authenticate}>
                <label>Email address<input name="email" type="email" placeholder="name@company.com" required /></label>
                <label className="password-label">
                  Password
                  {mode === 'login' && <button type="button" className="forgot-link" onClick={() => { setMode('forgot'); setError('') }}>Forgot password?</button>}
                  <span className="password-field">
                    <input name="password" type={showPassword ? 'text' : 'password'} minLength={8} placeholder="At least 8 characters" required />
                    <button type="button" className="password-toggle" aria-label={showPassword ? 'Hide password' : 'Show password'} aria-pressed={showPassword} onClick={() => setShowPassword(!showPassword)}>
                      <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M2 12s3.6-7 10-7 10 7 10 7-3.6 7-10 7S2 12 2 12Z" /><circle cx="12" cy="12" r="3" /></svg>
                    </button>
                  </span>
                </label>
                <button className="primary-button auth-submit" disabled={busy}>{busy ? 'Please wait…' : mode === 'login' ? 'Sign in' : 'Create account'}<span className="material-symbols-outlined" aria-hidden="true">arrow_forward</span></button>
              </form>
            )}
            <p className="switch">
              {mode === 'register' ? 'Already have an account?' : mode === 'forgot' ? 'Remember your password?' : 'New to Trackline?'}{' '}
              <button onClick={() => { setMode(mode === 'register' || mode === 'forgot' ? 'login' : 'register'); setError('') }}>
                {mode === 'register' || mode === 'forgot' ? 'Sign in' : 'Create account'}
              </button>
            </p>
            {error && <p className="error" role="alert">{error}</p>}
          </div>
        </section>
      </main>
    )
  }

  return (
    <>
      <ProjectBoard
        projects={projects}
        selectedProject={selectedProject}
        selectedProjectId={selected}
        issues={issues}
        loading={loadingIssues}
        error={error}
        query={query}
        statusFilter={statusFilter}
        priorityFilter={priorityFilter}
        email={localStorage.getItem('email') || ''}
        onSelectProject={selectProject}
        screen={screen}
        onShowProjects={() => setScreen('projects')}
        onShowComponents={() => setScreen('components')}
        onOpenBoard={() => setScreen('board')}
        onAddProject={addProjectFocus}
        onAddIssue={addIssueFocus}
        onLogout={logout}
        onDismissError={() => setError('')}
        onQueryChange={setQuery}
        onStatusFilterChange={setStatusFilter}
        onPriorityFilterChange={setPriorityFilter}
        onOpenIssue={setSelectedIssue}
        onSetStatus={setStatus}
      >
        <section className="creation-panels" aria-label="Create project or issue">
          {screen === 'board' && <form className="creation-panel" onSubmit={createProject}>
            <h2>New project</h2>
            <div className="creation-fields">
              <input id="project-name" name="name" placeholder="Project name" required />
              <input name="description" placeholder="Short description" />
              <button className="primary-button" type="submit">Create project</button>
            </div>
          </form>}
        </section>
      </ProjectBoard>
      {createIssueOpen && <NewIssueModal
        projects={projects}
        selectedProjectId={selected}
        initialStatus={newIssueStatus}
        error={error}
        onClose={() => setCreateIssueOpen(false)}
        onSubmit={createIssue}
      />}
      {selectedIssue && <IssueDetailDrawer issue={selectedIssue} projectId={selected ?? 0} onClose={() => setSelectedIssue(null)} onSaved={async () => {
        setSelectedIssue(null)
        if (selected) await loadIssues(selected)
      }} />}
    </>
  )
}

type IssueLabel = { id: number; name: string }
type ActivityEvent = { id: number; issue_id: number; actor_id: number; event_type: string; payload: Record<string, unknown>; occurred_at: string }

function IssueDetailDrawer({ issue, projectId, onClose, onSaved }: { issue: Issue; projectId: number; onClose: () => void; onSaved: () => Promise<void> }) {
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [labels, setLabels] = useState<IssueLabel[]>([])
  const [activity, setActivity] = useState<ActivityEvent[]>([])
  const [activityWindowFull, setActivityWindowFull] = useState(false)
  const [activeTab, setActiveTab] = useState<'details' | 'activity'>('details')
  const [labelName, setLabelName] = useState('')
  const [savingLabel, setSavingLabel] = useState(false)
  const dialogRef = useRef<HTMLElement>(null)
  const onCloseRef = useRef(onClose)
  onCloseRef.current = onClose

  useEffect(() => {
    let current = true
    api(`/issues/${issue.id}/labels`).then((result: IssueLabel[]) => {
      if (current) setLabels(result)
    }).catch(() => {
      if (current) setLabels([])
    })
    if (projectId) {
      api(`/activity?project_id=${projectId}&limit=100`).then((result: ActivityEvent[]) => {
        if (current) {
          setActivity(result.filter(item => item.issue_id === issue.id))
          setActivityWindowFull(result.length >= 100)
        }
      }).catch(() => {
        if (current) {
          setActivity([])
          setActivityWindowFull(false)
        }
      })
    }
    return () => { current = false }
  }, [issue.id, projectId])

  useEffect(() => {
    const previouslyFocused = document.activeElement as HTMLElement | null
    const dialog = dialogRef.current
    const focusable = dialog?.querySelectorAll<HTMLElement>('button:not([disabled]), input:not([disabled]), textarea:not([disabled]), select:not([disabled])')
    focusable?.[0]?.focus()

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        event.preventDefault()
        onCloseRef.current()
        return
      }
      if (event.key !== 'Tab' || !dialog) return
      const items = dialog.querySelectorAll<HTMLElement>('button:not([disabled]), input:not([disabled]), textarea:not([disabled]), select:not([disabled])')
      const first = items[0]
      const last = items[items.length - 1]
      if (!first || !last) return
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault()
        last.focus()
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault()
        first.focus()
      }
    }

    document.addEventListener('keydown', handleKeyDown)
    return () => {
      document.removeEventListener('keydown', handleKeyDown)
      previouslyFocused?.focus()
    }
  }, [])

  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    const title = String(form.get('title') || '').trim()
    if (!title) {
      setError('Issue title is required.')
      return
    }
    setBusy(true)
    setError('')
    try {
      await api(`/issues/${issue.id}`, {
        method: 'PATCH',
        body: JSON.stringify({
          title,
          description: form.get('description'),
          priority: form.get('priority'),
          status: form.get('status'),
          assignee_id: String(form.get('assignee_id') || '').trim() ? Number(form.get('assignee_id')) : null,
        }),
      })
      await onSaved()
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Could not update issue')
    } finally {
      setBusy(false)
    }
  }

  async function addLabel() {
    const name = labelName.trim()
    if (!name) return
    setSavingLabel(true)
    setError('')
    try {
      const added = await api(`/issues/${issue.id}/labels`, {
        method: 'POST',
        body: JSON.stringify({ name }),
      }) as IssueLabel
      setLabels(current => current.some(label => label.id === added.id) ? current : [...current, added].sort((a, b) => a.name.localeCompare(b.name)))
      setLabelName('')
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Could not add label')
    } finally {
      setSavingLabel(false)
    }
  }

  return (
    <div className="dialog-backdrop drawer-backdrop" role="presentation" onMouseDown={event => { if (event.target === event.currentTarget) onClose() }}>
      <section ref={dialogRef} className="issue-dialog issue-drawer" role="dialog" aria-modal="true" aria-labelledby="issue-dialog-title" tabIndex={-1}>
        <div className="drawer-topbar">
          <span className="drawer-breadcrumb">{projectId ? `Project ${projectId}` : 'Issue details'} <span>/</span> #{issue.id}</span>
          <div className="drawer-actions">
            <button type="button" className="icon-button" aria-label="Close issue details" onClick={onClose}><span className="material-symbols-outlined" aria-hidden="true">close</span></button>
          </div>
        </div>
        <div className="drawer-title-row">
          <span className="issue-id">TRK-{issue.id}</span>
          <span className={`priority-badge ${issue.priority}`}>{issue.priority}</span>
        </div>
        <h2 id="issue-dialog-title" className="drawer-title">{issue.title}</h2>
        <div className="drawer-tabs" role="tablist" aria-label="Issue details sections">
          <button type="button" role="tab" aria-selected={activeTab === 'details'} className={activeTab === 'details' ? 'selected' : ''} onClick={() => setActiveTab('details')}>Details</button>
          <button type="button" role="tab" aria-selected={activeTab === 'activity'} className={activeTab === 'activity' ? 'selected' : ''} onClick={() => setActiveTab('activity')}>Activity <span>{activity.length}</span></button>
        </div>
        {activeTab === 'details' ? (
          <div className="drawer-content">
            <form className="drawer-edit-form" onSubmit={save}>
              <label>Title<input name="title" defaultValue={issue.title} maxLength={240} required /></label>
              <label>Description<textarea name="description" defaultValue={issue.description} rows={6} placeholder="Add a description…" /></label>
              <div className="drawer-fields-row">
                <label>Status<select name="status" defaultValue={issue.status}><option value="todo">To do</option><option value="in_progress">In progress</option><option value="done">Done</option></select></label>
                <label>Priority<select name="priority" defaultValue={issue.priority}><option value="low">Low</option><option value="medium">Medium</option><option value="high">High</option><option value="critical">Critical</option></select></label>
              </div>
              <label>Assignee ID<input name="assignee_id" type="number" min="1" defaultValue={issue.assignee_id || ''} placeholder="Unassigned" /></label>
              {error && <p className="error-banner" role="alert">{error}</p>}
              <div className="dialog-actions"><button type="button" className="secondary-button" onClick={onClose}>Cancel</button><button className="primary-button" disabled={busy}>{busy ? 'Saving…' : 'Save changes'}</button></div>
            </form>
            <div className="drawer-labels">
              <div className="drawer-section-title"><h3>Labels</h3></div>
              <div className="label-list">{labels.map(label => <span className="label-chip" key={label.id}>{label.name}</span>)}{labels.length === 0 && <span className="label-empty">No labels attached</span>}</div>
              <div className="add-label-form">
                <input aria-label="New label" maxLength={64} value={labelName} onChange={event => setLabelName(event.target.value)} placeholder="Add a label" />
                <button className="secondary-button" type="button" onClick={() => void addLabel()} disabled={savingLabel || !labelName.trim()}>{savingLabel ? 'Adding…' : 'Add label'}</button>
              </div>
            </div>
          </div>
        ) : (
          <div className="drawer-content activity-content">
            {activity.length ? activity.map(item => <article className="activity-item" key={item.id}>
              <span className="activity-avatar">{item.actor_id ? `#${item.actor_id}` : '·'}</span>
              <div><strong>{activityDescription(item)}</strong><time>{new Date(item.occurred_at).toLocaleString()}</time></div>
            </article>) : <p className="activity-empty">{activityWindowFull ? 'No activity for this issue in the latest 100 project events; older activity may exist.' : 'No activity recorded for this issue yet.'}</p>}
            {activityWindowFull && <p className="activity-limit-note">Showing activity from the latest 100 project events. Older entries may be omitted.</p>}
          </div>
        )}
        <div className="drawer-footer-note">Comments and checklist items aren’t available for this issue yet.</div>
      </section>
    </div>
  )
}

function activityDescription(item: ActivityEvent) {
  if (item.event_type === 'IssueCreated') return 'Issue created'
  if (item.event_type === 'StatusChanged' && typeof item.payload?.status === 'string') return `Status changed to ${item.payload.status.replace('_', ' ')}`
  return item.event_type.replace(/([a-z])([A-Z])/g, '$1 $2') || 'Issue updated'
}

function NewIssueModal({ projects, selectedProjectId, initialStatus, error, onClose, onSubmit }: {
  projects: Project[]
  selectedProjectId: number | null
  initialStatus: string
  error: string
  onClose: () => void
  onSubmit: (event: FormEvent<HTMLFormElement>) => void
}) {
  const dialogRef = useRef<HTMLElement>(null)
  const onCloseRef = useRef(onClose)
  onCloseRef.current = onClose

  useEffect(() => {
    const previouslyFocused = document.activeElement as HTMLElement | null
    const dialog = dialogRef.current
    const focusable = dialog?.querySelectorAll<HTMLElement>('button:not([disabled]), input:not([disabled]), textarea:not([disabled]), select:not([disabled])')
    focusable?.[1]?.focus()

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        event.preventDefault()
        onCloseRef.current()
        return
      }
      if (event.key !== 'Tab' || !dialog) return
      const items = dialog.querySelectorAll<HTMLElement>('button:not([disabled]), input:not([disabled]), textarea:not([disabled]), select:not([disabled])')
      const first = items[0]
      const last = items[items.length - 1]
      if (!first || !last) return
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault()
        last.focus()
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault()
        first.focus()
      }
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => {
      document.removeEventListener('keydown', handleKeyDown)
      previouslyFocused?.focus()
    }
  }, [])

  return (
    <div className="dialog-backdrop" role="presentation" onMouseDown={event => { if (event.target === event.currentTarget) onClose() }}>
      <section ref={dialogRef} className="issue-dialog new-issue-dialog" role="dialog" aria-modal="true" aria-labelledby="create-issue-title" tabIndex={-1}>
        <header className="create-issue-header">
          <div><p className="eyebrow">Trackline / Issues</p><h2 id="create-issue-title">Create new issue</h2></div>
          <button className="icon-button" type="button" aria-label="Close new issue" onClick={onClose}><span className="material-symbols-outlined" aria-hidden="true">close</span></button>
        </header>
        <form className="new-issue-form" onSubmit={onSubmit}>
          <label>Project
            <select name="project_id" aria-label="Issue project" defaultValue={selectedProjectId ?? projects[0]?.id ?? ''} required>
              {projects.map(project => <option value={project.id} key={project.id}>{project.name}</option>)}
            </select>
          </label>
          <label>Title
            <input name="title" placeholder="Issue title..." maxLength={240} required />
          </label>
          <label>Description
            <textarea name="description" rows={6} placeholder="Add a detailed description or reproduction steps..." />
          </label>
          <div className="new-issue-fields">
            <label>Status<select name="status" defaultValue={initialStatus}><option value="todo">To do</option><option value="in_progress">In progress</option><option value="done">Done</option></select></label>
            <label>Priority<select name="priority" defaultValue="medium"><option value="critical">Urgent</option><option value="high">High</option><option value="medium">Medium</option><option value="low">Low</option></select></label>
            <label>Assignee ID<input name="assignee_id" type="number" min="1" placeholder="Unassigned" /></label>
          </div>
          <label>Labels <span className="optional-note">Comma-separated (applied after creation)</span>
            <input name="labels" placeholder="e.g. Security, iOS" maxLength={256} />
          </label>
          {error && <p className="error-banner" role="alert">{error}</p>}
          <footer className="new-issue-footer">
            <button className="secondary-button" type="button" onClick={onClose}>Cancel</button>
            <button className="primary-button" type="submit"><span className="material-symbols-outlined" aria-hidden="true">add</span>Create issue <kbd>⌘↵</kbd></button>
          </footer>
        </form>
      </section>
    </div>
  )
}
