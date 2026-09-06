# Bill Ledger Design System

UI/UX Pro Max generated the product direction as a mobile-first personal-finance
dashboard with a responsive Bento layout. This file records the refined visual
system used by the application.

## Product direction

- Style: refined editorial ledger with tactile Bento cards
- Character: calm, trustworthy, personal, materially different from an admin template
- Density: 7/10; compact data without sacrificing touch space
- Motion: 4/10; 150–300 ms state transitions, no decorative continuous motion
- Layout: two-column mobile cash-flow cards, adaptive four/six-column desktop grid

## Color

| Role | Light | Dark |
| --- | --- | --- |
| Canvas | `#F7F6F2` | `#11121A` |
| Surface | `#FFFEFB` | `#1B1D2A` |
| Ink | `#1C1E2A` | `#F7F5F1` |
| Primary | `#252D5B` | `#6F7CCD` |
| Accent | `#F47052` | `#FF8969` |
| Income | `#26785E` | `#68B897` |
| Expense | `#C34041` | `#F77070` |
| Border | `#DBD9D5` | `#393B4C` |

Coral is reserved for selected navigation, small status marks, and focused
actions. Income and expense always include direction, icon, or text—not color alone.

## Type and data

- UI: Manrope, with PingFang SC / Microsoft YaHei fallbacks
- Money, dates, indices, and metadata: IBM Plex Mono
- Use tabular figures for every amount
- Display headings use tight tracking; body text remains at least 16 px on mobile forms

## App icon

- Source of truth: `icon-preview/icon-3d-1024.png`; `icon-3d.svg` is only a preview wrapper
- Silhouette: a warm-white open ledger with a deep-navy cover and one coral `¥` seal on a medium slate-indigo glass tile
- Contrast rule: the warm-white ledger must remain clearly separated from the mid-tone tile at 64 px; the subject and background cannot share the same dominant tone
- The app icon shares Notepad's square glass 3D language—not its cyan palette—with a luminous rim, compact near-edge subject scale, visible page layers, soft ambient shadow, and a raised glossy `¥` seal
- fnOS and PWA bitmap assets must be rendered from the 1024 px master instead of edited independently
- The browser loads the dedicated 16 px and 32 px PNG derivatives for predictable small-size rendering

## Components

- Primary action: full pill, deep navy background; coral in dark mode
- Secondary action: surface pill with a visible 1 px border
- Cards: 20–28 px radius, warm solid surface, thin border, low diffuse shadow
- Inputs: 48 px minimum height, 16 px radius, persistent visible labels
- Navigation: navy sidebar on desktop, floating warm dock on mobile
- Icons: one outline SVG family, 1.8–2 px stroke, no emoji

## Accessibility and responsive rules

- WCAG AA text contrast and visible 3 px focus rings
- Touch targets at least 44×44 px
- Bottom safe-area padding and content clearance for the floating mobile dock
- Breakpoints checked at 375, 768, 1024, and 1440 px, including landscape
- No horizontal page scrolling
- `prefers-reduced-motion` reduces all transitions and animations
- Charts have text labels, legends, titles, and a useful empty state
