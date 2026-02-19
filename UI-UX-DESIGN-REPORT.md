# UI/UX Design Research Report for FruitSalade

> Award-winning design patterns, best practices, and actionable recommendations
> for desktop and mobile experiences. Based on 2025-2026 trends.

---

## Table of Contents

1. [Award-Winning File Management UIs](#1-award-winning-file-management-uis)
2. [2025-2026 Design Trends](#2-2025-2026-design-trends)
3. [Design Tokens: Color, Typography, Spacing](#3-design-tokens-color-typography-spacing)
4. [Navigation Patterns (Desktop + Mobile)](#4-navigation-patterns-desktop--mobile)
5. [Layout Systems: Bento Grids, Cards, Data Tables](#5-layout-systems-bento-grids-cards-data-tables)
6. [Micro-Interactions & Animation](#6-micro-interactions--animation)
7. [Accessibility (WCAG 2.2)](#7-accessibility-wcag-22)
8. [Login & Authentication UX](#8-login--authentication-ux)
9. [File Browser & Gallery Patterns](#9-file-browser--gallery-patterns)
10. [Actionable Recommendations for FruitSalade](#10-actionable-recommendations-for-fruitsalade)

---

## 1. Award-Winning File Management UIs

### What Leaders Are Doing

**Google Drive** uses Material Design 3: clean left sidebar, top toolbar with list/grid
toggle, prominent search bar. Everything feels integrated through the Material You
design language.

**OneDrive** (2024-2025 redesign): dedicated Photos tab, cleaner sidebar, improved file
preview cards. Emphasizes visual previews and contextual actions.

**Nextcloud**: media-focused redesign proving self-hosted solutions can achieve polished UIs.

**Seafile 13** (2025): ultra-clean interface with libraries-as-folders metaphor, thumbnail
galleries, quick-share link generation, inline Markdown editing.

**pCloud**: purpose-built views per content type, letting users quickly access different
media file types.

### Key Takeaways

- Support both **list view** (data-dense) and **grid view** (visual/thumbnail) with a toggle
- Sidebar navigation is the universal standard for file management apps
- Dedicated **Photos/Gallery** view is expected, not a luxury
- Quick-share actions must be accessible within 1-2 clicks from any file
- Search must be prominent, persistent, and fast

---

## 2. 2025-2026 Design Trends

### Must-Have

| Trend | What It Means |
|-------|---------------|
| **Bento Grid Layouts** | Modular layouts with asymmetric but balanced card compositions. Mix large feature cards with compact widgets. Dominant dashboard pattern. |
| **Motion as Communication** | Animations guide users, confirm actions, build trust. A button pulsing after tap, a checkmark animating after submission -- these are expected. |
| **Accessibility as Core** | WCAG 2.2 is baseline. European Accessibility Act enforced since June 2025. |
| **Performance-First** | Every 1s delay drops conversions 20%. Clean interfaces, fewer animations, fast loading. |

### Competitive Differentiators

| Trend | What It Means |
|-------|---------------|
| **Hyper-Personalization** | Adapt to user behavior: recently accessed files, preferred views, customized layouts. |
| **Neuro-Diverse Design** | Reduce cognitive load, offer multiple input methods, allow density customization. |
| **Glassmorphism (Selective)** | Translucent/frosted surfaces for hierarchy, used sparingly. Apple's 2025 "Liquid Glass" validates this. |

### Data-Backed Statistics

- 88% of users won't return after a bad UX encounter
- $1 invested in UX generates $100 in returns (9,900% ROI)
- 60% of buyers return when personalization is done well
- Brand-consistent fonts increase engagement by 30%
- Quality UI can double conversions; refined UX can quadruple them

---

## 3. Design Tokens: Color, Typography, Spacing

### Color System

#### Light Mode

| Token | Purpose | Value |
|-------|---------|-------|
| `--bg` | Page background | `#F8FAFC` |
| `--surface` | Cards, panels | `#FFFFFF` |
| `--surface-variant` | Secondary surfaces | `#F1F5F9` |
| `--primary` | Brand, CTAs | `#2563EB` |
| `--primary-hover` | Interactive hover | `#1D4ED8` |
| `--on-primary` | Text on primary | `#FFFFFF` |
| `--secondary` | Secondary actions | `#64748B` |
| `--success` | Confirmations | `#16A34A` |
| `--warning` | Alerts | `#F59E0B` |
| `--error` | Errors, destructive | `#DC2626` |
| `--text` | Primary text | `#0F172A` |
| `--text-secondary` | Secondary text | `#475569` |
| `--text-muted` | Disabled, hints | `#94A3B8` |
| `--border` | Borders, dividers | `#E2E8F0` |
| `--border-strong` | Emphasized borders | `#CBD5E1` |

#### Dark Mode

| Token | Purpose | Value |
|-------|---------|-------|
| `--bg` | Page background | `#0C1120` (deep navy, NOT pure black) |
| `--surface` | Cards, panels | `#1E293B` |
| `--surface-variant` | Secondary surfaces | `#334155` |
| `--primary` | Brand, CTAs | `#60A5FA` (lighter blue for dark bg) |
| `--text` | Primary text | `#F8FAFC` |
| `--text-secondary` | Secondary text | `#94A3B8` |
| `--text-muted` | Disabled, hints | `#64748B` |
| `--border` | Borders | `#334155` |

#### Rules

- Never use pure black (`#000`) for dark mode -- use `#0C1120` to `#1A1A2E`
- Increase accent saturation 10-20% in dark mode
- Add 20-30% more padding in dark mode for visual breathing room
- Minimum **4.5:1** contrast for body text; **3:1** for large/bold text

### Typography Scale

| Token | Size | Weight | Line Height | Use Case |
|-------|------|--------|-------------|----------|
| `--text-display` | 2.25rem (36px) | 700 | 1.2 | Page titles |
| `--text-headline` | 1.5rem (24px) | 600 | 1.3 | Section headers |
| `--text-title` | 1.125rem (18px) | 600 | 1.4 | Card titles, nav |
| `--text-body` | 1rem (16px) | 400 | 1.5 | Default body |
| `--text-body-sm` | 0.875rem (14px) | 400 | 1.5 | Table cells, secondary |
| `--text-label` | 0.75rem (12px) | 500 | 1.4 | Labels, badges |
| `--text-caption` | 0.6875rem (11px) | 400 | 1.4 | Timestamps, metadata |

**Font Stack:** `-apple-system, BlinkMacSystemFont, "Segoe UI", "Inter", Roboto, sans-serif`

**Rules:**
- Minimum body text: 16px
- Use `rem` units (respects user zoom preferences)
- Line heights: multiples of 4px for vertical rhythm
- Limit to 2-3 font weights per page

### Spacing System (4px base, 8px primary grid)

| Token | Value | Use Case |
|-------|-------|----------|
| `--space-1` | 4px | Tight gaps, icon padding |
| `--space-2` | 8px | Inline spacing, small gaps |
| `--space-3` | 12px | List item padding |
| `--space-4` | 16px | Standard padding, card body |
| `--space-5` | 20px | Section gaps |
| `--space-6` | 24px | Card padding, section spacing |
| `--space-8` | 32px | Large section breaks |
| `--space-10` | 40px | Page-level spacing |
| `--space-12` | 48px | Major layout gaps |
| `--space-16` | 64px | Hero spacing |

**Rules:** All values must be multiples of 4px. Use CSS custom properties.

---

## 4. Navigation Patterns (Desktop + Mobile)

### Desktop (>1024px)

- Fixed left sidebar, 250-280px wide
- Icons AND text labels (icons for scanability, labels for clarity)
- Sections: Primary (Files, Gallery, Dashboard), Secondary (Shares), Admin (Users, Groups, Storage)
- Collapsible to icon-only mode (~64px) for more content space
- Top bar with search, user menu

### Tablet (768px-1024px)

- Sidebar collapses to icon-only by default
- Hamburger menu to expand full sidebar as overlay
- Top bar remains

### Mobile (<768px)

- Sidebar hidden entirely
- **Bottom navigation bar** with 4-5 primary destinations
- Hamburger in top bar for secondary/admin navigation
- Touch targets: minimum **48x48px**

### Sidebar Design Rules

- Active item: highlighted background + left accent border (3-4px, primary color)
- Hover state: subtle background change (5-8% opacity of primary)
- Nested items: indented 16px per level, max 2 levels deep
- Dividers between groups: 1px line with 8px vertical margin
- Scrollable if content exceeds viewport

---

## 5. Layout Systems: Bento Grids, Cards, Data Tables

### Bento Grid for Dashboard

```css
.dashboard-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
    gap: var(--space-4);
    padding: var(--space-6);
}

.card--featured {
    grid-column: span 2;
}

@media (min-width: 1024px) {
    .dashboard-grid {
        grid-template-columns: repeat(3, 1fr);
    }
}
```

### Card Design

```
+----------------------------------+
|  [Icon] Title              [...]  |  <- Header: 48px
|----------------------------------|
|  Content Area                    |  <- Body: flexible
|  (metric, list, chart, etc.)     |
|----------------------------------|
|  Footer / Action                 |  <- Footer: optional, 40px
+----------------------------------+
```

- Padding: 16-24px
- Border radius: **8-12px** (current 6px is slightly dated)
- Shadow: `0 1px 3px rgba(0,0,0,0.1), 0 1px 2px rgba(0,0,0,0.06)`
- Hover shadow: `0 4px 12px rgba(0,0,0,0.1)` with 200ms transition

### Data Tables on Mobile

**Desktop:** Full table with columns (Name, Size, Modified, Type, Actions)

**Tablet:** Hide less important columns, compress actions to icons

**Mobile:** Transform to card-list:
```
+--[icon]-- filename.pdf -----------+
|           2.4 MB  ·  Jan 15, 2025 |
|           [Share] [Download] [...] |
+------------------------------------+
```

**Implementation:** Use CSS Grid (not `<table>`). On mobile, each row becomes a stacked
card with `grid-template-areas`. Touch targets: 44x44px minimum, 48x48px preferred.

### Responsive Breakpoints

| Name | Value | Target |
|------|-------|--------|
| `sm` | 480px | Large phones |
| `md` | 768px | Tablets |
| `lg` | 1024px | Small laptops |
| `xl` | 1280px | Desktops |
| `2xl` | 1536px | Large desktops |

Use mobile-first `min-width` media queries.

---

## 6. Micro-Interactions & Animation

### Timing Standards

| Interaction | Duration | Easing |
|-------------|----------|--------|
| Hover state | 150-200ms | `ease-in-out` |
| Click feedback | 100ms | `ease-out` |
| Page transitions | 200-300ms | `ease-in-out` |
| Toast in/out | 300ms / 200ms | `ease-out` / `ease-in` |
| Modal open | 200-250ms | `cubic-bezier(0.4, 0, 0.2, 1)` |
| Modal close | 150-200ms | `cubic-bezier(0.4, 0, 1, 1)` |
| Skeleton shimmer | 1.5-2s loop | `linear` |
| Dropdown expand | 150-200ms | `ease-out` |
| Sidebar collapse | 200ms | `ease-in-out` |

### Essential Micro-Interactions

1. **File Upload Progress**: Per-file progress bar + filename + percentage + cancel button
2. **Drag-and-Drop Feedback**: Item elevates with shadow, drop zones highlight, 100ms settle
3. **Skeleton Loading**: Content-shaped placeholders with shimmer animation (replace spinners)
4. **Button States**: Idle -> Hover (150ms) -> Active (scale 0.97) -> Loading (spinner) -> Success (checkmark)
5. **Toast Notifications**: Slide-in from top-right, auto-dismiss after 4-5s, swipeable on mobile

### Skeleton Loading CSS

```css
@keyframes shimmer {
    0% { background-position: -200% 0; }
    100% { background-position: 200% 0; }
}
.skeleton {
    background: linear-gradient(90deg, #f0f0f0 25%, #e0e0e0 50%, #f0f0f0 75%);
    background-size: 200% 100%;
    animation: shimmer 1.5s infinite linear;
    border-radius: 4px;
}
```

### Performance Rules

- Use CSS `transform` and `opacity` only (GPU-accelerated)
- Never animate `width`, `height`, `top`, `left` (triggers layout reflow)
- Respect `prefers-reduced-motion`:
```css
@media (prefers-reduced-motion: reduce) {
    *, *::before, *::after {
        animation-duration: 0.01ms !important;
        transition-duration: 0.01ms !important;
    }
}
```

---

## 7. Accessibility (WCAG 2.2)

### Contrast Ratios

- Normal text (<18px): **4.5:1** minimum
- Large text (>=18px bold or >=24px): **3:1** minimum
- UI components & graphical objects: **3:1** minimum

### Touch/Click Targets

- WCAG 2.2 Level AA: minimum **24x24 CSS px**
- Apple HIG: recommended **44x44px**
- Material Design: recommended **48x48px**
- Adequate spacing between targets to prevent mis-taps

### Keyboard Navigation

- All interactive elements reachable via Tab
- Visible focus indicators (2-3px outline, high contrast)
- Focus trap for modals (Tab cycles within)
- Escape to close modals/dropdowns/overlays
- Arrow keys for menu navigation

### Drag-and-Drop (WCAG 2.2 new)

- Every drag action **must** have a non-drag alternative (menu/button)
- Support: Spacebar to pick up, Arrow keys to move, Space to drop

### Semantic HTML

- Use `<nav>`, `<main>`, `<aside>`, `<header>`, `<footer>` landmarks
- `<button>` for actions, `<a>` for navigation
- Logical heading hierarchy (`<h1>`-`<h6>`, never skip levels)
- `aria-label` for icon-only buttons
- `aria-live="polite"` for dynamic updates (upload progress, notifications)
- `role="status"` for toasts

### File Browser Specific

- File list: `role="grid"` or `role="table"` with proper row/cell roles
- Sort actions: announce new order via `aria-live`
- Selection: `aria-selected` attribute
- Breadcrumbs: `aria-label="Breadcrumb"` + `aria-current="page"`

### Testing Tools

- axe DevTools (browser extension)
- Lighthouse accessibility audit
- WAVE (WebAIM)
- Manual keyboard-only navigation testing

---

## 8. Login & Authentication UX

### Recommended Layout

Centered card on clean background. For self-hosted apps, simpler than split-screen:

```
+--------------------------------------------+
|          [FruitSalade Logo]                 |
|          Sign in to your account            |
|                                             |
|  Username                                   |
|  +--------------------------------------+   |
|  | admin                                |   |
|  +--------------------------------------+   |
|                                             |
|  Password                     [Forgot?]     |
|  +--------------------------------------+   |
|  | ********            [eye icon]       |   |
|  +--------------------------------------+   |
|                                             |
|  [x] Remember this device                   |
|                                             |
|  +--------------------------------------+   |
|  |          Sign In                     |   |
|  +--------------------------------------+   |
|                                             |
|  ── or ──                                   |
|                                             |
|  +--------------------------------------+   |
|  |  [G] Continue with SSO (OIDC)        |   |
|  +--------------------------------------+   |
+--------------------------------------------+
```

### Key Rules

- Labels **above** inputs, not floating/placeholder-only
- Password visibility toggle (eye icon)
- `autocomplete="username"` and `autocomplete="current-password"` for password managers
- Never clear email/username on failed login
- Error: adjacent to field, specific but not security-leaking
- Sign In button: primary color, full-width within card
- Minimum touch target: 44x44px

---

## 9. File Browser & Gallery Patterns

### File Browser View Modes

1. **List View** (default): sortable columns, checkbox selection
2. **Grid View**: thumbnail cards with filename below
3. **Compact List** (power users): denser rows, more items visible

### Interaction Patterns

- **Breadcrumb** at top: `/ > Documents > Projects`
- **Multi-select**: Click, Shift+Click (range), Ctrl+Click (toggle)
- **Context menu**: right-click (desktop), long-press (mobile), "..." overflow
- **Drag-and-drop**: move files, with menu alternative ("Move to...")
- **Inline rename**: double-click filename
- **Quick actions**: Share, Download, Delete on hover or row actions

### Toolbar

Desktop: View toggle, Sort, Filter, Upload (primary CTA), New Folder
Mobile: collapse to icon-only, Upload as FAB (Floating Action Button)

### Photo Gallery

- CSS Grid with `auto-fill`, `object-fit: cover` for justified layout
- Lazy-load images with blur-up placeholder or skeleton
- `loading="lazy"` on all `<img>` below the fold
- Albums: cover thumbnail, name, count, date range
- Lightbox: full-screen, arrow/swipe navigation, pinch-to-zoom, EXIF panel, Escape to close, preload adjacent images

### Drag-and-Drop Upload States

1. **Idle**: dashed border, "Drag files here or click to upload"
2. **Drag enter**: full-area highlight, blue/green tint
3. **Uploading**: per-file progress bars with filename, size, %, cancel
4. **Complete**: checkmark, auto-dismiss 3s
5. **Error**: red highlight, retry button, error message

Always provide a file picker button alongside drag-and-drop.

---

## 10. Actionable Recommendations for FruitSalade

### Priority 1: Design Token Foundation

| Change | Current | Recommended |
|--------|---------|-------------|
| Color tokens | 10 tokens | ~25 tokens (surfaces, text tiers, semantics) |
| Spacing | Ad-hoc rem/px | 4px-based scale (`--space-1` to `--space-16`) |
| Typography | Single body size | 7-tier scale (display to caption) |
| Border radius | 6px | 8-12px |
| Shadows | None defined | 3 elevation levels |
| Dark mode | Manual toggle, limited palette | Full token swap with proper dark values |

### Priority 2: Layout Modernization

1. Increase topbar height from 48px to 56-64px (better touch targets)
2. Make sidebar collapsible to icon-only (64px) with 200ms transition
3. Convert dashboard to bento grid: `repeat(auto-fit, minmax(280px, 1fr))`
4. Add breakpoints at 480/768/1024/1280px (mobile-first)
5. Add bottom nav for mobile (<768px) with 4-5 destinations
6. Convert file lists from `<table>` to CSS Grid for mobile card transformation

### Priority 3: Interaction & Polish

1. Skeleton loading states instead of blank screens/spinners
2. Transition animations: buttons (150ms), cards (200ms), modals (200ms)
3. Drag-and-drop upload with visual feedback states
4. View toggle (list/grid) for file browser
5. Improved toast notifications with slide-in animation
6. Respect `prefers-reduced-motion`

### Priority 4: Accessibility

1. Semantic landmarks: `<nav>`, `<main>`, `<aside>`, `<header>`
2. `aria-label` on all icon-only buttons
3. Visible focus states (2-3px outline) on all interactive elements
4. Keyboard navigation for file list (arrows, Enter, Space)
5. Contrast ratio verification (4.5:1 minimum)
6. `aria-live` regions for dynamic content

### Priority 5: Login Page

1. Labels above inputs
2. Password visibility toggle
3. `autocomplete` attributes for password managers
4. "Remember this device" checkbox
5. OIDC button styling
6. Inline validation with error messages

---

## The 5 Core Principles

1. **Consistency through tokens**: Every color, spacing, font size, shadow from a defined
   token. Enables dark mode, theming, and cohesion.

2. **Progressive disclosure**: Essential info first; details on demand. Metadata on hover,
   advanced settings behind expandable sections, secondary actions in overflow menus.

3. **Responsive by structure**: Use CSS Grid `auto-fit`/`minmax()` so layouts adapt
   naturally. Reserve media queries for structural changes (sidebar to bottom nav).

4. **Motion with purpose**: Every animation has a job -- confirming, guiding, showing state.
   No decorative animation. Respect reduced-motion.

5. **Accessible by default**: Semantic HTML, ARIA attributes, focus states, touch targets
   are the foundation, not features to add later.
