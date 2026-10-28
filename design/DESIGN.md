# Trackline project-board design reference

This document records the design tokens and visual guidance retrieved from the Stitch project **Trackline Issue Tracker UI**, screen **Trackline - Project Kanban Board**. The source screen is saved in [`board.html`](./board.html) and is a visual reference only; the React app is implemented independently.

- Stitch project: `projects/8862440017293089640`
- Stitch screen: `projects/8862440017293089640/screens/8032dbd1a1f4416d85dc5210eba82a90`
- Reference viewport: 2880 × 2048, desktop
- Design system: Precision Engineering Canvas
- Typeface: Inter

## Color tokens

Values below are from the screen's generated Tailwind theme and Stitch design-system theme. Prefer these tokens over hard-coded colors in the React implementation.

```css
:root {
  --color-background: #f8f9ff;
  --color-surface: #f8f9ff;
  --color-surface-bright: #f8f9ff;
  --color-surface-dim: #cbdbf5;
  --color-surface-container-lowest: #ffffff;
  --color-surface-container-low: #eff4ff;
  --color-surface-container: #e5eeff;
  --color-surface-container-high: #dce9ff;
  --color-surface-container-highest: #d3e4fe;
  --color-surface-variant: #d3e4fe;
  --color-on-surface: #0b1c30;
  --color-on-surface-variant: #464555;
  --color-on-background: #0b1c30;
  --color-primary: #3525cd;
  --color-primary-container: #4f46e5;
  --color-on-primary: #ffffff;
  --color-on-primary-container: #dad7ff;
  --color-primary-fixed: #e2dfff;
  --color-primary-fixed-dim: #c3c0ff;
  --color-secondary: #565e74;
  --color-secondary-container: #dae2fd;
  --color-on-secondary: #ffffff;
  --color-tertiary: #684000;
  --color-tertiary-container: #885500;
  --color-error: #ba1a1a;
  --color-error-container: #ffdad6;
  --color-outline: #777587;
  --color-outline-variant: #c7c4d8;
  --color-surface-tint: #4d44e3;
  --color-inverse-surface: #213145;
  --color-inverse-on-surface: #eaf1ff;
  --color-inverse-primary: #c3c0ff;

  /* Semantic accents used by the board cards. */
  --color-priority-high: #e11d48;
  --color-priority-high-soft: #fff1f2;
  --color-priority-medium: #d97706;
  --color-priority-medium-soft: #fffbeb;
  --color-priority-low: #059669;
  --color-priority-low-soft: #ecfdf5;
  --color-success: #10b981;
}
```

The Stitch theme's complete named-color map is:

| Token | Value | Token | Value |
|---|---|---|---|
| `background` | `#f8f9ff` | `on_background` | `#0b1c30` |
| `surface` | `#f8f9ff` | `on_surface` | `#0b1c30` |
| `surface_bright` | `#f8f9ff` | `surface_dim` | `#cbdbf5` |
| `surface_container_lowest` | `#ffffff` | `surface_container_low` | `#eff4ff` |
| `surface_container` | `#e5eeff` | `surface_container_high` | `#dce9ff` |
| `surface_container_highest` | `#d3e4fe` | `surface_variant` | `#d3e4fe` |
| `surface_tint` | `#4d44e3` | `on_surface_variant` | `#464555` |
| `primary` | `#3525cd` | `on_primary` | `#ffffff` |
| `primary_container` | `#4f46e5` | `on_primary_container` | `#dad7ff` |
| `primary_fixed` | `#e2dfff` | `primary_fixed_dim` | `#c3c0ff` |
| `on_primary_fixed` | `#0f0069` | `on_primary_fixed_variant` | `#3323cc` |
| `secondary` | `#565e74` | `on_secondary` | `#ffffff` |
| `secondary_container` | `#dae2fd` | `on_secondary_container` | `#5c647a` |
| `secondary_fixed` | `#dae2fd` | `secondary_fixed_dim` | `#bec6e0` |
| `on_secondary_fixed` | `#131b2e` | `on_secondary_fixed_variant` | `#3f465c` |
| `tertiary` | `#684000` | `on_tertiary` | `#ffffff` |
| `tertiary_container` | `#885500` | `on_tertiary_container` | `#ffd4a4` |
| `tertiary_fixed` | `#ffddb8` | `tertiary_fixed_dim` | `#ffb95f` |
| `on_tertiary_fixed` | `#2a1700` | `on_tertiary_fixed_variant` | `#653e00` |
| `error` | `#ba1a1a` | `on_error` | `#ffffff` |
| `error_container` | `#ffdad6` | `on_error_container` | `#93000a` |
| `outline` | `#777587` | `outline_variant` | `#c7c4d8` |
| `inverse_surface` | `#213145` | `inverse_on_surface` | `#eaf1ff` |
| `inverse_primary` | `#c3c0ff` |  |  |

The screen additionally uses Tailwind semantic accents: high priority `rose-600` on `rose-50`, medium `amber-600` on `amber-50`, low `emerald-600` on `emerald-50`, and completed states `emerald-500`/`emerald-100`.

## Typography

Font family: `Inter` (400, 500, 600, 700). Apply tabular numerals to issue identifiers.

| Style | Size | Weight | Line height | Letter spacing |
|---|---:|---:|---:|---:|
| `display` | 28px | 600 | 36px | -0.02em |
| `headline-lg` | 22px | 600 | 28px | -0.015em |
| `headline-md` | 18px | 600 | 24px | -0.01em |
| `headline-sm` | 15px | 600 | 20px | -0.005em |
| `body-lg` | 15px | 400 | 22px | normal |
| `body-md` | 13px | 400 | 18px | normal |
| `body-sm` | 12px | 400 | 16px | normal |
| `label-md` | 13px | 500 | 18px | normal |
| `label-sm` | 11px | 500 | 14px | 0.01em |
| `code-sm` | 12px | 500 | 16px | -0.01em |

## Spacing, shape, and elevation

The design uses a 4px base grid. Stitch spacing tokens:

| Token | Value |
|---|---:|
| `space-xs` | 0.25rem / 4px |
| `space-sm`, `gutter-dense` | 0.5rem / 8px |
| `space-md` | 0.75rem / 12px |
| `space-lg`, `gutter` | 1rem / 16px |
| `space-xl` | 1.5rem / 24px |
| `margin-panel` | 1rem / 16px |
| `margin` | 1.5rem / 24px |

Radius scale: `sm` 4px, default 8px, `md` 12px, `lg` 16px, `xl` 24px, `full` 9999px. Main issue cards use a 12px radius and 12px padding; buttons and inputs use 6–8px radii. Use crisp 1px borders. Resting card shadow is `0 1px 3px rgba(15, 23, 42, 0.05)`; hover is `0 4px 6px -1px rgba(15, 23, 42, 0.07), 0 2px 4px -2px rgba(15, 23, 42, 0.04)`.

## Layout and component guidance

- Desktop workspace shell: fixed 240px sidebar, sticky 56px top navigation, fluid canvas.
- Sidebar contains brand/workspace, primary navigation, project list, create-project action, and user row.
- Board header contains project title/description, sprint status, members, and issue summary chips.
- Toolbar contains issue search/filter, status/assignee/priority/label filters, sorting, and Board/List switch.
- Kanban board has three visible lanes: To do, In progress, Done. Each lane has a header/count, add/menu actions, issue cards, and an add-issue affordance.
- Issue cards show ID, priority, title, labels, comment count, and assignee avatar when the API supplies the data. Do not fabricate backend data for details the API does not provide.
- Target desktop density is compact. The supplied HTML has a 1440px minimum width; the React implementation should adapt the shell and board to narrower viewports rather than preserving that fixed minimum.
- Keep the reference HTML as a visual artifact only. Build the React UI with semantic components and the app's existing gateway APIs.
