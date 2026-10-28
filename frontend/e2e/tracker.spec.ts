import { expect, test } from '@playwright/test'

test('registration, login, project, issue edit, filters, and status update', async ({ page }) => {
  const suffix = `${Date.now()}-${Math.floor(Math.random() * 10000)}`
  const email = `playwright-${suffix}@example.test`
  const password = 'playwright-password-123'
  await page.goto('/')
  await page.getByRole('button', { name: 'Create account' }).last().click()
  await page.getByLabel('Email address').fill(email)
  await page.locator('input[placeholder="At least 8 characters"]').fill(password)
  await page.getByRole('button', { name: 'Create account' }).last().click()
  await expect(page.getByRole('heading', { name: 'Welcome back' })).toBeVisible()
  await page.getByLabel('Email address').fill(email)
  await page.locator('input[placeholder="At least 8 characters"]').fill(password)
  await page.getByRole('button', { name: 'Sign in' }).click()

  await page.getByRole('button', { name: 'New project' }).click()
  await page.locator('#project-name').fill(`E2E ${suffix}`)
  await page.getByPlaceholder('Short description').fill('Playwright flow')
  await page.getByRole('button', { name: 'Create project' }).click()
  await expect(page.getByRole('heading', { name: `E2E ${suffix}` })).toBeVisible()
  await page.getByRole('button', { name: 'Open current board' }).click()
  const title = `e2e issue ${suffix}`
  await page.getByRole('button', { name: 'Add issue to In progress' }).click()
  await expect(page.getByRole('dialog')).toBeVisible()
  await expect(page.getByRole('dialog').getByLabel('Status')).toHaveValue('in_progress')
  await page.getByPlaceholder('Issue title...').fill(title)
  await page.getByPlaceholder('Add a detailed description or reproduction steps...').fill('Initial issue detail')
  await page.getByRole('button', { name: /Create issue/ }).click()
  await expect(page.getByRole('button', { name: title })).toBeVisible()
  await expect(page.locator('.kanban-board')).toBeVisible()
  await page.getByRole('button', { name: 'List', exact: true }).click()
  await expect(page.getByRole('list', { name: 'Issues' })).toBeVisible()
  await page.getByRole('group', { name: 'Issue view' }).getByRole('button', { name: 'Board', exact: true }).click()

  await page.getByRole('button', { name: title }).click()
  await expect(page.getByRole('dialog')).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(page.getByRole('dialog')).toBeHidden()
  await page.getByRole('button', { name: title }).click()
  await expect(page.getByRole('dialog')).toBeVisible()
  await page.getByLabel('New label').fill('playwright')
  await page.getByRole('button', { name: 'Add label' }).click()
  await expect(page.getByText('playwright', { exact: true })).toBeVisible()
  await page.getByRole('tab', { name: /Activity/ }).click()
  await expect(page.getByText(/Issue created|Status changed/).first()).toBeVisible()
  await page.getByRole('tab', { name: 'Details' }).click()
  await page.getByLabel('Title', { exact: true }).fill(`${title} edited`)
  await page.getByLabel('Description').fill('Edited through detail view')
  await page.getByRole('dialog').locator('select[name="priority"]').selectOption('high')
  await page.getByRole('button', { name: 'Save changes' }).click()
  await expect(page.getByRole('button', { name: `${title} edited` })).toBeVisible()

  await page.getByLabel('Search issues').fill('does-not-match')
  await expect(page.getByText('No issues match these filters.')).toBeVisible()
  await expect(page).toHaveURL(/q=does-not-match/)
  await page.getByLabel('Search issues').fill('')
  await page.getByLabel('Filter by priority').selectOption('high')
  await expect(page.getByRole('button', { name: `${title} edited` })).toBeVisible()
  await expect(page).toHaveURL(/priority=high/)
  await page.getByLabel('Change issue status').selectOption('done')
  await page.getByLabel('Filter by priority').selectOption('')
  await expect(page.locator('.kanban-column').last().getByRole('button', { name: `${title} edited` })).toBeVisible()

  await page.getByRole('button', { name: 'Projects', exact: true }).first().click()
  await expect(page.getByRole('heading', { name: 'Projects Directory' })).toBeVisible()
  await page.getByRole('button', { name: `Open current board` }).click()
  await expect(page.getByRole('heading', { name: `E2E ${suffix}` })).toBeVisible()
  await expect(page.getByRole('button', { name: 'UI components' })).toHaveCount(0)
  await expect(page.getByRole('heading', { name: 'Design System' })).toHaveCount(0)

  const issueCard = page.locator('.issue-card').filter({ has: page.getByRole('button', { name: `${title} edited` }) })
  await issueCard.getByRole('button', { name: /Delete issue/ }).click()
  await page.getByRole('alertdialog').getByRole('button', { name: 'Delete issue' }).click()
  await expect(page.getByRole('button', { name: `${title} edited` })).toHaveCount(0)

  await page.getByRole('button', { name: 'Projects', exact: true }).first().click()
  await page.getByRole('button', { name: `Delete project E2E ${suffix}` }).click()
  await page.getByRole('alertdialog').getByRole('button', { name: 'Delete project' }).click()
  await expect(page.getByRole('heading', { name: 'No projects yet' })).toBeVisible()
})

test('auth screen fits a narrow viewport', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/')
  await expect(page.getByRole('heading', { name: 'Welcome back' })).toBeVisible()
  const overflows = await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth)
  expect(overflows).toBe(false)
})

test('required password update is balanced on desktop and mobile', async ({ page }) => {
  await page.addInitScript(() => {
    const payload = btoa(JSON.stringify({ sub: 42, role: 'member', exp: 4_000_000_000, must_change_password: true }))
    localStorage.setItem('token', `header.${payload}.signature`)
  })
  await page.setViewportSize({ width: 1920, height: 960 })
  await page.goto('/')
  await expect(page.getByRole('heading', { name: 'Choose a new password' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Sign out' })).toBeVisible()
  const desktopLayout = await page.evaluate(() => {
    const security = document.querySelector('.password-security-panel')!.getBoundingClientRect()
    const form = document.querySelector('.password-change-panel')!.getBoundingClientRect()
    return { viewportWidth: window.innerWidth, securityRight: security.right, formLeft: form.left, documentWidth: document.documentElement.scrollWidth }
  })
  expect(desktopLayout.securityRight).toBeGreaterThan(desktopLayout.viewportWidth * 0.3)
  expect(desktopLayout.formLeft).toBeGreaterThan(desktopLayout.securityRight - 2)
  expect(desktopLayout.documentWidth).toBe(desktopLayout.viewportWidth)

  await page.setViewportSize({ width: 850, height: 900 })
  const tabletWidth = await page.evaluate(() => document.documentElement.scrollWidth)
  expect(tabletWidth).toBe(850)

  await page.setViewportSize({ width: 390, height: 844 })
  await expect(page.getByRole('textbox', { name: 'New password', exact: true })).toBeVisible()
  const mobileWidth = await page.evaluate(() => document.documentElement.scrollWidth)
  expect(mobileWidth).toBe(390)
})

test('global admin project directory uses available width and adapts', async ({ page }) => {
  const projects = Array.from({ length: 12 }, (_, index) => ({
    id: index + 1,
    name: `Workspace project ${index + 1}`,
    description: `Project description ${index + 1}`,
    owner_id: index + 10,
  }))
  await page.addInitScript(() => {
    const payload = btoa(JSON.stringify({ sub: 1, role: 'admin', exp: 4_000_000_000 }))
    localStorage.setItem('token', `header.${payload}.signature`)
  })
  await page.route('**/api/**', async route => {
    const url = new URL(route.request().url())
    if (url.pathname === '/api/projects') {
      await route.fulfill({ json: projects })
    } else if (url.pathname === '/api/users/names') {
      const ownerIDs = JSON.parse(route.request().postData() || '{}').user_ids as number[]
      await route.fulfill({ json: ownerIDs.map(id => ({ id, username: `creator-${id}`, active: true })) })
    } else if (url.pathname === '/api/users/directory') {
      await route.fulfill({ json: { users: [], next_after_id: null } })
    } else {
      await route.fulfill({ json: [] })
    }
  })

  await page.setViewportSize({ width: 1920, height: 960 })
  await page.goto('/')
  await page.getByRole('button', { name: 'Projects' }).first().click()
  await expect(page.getByRole('heading', { name: 'All user projects' })).toBeVisible()
  const desktopLayout = await page.evaluate(() => ({
    directory: document.querySelector('.directory-page')!.getBoundingClientRect().width,
    projectGrid: getComputedStyle(document.querySelector('.platform-project-list')!).gridTemplateColumns.split(' ').length,
    projectCard: document.querySelector('.platform-project-card')!.getBoundingClientRect().width,
    document: document.documentElement.scrollWidth,
  }))
  expect(desktopLayout.directory).toBeGreaterThan(1250)
  expect(desktopLayout.projectGrid).toBe(3)
  expect(desktopLayout.projectCard).toBeLessThan(600)
  expect(desktopLayout.document).toBe(1920)

  for (const [width, columns] of [[850, 2], [390, 1]]) {
    await page.setViewportSize({ width, height: 900 })
    await expect(page.getByRole('heading', { name: 'All user projects' })).toBeVisible()
    const layout = await page.evaluate(() => ({
      document: document.documentElement.scrollWidth,
      columns: getComputedStyle(document.querySelector('.platform-project-list')!).gridTemplateColumns.split(' ').length,
    }))
    expect(layout.document).toBe(width)
    expect(layout.columns).toBe(columns)
  }
})
