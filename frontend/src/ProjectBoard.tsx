import { ReactNode, useMemo, useState } from 'react'

export type Project = { id: number; name: string; description: string }
export type Issue = {
  id: number
  title: string
  description: string
  priority: string
  status: string
  assignee_id?: number | null
}

const columns = [
  { id: 'todo', label: 'To do', tone: 'todo' },
  { id: 'in_progress', label: 'In progress', tone: 'progress' },
  { id: 'done', label: 'Done', tone: 'done' },
] as const

type Status = (typeof columns)[number]['id']

type ProjectBoardProps = {
  screen: 'board' | 'projects' | 'components'
  projects: Project[]
  selectedProject: Project | undefined
  selectedProjectId: number | null
  issues: Issue[]
  loading: boolean
  error: string
  query: string
  statusFilter: string
  priorityFilter: string
  email: string
  children: ReactNode
  onSelectProject: (id: number) => void
  onShowProjects: () => void
  onShowComponents: () => void
  onOpenBoard: () => void
  onAddProject: () => void
  onAddIssue: (status?: string) => void
  onLogout: () => void
  onDismissError: () => void
  onQueryChange: (value: string) => void
  onStatusFilterChange: (value: string) => void
  onPriorityFilterChange: (value: string) => void
  onOpenIssue: (issue: Issue) => void
  onSetStatus: (issue: Issue, status: string) => void
}

export function ProjectBoard(props: ProjectBoardProps) {
  const [view, setView] = useState<'board' | 'list'>('board')
  const [sortDescending, setSortDescending] = useState(true)

  const sortedIssues = useMemo(() => {
    const priorityRank: Record<string, number> = { critical: 0, high: 1, medium: 2, low: 3 }
    return [...props.issues].sort((a, b) => {
      const priorityOrder = (priorityRank[a.priority] ?? 4) - (priorityRank[b.priority] ?? 4)
      const order = priorityOrder || a.id - b.id
      return sortDescending ? order : -order
    })
  }, [props.issues, sortDescending])

  const openCount = props.issues.filter(issue => issue.status !== 'done').length
  const completedCount = props.issues.filter(issue => issue.status === 'done').length
  const highCount = props.issues.filter(issue => ['high', 'critical'].includes(issue.priority) && issue.status !== 'done').length

  return (
    <div className="app-shell">
      <Sidebar
        projects={props.projects}
        selectedProjectId={props.selectedProjectId}
        screen={props.screen}
        email={props.email}
        onSelectProject={props.onSelectProject}
        onShowProjects={props.onShowProjects}
        onShowComponents={props.onShowComponents}
        onOpenBoard={props.onOpenBoard}
        onAddProject={props.onAddProject}
        onLogout={props.onLogout}
      />
      <div className="main-area">
        <TopBar projectName={props.selectedProject?.name} query={props.query} onQueryChange={props.onQueryChange} onAddIssue={props.onAddIssue} />
        <main className="board-content">
          {props.screen === 'board' ? <>
          <ProjectHeader
            project={props.selectedProject}
            openCount={openCount}
            completedCount={completedCount}
            highCount={highCount}
          />
          {props.error && (
            <div className="error-banner" role="alert">
              {props.error}
              <button type="button" onClick={props.onDismissError}>Dismiss</button>
            </div>
          )}
          <BoardToolbar
            query={props.query}
            statusFilter={props.statusFilter}
            priorityFilter={props.priorityFilter}
            view={view}
            sortDescending={sortDescending}
            onQueryChange={props.onQueryChange}
            onStatusFilterChange={props.onStatusFilterChange}
            onPriorityFilterChange={props.onPriorityFilterChange}
            onViewChange={setView}
            onSortChange={() => setSortDescending(current => !current)}
          />
          {props.loading ? (
            <p className="state-message" role="status">Loading issues…</p>
          ) : (
            <>
              {sortedIssues.length === 0 && <p className="state-message">No issues match these filters.</p>}
              {view === 'board' ? (
                <KanbanBoard issues={sortedIssues} onOpenIssue={props.onOpenIssue} onSetStatus={props.onSetStatus} onAddIssue={props.onAddIssue} />
              ) : sortedIssues.length > 0 ? (
                <IssueList issues={sortedIssues} onOpenIssue={props.onOpenIssue} onSetStatus={props.onSetStatus} />
              ) : null}
            </>
          )}
          {props.children}
          </> : props.screen === 'projects' ? <ProjectsDirectory projects={props.projects} selectedProjectId={props.selectedProjectId} onSelectProject={id => { props.onSelectProject(id); props.onOpenBoard() }} onAddProject={props.onAddProject} /> : <ComponentSheet />}
        </main>
      </div>
    </div>
  )
}

function Sidebar({
  projects,
  selectedProjectId,
  screen,
  email,
  onSelectProject,
  onShowProjects,
  onShowComponents,
  onOpenBoard,
  onAddProject,
  onLogout,
}: Pick<ProjectBoardProps, 'projects' | 'selectedProjectId' | 'screen' | 'email' | 'onSelectProject' | 'onShowProjects' | 'onShowComponents' | 'onOpenBoard' | 'onAddProject' | 'onLogout'>) {
  return (
    <aside className="sidebar">
      <div className="sidebar-main">
        <a className="brand" href="#board" aria-label="Trackline home">
          <span className="brand-icon material-symbols-outlined" aria-hidden="true">view_kanban</span>
          <span className="brand-copy"><strong>Trackline</strong><small>Engineering Workspace</small></span>
        </a>
        <nav className="primary-nav" aria-label="Workspace">
          <button className={`nav-item${screen === 'projects' ? ' active' : ''}`} type="button" onClick={onShowProjects}><span className="material-symbols-outlined" aria-hidden="true">folder</span>Projects</button>
          <span className="nav-item nav-item-disabled" aria-disabled="true" title="Not available yet"><span className="material-symbols-outlined" aria-hidden="true">checklist</span>My issues</span>
          <span className="nav-item nav-item-disabled" aria-disabled="true" title="Not available yet"><span className="material-symbols-outlined" aria-hidden="true">switch_account</span>Activity</span>
          <span className="nav-item nav-item-disabled" aria-disabled="true" title="Not available yet"><span className="material-symbols-outlined" aria-hidden="true">monitoring</span>Analytics</span>
          <button className={`nav-item${screen === 'components' ? ' active' : ''}`} type="button" onClick={onShowComponents}><span className="material-symbols-outlined" aria-hidden="true">palette</span>UI components</button>
        </nav>
        <div className="sidebar-projects">
          <div className="sidebar-section-title"><span>Projects</span><span className="material-symbols-outlined" aria-hidden="true">expand_more</span></div>
          <div className="project-list">
            {projects.map((project, index) => (
              <button
                className={`project-link${selectedProjectId === project.id ? ' selected' : ''}`}
                type="button"
                key={project.id}
                onClick={() => { onSelectProject(project.id); onOpenBoard() }}
              >
                <span className={`project-dot dot-${index % 4}`} />
                <span>{project.name}</span>
              </button>
            ))}
            {projects.length === 0 && <span className="sidebar-empty">No projects yet</span>}
          </div>
        </div>
      </div>
      <div className="sidebar-footer">
        <button className="new-project-button" type="button" onClick={onAddProject}>
          <span className="material-symbols-outlined" aria-hidden="true">add</span>New project
        </button>
        <div className="user-card">
          <span className="avatar">{email[0]?.toUpperCase() || 'U'}</span>
          <span className="user-copy"><strong>{email || 'Your workspace'}</strong><small>Member account</small></span>
          <button className="icon-button logout" type="button" aria-label="Sign out" title="Sign out" onClick={onLogout}>
            <span className="material-symbols-outlined" aria-hidden="true">logout</span>
          </button>
        </div>
      </div>
    </aside>
  )
}

function TopBar({ projectName, query, onQueryChange, onAddIssue }: {
  projectName?: string
  query: string
  onQueryChange: (value: string) => void
  onAddIssue: (status?: string) => void
}) {
  return (
    <header className="topbar">
      <div className="topbar-left">
        <nav className="breadcrumbs" aria-label="Breadcrumb">
          <span>Projects</span><span className="breadcrumb-separator">/</span>
          <span>{projectName || 'Your projects'}</span><span className="breadcrumb-separator">/</span>
          <strong>Board</strong>
        </nav>
        <label className="global-search">
          <span className="material-symbols-outlined" aria-hidden="true">search</span>
          <input aria-label="Global search" placeholder="Search issues, keys…" value={query} onChange={event => onQueryChange(event.target.value)} />
          <kbd>⌘K</kbd>
        </label>
      </div>
      <div className="topbar-actions">
        <button className="icon-button notification-button" type="button" aria-label="Notifications" title="Not available yet" disabled>
          <span className="material-symbols-outlined" aria-hidden="true">notifications</span><span className="notification-dot" />
        </button>
        <button className="icon-button" type="button" aria-label="Help" title="Not available yet" disabled><span className="material-symbols-outlined" aria-hidden="true">help</span></button>
        <span className="topbar-divider" />
        <button className="primary-button" type="button" onClick={() => onAddIssue()}>
          <span className="material-symbols-outlined" aria-hidden="true">add</span>New issue
        </button>
        <span className="avatar avatar-small" aria-label="Your account">U</span>
      </div>
    </header>
  )
}

function ProjectHeader({ project, openCount, completedCount, highCount }: {
  project?: Project
  openCount: number
  completedCount: number
  highCount: number
}) {
  return (
    <section className="project-header" aria-labelledby="project-title">
      <div className="project-summary">
        <div className="project-title-row">
          <h1 id="project-title">{project?.name || 'Your projects'}</h1>
          {project && <span className="project-status">Project board</span>}
        </div>
        <p>{project?.description || 'Create a project and start tracking what matters.'}</p>
      </div>
      <div className="project-stats" aria-label="Issue summary">
        <StatChip label="Open" value={openCount} tone="primary" />
        <StatChip label="Completed" value={completedCount} tone="success" />
        <StatChip label="High priority" value={highCount} tone="danger" />
      </div>
    </section>
  )
}

function ProjectsDirectory({ projects, selectedProjectId, onSelectProject, onAddProject }: {
  projects: Project[]
  selectedProjectId: number | null
  onSelectProject: (id: number) => void
  onAddProject: () => void
}) {
  const [query, setQuery] = useState('')
  const filtered = projects.filter(project => `${project.name} ${project.description}`.toLowerCase().includes(query.toLowerCase()))
  return <section className="directory-page" aria-labelledby="directory-title">
    <div className="directory-heading">
      <div><p className="eyebrow">Workspace / Projects</p><h1 id="directory-title">Projects Directory</h1><p>All your team’s work, organized in one place.</p></div>
      <button type="button" className="primary-button" onClick={onAddProject}><span className="material-symbols-outlined" aria-hidden="true">add</span>New project</button>
    </div>
    <div className="directory-toolbar"><label className="directory-search"><span className="material-symbols-outlined" aria-hidden="true">search</span><input aria-label="Search projects" placeholder="Search projects..." value={query} onChange={event => setQuery(event.target.value)} /></label><span>{filtered.length} {filtered.length === 1 ? 'project' : 'projects'}</span></div>
    {filtered.length ? <div className="project-directory-grid">{filtered.map((project, index) => <article className="directory-card" key={project.id}>
      <div className="directory-card-top"><span className={`directory-project-icon dot-${index % 4}`}><span className="material-symbols-outlined" aria-hidden="true">folder_open</span></span><span className="project-status">Project</span></div>
      <h2>{project.name}</h2><p>{project.description || 'No project description yet.'}</p>
      <div className="directory-card-meta"><span><span className="material-symbols-outlined" aria-hidden="true">confirmation_number</span>Issues available in board</span><span>#{project.id}</span></div>
      <button type="button" className={`directory-open${selectedProjectId === project.id ? ' current' : ''}`} onClick={() => onSelectProject(project.id)}>{selectedProjectId === project.id ? 'Open current board' : 'Open project'}<span className="material-symbols-outlined" aria-hidden="true">arrow_forward</span></button>
    </article>)}</div> : <div className="directory-empty"><span className="material-symbols-outlined" aria-hidden="true">folder_off</span><h2>{projects.length ? 'No matching projects' : 'No projects yet'}</h2><p>{projects.length ? 'Try another search.' : 'Create a project to give your team a place to track issues.'}</p>{!projects.length && <button type="button" className="primary-button" onClick={onAddProject}>Create your first project</button>}</div>}
  </section>
}

function ComponentSheet() {
  const [copied, setCopied] = useState('')
  async function copyToken(token: string) {
    try { await navigator.clipboard.writeText(token); setCopied(token); window.setTimeout(() => setCopied(''), 1400) } catch { setCopied('') }
  }
  return <section className="component-sheet" aria-labelledby="component-sheet-title">
    <div className="component-sheet-heading"><div><p className="eyebrow">Workspace / UI Components</p><h1 id="component-sheet-title">Design System</h1><p>Production tokens and interface patterns used throughout Trackline.</p></div><span className="design-system-badge"><span className="material-symbols-outlined" aria-hidden="true">verified</span>Trackline UI v1.0</span></div>
    <div className="component-sheet-grid">
      <section className="component-section"><div className="component-section-heading"><div><h2>Buttons & actions</h2><p>Clear hierarchy for frequent and destructive actions.</p></div></div><div className="component-samples"><button className="primary-button" type="button">Primary action</button><button className="secondary-button" type="button">Secondary</button><button className="ghost-button" type="button">Ghost action</button><button className="danger-button" type="button">Delete issue</button><button className="primary-button" type="button" disabled>Disabled</button></div></section>
      <section className="component-section"><div className="component-section-heading"><div><h2>Form fields</h2><p>Accessible inputs with consistent spacing and focus behavior.</p></div></div><div className="component-samples component-inputs"><label>Text input<input placeholder="Enter a value" /></label><label>Search<input placeholder="Search issues..." /></label><label>Select<select defaultValue="medium"><option value="low">Low priority</option><option value="medium">Medium priority</option><option value="high">High priority</option></select></label></div></section>
      <section className="component-section"><div className="component-section-heading"><div><h2>Status & priority</h2><p>Compact semantic states used on cards and issue details.</p></div></div><div className="component-samples"><span className="status-pill todo">To do</span><span className="status-pill progress">In progress</span><span className="status-pill done">Done</span>{['critical','high','medium','low'].map(priority => <span key={priority} className={`priority-badge ${priority}`}>{priority}</span>)}</div></section>
      <section className="component-section"><div className="component-section-heading"><div><h2>Surface & color tokens</h2><p>Select a swatch to copy its CSS custom property name.</p></div></div><div className="token-grid">{[['Primary', '#5149d8', '--primary'], ['Surface', '#ffffff', '--surface'], ['Soft surface', '#f5f5fa', '--surface-low'], ['Text', '#252b3b', '--text'], ['Muted text', '#74798b', '--text-secondary'], ['Danger', '#c43d56', '--danger']].map(([name, color, token]) => <button className="token-swatch" type="button" key={token} onClick={() => void copyToken(token)}><span style={{ backgroundColor: color }} /><strong>{name}</strong><code>{copied === token ? 'Copied!' : token}</code></button>)}</div></section>
      <section className="component-section component-wide"><div className="component-section-heading"><div><h2>Issue card specimen</h2><p>Example combines identity, priority, descriptive copy and ownership.</p></div></div><article className="issue-card specimen-card"><div className="issue-card-top"><span className="issue-id">TRK-104</span><span className="priority-badge high">High</span></div><strong className="specimen-title">Improve keyboard navigation in issue board</strong><p className="issue-description">Keep focus visible while moving between columns and issue actions.</p><div className="issue-card-bottom"><span className="assignee-chip">Assignee #12</span><span className="status-pill progress">In progress</span></div></article></section>
    </div>
  </section>
}

function StatChip({ label, value, tone }: { label: string; value: number; tone: string }) {
  return <span className={`stat-chip ${tone}`}><span className="stat-dot" />{label}: {value}</span>
}

function BoardToolbar({
  query,
  statusFilter,
  priorityFilter,
  view,
  sortDescending,
  onQueryChange,
  onStatusFilterChange,
  onPriorityFilterChange,
  onViewChange,
  onSortChange,
}: {
  query: string
  statusFilter: string
  priorityFilter: string
  view: 'board' | 'list'
  sortDescending: boolean
  onQueryChange: (value: string) => void
  onStatusFilterChange: (value: string) => void
  onPriorityFilterChange: (value: string) => void
  onViewChange: (value: 'board' | 'list') => void
  onSortChange: () => void
}) {
  return (
    <div className="board-toolbar">
      <div className="toolbar-filters">
        <label className="board-search">
          <span className="material-symbols-outlined" aria-hidden="true">filter_list</span>
          <input aria-label="Search issues" placeholder="Filter issues…" value={query} onChange={event => onQueryChange(event.target.value)} />
        </label>
        <label className="filter-select"><span className="visually-hidden">Filter by status</span>
          <select aria-label="Filter by status" value={statusFilter} onChange={event => onStatusFilterChange(event.target.value)}>
            <option value="">Status: All</option><option value="todo">Status: To do</option><option value="in_progress">Status: In progress</option><option value="done">Status: Done</option>
          </select>
        </label>
        <label className="filter-select"><span className="visually-hidden">Filter by priority</span>
          <select aria-label="Filter by priority" value={priorityFilter} onChange={event => onPriorityFilterChange(event.target.value)}>
            <option value="">Priority: Any</option><option value="critical">Priority: Critical</option><option value="high">Priority: High</option><option value="medium">Priority: Medium</option><option value="low">Priority: Low</option>
          </select>
        </label>
        <button className="sort-button" type="button" onClick={onSortChange} aria-label={`Sort priority ${sortDescending ? 'descending' : 'ascending'}`}>
          Sort: Priority {sortDescending ? '↓' : '↑'}<span className="material-symbols-outlined" aria-hidden="true">swap_vert</span>
        </button>
      </div>
      <div className="view-toggle" role="group" aria-label="Issue view">
        <button type="button" className={view === 'board' ? 'selected' : ''} aria-pressed={view === 'board'} onClick={() => onViewChange('board')}>
          <span className="material-symbols-outlined" aria-hidden="true">view_column</span>Board
        </button>
        <button type="button" className={view === 'list' ? 'selected' : ''} aria-pressed={view === 'list'} onClick={() => onViewChange('list')}>
          <span className="material-symbols-outlined" aria-hidden="true">format_list_bulleted</span>List
        </button>
      </div>
    </div>
  )
}

function KanbanBoard({ issues, onOpenIssue, onSetStatus, onAddIssue }: {
  issues: Issue[]
  onOpenIssue: (issue: Issue) => void
  onSetStatus: (issue: Issue, status: string) => void
  onAddIssue: (status?: string) => void
}) {
  return (
    <div className="kanban-board">
      {columns.map(column => {
        const columnIssues = issues.filter(issue => issue.status === column.id)
        return (
          <KanbanColumn key={column.id} status={column.id} label={column.label} tone={column.tone} issues={columnIssues} onOpenIssue={onOpenIssue} onSetStatus={onSetStatus} onAddIssue={onAddIssue} />
        )
      })}
    </div>
  )
}

function KanbanColumn({ status, label, tone, issues, onOpenIssue, onSetStatus, onAddIssue }: {
  status: Status
  label: string
  tone: string
  issues: Issue[]
  onOpenIssue: (issue: Issue) => void
  onSetStatus: (issue: Issue, status: string) => void
  onAddIssue: (status?: string) => void
}) {
  return (
    <section className={`kanban-column ${tone}`} aria-label={`${label} issues`}>
      <header className="column-header">
        <div className="column-heading"><span className="column-indicator" /><h2>{label}</h2><span className="column-count">{issues.length}</span></div>
        <button className="icon-button column-menu" type="button" aria-label={`Add issue to ${label}`} title={`Add issue to ${label}`} onClick={() => onAddIssue(status)}>
          <span className="material-symbols-outlined" aria-hidden="true">add</span>
        </button>
      </header>
      <div className="column-cards">
        {issues.map(issue => <IssueCard key={issue.id} issue={issue} onOpenIssue={onOpenIssue} onSetStatus={onSetStatus} />)}
        {issues.length === 0 && <p className="empty-column">No issues</p>}
      </div>
      <button className="add-issue-button" type="button" onClick={() => onAddIssue(status)}>
        <span className="material-symbols-outlined" aria-hidden="true">add</span>Add issue
      </button>
    </section>
  )
}

function IssueCard({ issue, onOpenIssue, onSetStatus }: {
  issue: Issue
  onOpenIssue: (issue: Issue) => void
  onSetStatus: (issue: Issue, status: string) => void
}) {
  return (
    <article className={`issue-card ${issue.status === 'done' ? 'is-done' : ''}`}>
      <div className="issue-card-top">
        <span className="issue-id">#{issue.id}</span>
        <span className={`priority-badge ${issue.priority}`}><span className="priority-icon material-symbols-outlined" aria-hidden="true">{priorityIcon(issue.priority)}</span>{capitalize(issue.priority)}</span>
      </div>
      <button className="issue-title" type="button" onClick={() => onOpenIssue(issue)}>{issue.title}</button>
      {issue.description && <p className="issue-description">{issue.description}</p>}
      <div className="issue-card-bottom">
        {issue.assignee_id ? <span className="assignee-chip">Assignee #{issue.assignee_id}</span> : <span className="assignee-chip unassigned">Unassigned</span>}
        <label className="status-control"><span className="visually-hidden">Change issue status</span>
          <select aria-label="Change issue status" value={issue.status} onChange={event => onSetStatus(issue, event.target.value)}>
            {columns.map(column => <option key={column.id} value={column.id}>{column.label}</option>)}
          </select>
        </label>
      </div>
    </article>
  )
}

function IssueList({ issues, onOpenIssue, onSetStatus }: {
  issues: Issue[]
  onOpenIssue: (issue: Issue) => void
  onSetStatus: (issue: Issue, status: string) => void
}) {
  return (
    <div className="issue-list" role="list" aria-label="Issues">
      {issues.map(issue => (
        <div className="issue-list-row" role="listitem" key={issue.id}>
          <span className="issue-id">#{issue.id}</span>
          <button className="issue-title" type="button" onClick={() => onOpenIssue(issue)}>{issue.title}</button>
          <span className={`priority-badge ${issue.priority}`}>{capitalize(issue.priority)}</span>
          <label className="status-control"><span className="visually-hidden">Change issue status</span>
            <select aria-label="Change issue status" value={issue.status} onChange={event => onSetStatus(issue, event.target.value)}>
              {columns.map(column => <option key={column.id} value={column.id}>{column.label}</option>)}
            </select>
          </label>
        </div>
      ))}
    </div>
  )
}

function priorityIcon(priority: string) {
  if (priority === 'critical' || priority === 'high') return 'priority_high'
  if (priority === 'low') return 'arrow_downward'
  return 'signal_cellular_alt_2_bar'
}

function capitalize(value: string) {
  return value ? `${value[0].toUpperCase()}${value.slice(1)}` : 'Unknown'
}
