# Project management, assignment, and calendar

This implementation follows the core YouTrack-inspired workflow requested for
this tracker; it is not an attempt to reproduce all YouTrack features.

## Role model

- The Auth service's `admin` role is platform-wide and remains distinct from
  project membership roles.
- A Project service `admin` is the project manager: they can manage project
  membership and promote/demote project members.
- A project `member` is a contributor. Contributors can view project members
  and work with issues, but cannot change team membership or project roles.
- Projects retain their owner as an admin and must retain at least one admin.

All role checks are performed in the Project service. The frontend only
reflects permissions; it is not the authorization boundary. The Project
service owns `project_members` and the Issue service calls its membership
endpoint rather than reading Project database tables.

## Team and assignments

- `GET /projects/{projectId}/members` lists user IDs and project roles for a
  project member.
- Project admins use `POST /projects/{projectId}/members` to add members,
  `PATCH` to set a member's `admin` or `member` role, and `DELETE` with
  `user_id` to remove a member.
- Authenticated username autocomplete is used by the admin UI to find users.
  Search responses expose only user ID and username, not email or password
  data.
- Issue create/update accepts an optional `assignee_id`. The Issue service
  checks that the ID belongs to the project through the Project service.
  Issue tables store user IDs only; no cross-service foreign key is used.

## Schedule and calendar

Issue schedule fields belong to the Issue service:

- `start_date` and `due_date` are optional date-only fields; if both exist, due
  date must be the same day or later than the planned start date.
- `assigned_at` is set when an assignee is assigned or changed.
- `started_at` is set the first time an issue enters `in_progress`.
- `completed_at` is set when an issue enters `done` (and cleared if reopened).

`GET /issues/calendar?project_id=...&from=YYYY-MM-DD&to=YYYY-MM-DD` returns
scheduled or timestamped project issues overlapping the inclusive date range.
Ranges may span at most 93 days and results are capped at 500 issues. The
React calendar currently displays a month grid; issue details expose exact
milestone timestamps and allow editing the planned dates and assignee.

Apply the new schema with:

```sh
make migrate-up MIGRATE_DATABASE=issue_db
```

Migration `000005_issue_schedule` adds nullable schedule/milestone columns and
project/date indexes. Its down migration removes those fields and therefore
discards their values.

## YouTrack research and deliberate scope

The design was informed by the official documentation:

- [Default roles](https://www.jetbrains.com/help/youtrack/server/default-roles.html)
- [Configure a project](https://www.jetbrains.com/help/youtrack/cloud/configuring-a-project.html)
- [Due Date widget](https://www.jetbrains.com/help/youtrack/cloud/due-date-widget.html)
- [Gantt charts](https://www.jetbrains.com/help/youtrack/cloud/create-and-use-gantt-chart.html)

YouTrack offers a substantially larger platform, including configurable
workflows, custom fields, advanced permissions, reports, notifications, time
tracking, and Gantt planning. Those features are not represented as complete
in this release. The current slice is limited to project roles/team control,
 member assignment, planned start/due dates, timestamped work milestones, and a
 project calendar.

## Workspace sections

- **My issues** lists issues assigned to the signed-in user across accessible
  projects, with completed issues hidden until requested. It paginates through
  each project with bounded concurrent service calls.
- **Activity** merges the latest 100 Activity-service events per project. It is
  a recent feed, not guaranteed-complete history; failures for individual
  projects do not hide events from other projects.
- **Analytics** shows current counts by status and priority for the selected
  project, plus open high/critical issues. It does not infer trends or velocity
  from the event stream.

## Platform user administration

The **Users** sidebar entry is available only to global Auth admins. The
Auth-owned `GET /users` endpoint provides a paginated account list, and
`POST /users/{userId}/password-reset` stores a bcrypt hash for an admin-chosen
temporary password. This is a reset flow—not a way to view an existing
password. Reset users must choose their own new password at next sign-in; the
gateway restricts their reset-marked JWT to the password-change endpoint until
Auth issues a fresh token. Project admins do not receive platform-wide user
administration rights. Apply migration `000003_password_reset` to `auth_db`:

```sh
make migrate-up MIGRATE_DATABASE=auth_db
```

Global admins can also create member accounts with temporary passwords,
activate/deactivate accounts, and use **Manage projects** on a user row to add
or remove that account's membership across projects. New accounts must change
their temporary password at first sign-in. Deactivation is reversible and
preserves memberships, issue assignments, and historical attribution; inactive
accounts cannot sign in or be newly assigned work. Reactivation restores access
to retained project memberships. Self-deactivation is blocked to avoid
accidentally locking out the signed-in administrator. Apply migration
`000004_user_active` to `auth_db` as well (or migrate all services with the
command above).
