# FPL Picker Design System

## 1. Atmosphere & Identity

FPL Picker is a focused matchday command board: energetic, numerical, and immediately recognizable without imitating a cramped desktop table. Its signature is **purple pitch depth**—deep violet surfaces frame electric green recommendations, pink leadership markers, and cyan fixture context. The interface should feel decisive rather than decorative: every accent explains an action, status, or football datum.

## 2. Semantic Color Tokens

All implementations use semantic token names. Hex values exist only in this document and the platform theme declaration that maps these tokens.

| Role | Compose token | Light | Dark | Usage |
|---|---|---:|---:|---|
| Surface / canvas | `FplColors.canvas` | `#F7F3F8` | `#18001C` | Edge-to-edge app background |
| Surface / primary | `FplColors.surface` | `#FFFFFF` | `#240029` | Main sections and app bar |
| Surface / raised | `FplColors.raised` | `#F0E7F2` | `#2D0032` | Cards, grouped controls |
| Surface / selected | `FplColors.selected` | `#E5D6E8` | `#38003C` | Selected formula and emphasized rows |
| Text / primary | `FplColors.onSurface` | `#210625` | `#FFFFFF` | Headings and primary values |
| Text / secondary | `FplColors.onSurfaceMuted` | `#655568` | `#C9BCCA` | Labels, fixtures, supporting copy |
| Text / disabled | `FplColors.onSurfaceDisabled` | `#8F8391` | `#8F7D91` | Disabled controls and unavailable data |
| Text / on brand | `FplColors.onBrand` | `#FFFFFF` | `#FFFFFF` | Supporting text on the purple app bar |
| Border / default | `FplColors.outline` | `#CDBFD0` | `#59405D` | Cards, inputs, section dividers |
| Brand / purple | `FplColors.brandPurple` | `#38003C` | `#38003C` | Brand anchor and text on green |
| Accent / primary | `FplColors.accentGreen` | `#008A49` | `#00FF87` | Optimize action, scores, captain |
| Accent / primary container | `FplColors.accentGreenContainer` | `#B8F8D5` | `#004D2A` | Selected/positive backgrounds |
| Accent / pink | `FplColors.accentPink` | `#C00045` | `#FF4A88` | Vice-captain and urgent identity detail |
| Accent / pink container | `FplColors.accentPinkContainer` | `#FFD9E5` | `#650024` | Vice-captain background |
| Accent / cyan | `FplColors.accentCyan` | `#007D85` | `#04F5FF` | Formation, fixtures, neutral info |
| Status / warning | `FplColors.warning` | `#8A4B00` | `#FFB95C` | Stale-cache banner |
| Status / warning container | `FplColors.warningContainer` | `#FFF0DA` | `#4A2800` | Stale-cache surface |
| Status / error | `FplColors.error` | `#B3261E` | `#FFB4AB` | Validation and network errors |
| Status / error container | `FplColors.errorContainer` | `#FFDAD6` | `#690005` | Error-state surface |

Rules:
- Green is reserved for the primary action, recommendation score, and captain.
- Pink is reserved for vice-captain or urgent identity emphasis; cyan identifies formation and fixture context.
- Purple remains present in both themes so the product retains its FPL identity.
- Text/background combinations must meet WCAG AA; large numeric display text must meet at least 3:1.

## 3. Typography

Use Android's system sans family for reliable rendering, dynamic font scaling, and no network/font payload. Enable tabular figures for scores, costs, and gameweek values.

| Level | Compose token | Size | Weight | Line height | Usage |
|---|---|---:|---:|---:|---|
| Display | `FplType.display` | 32sp | 800 | 36sp | Starting-XI score |
| Title | `FplType.title` | 24sp | 800 | Screen title |
| Section | `FplType.section` | 20sp | 700 | Starting XI and bench headings |
| Card title | `FplType.cardTitle` | 16sp | 700 | Player names and state titles |
| Body | `FplType.body` | 16sp | 400 | Controls and primary copy |
| Label | `FplType.label` | 14sp | 600 | Metadata and compact controls |
| Caption | `FplType.caption` | 12sp | 500 | Fixture context and supporting status |

Rules:
- Respect system font scaling; never constrain text to a fixed-height parent.
- Use at most two lines for player names and fixture descriptions, but expose the complete text to accessibility services.
- Do not use all caps for paragraphs. Short positional and leadership badges may use uppercase.

## 4. Base-4 Spacing & Layout

Every dimension derives from a 4dp base unit.

| Token | Value | Usage |
|---|---:|---|
| `FplSpacing.xs` | 4dp | Badge internals, tight label gaps |
| `FplSpacing.sm` | 8dp | Inline groups, compact row gaps |
| `FplSpacing.md` | 12dp | Input/card internal gap |
| `FplSpacing.lg` | 16dp | Screen gutters and standard padding |
| `FplSpacing.xl` | 20dp | Comfortable card padding |
| `FplSpacing.xxl` | 24dp | Section separation |
| `FplSpacing.xxxl` | 32dp | State illustration breathing room |
| `FplSpacing.state` | 40dp | Empty/loading state vertical inset |
| `FplSpacing.touch` | 48dp | Minimum interactive target |

| Shape token | Value | Usage |
|---|---:|---|
| `FplShapes.small` | 4dp | Leadership and position badges |
| `FplShapes.medium` | 8dp | Inputs and buttons |
| `FplShapes.large` | 12dp | Player and summary cards |
| `FplShapes.hero` | 16dp | Primary score panel |

Layout rules:
- Single scrolling column with 16dp phone gutters and a centered maximum width of 720dp on larger devices.
- Apply system safe insets to all edges; the bottom action area never overlaps gesture navigation or the IME.
- At widths below 360dp, controls stack vertically. No horizontal scrolling is permitted.
- Lists use mobile player rows/cards, never the extension's six-column table.

## 5. Components, States & Accessibility

### Strategy control group
- **Structure:** labeled budget text field, three-choice formula selector, Optimize primary button, Refresh tonal button.
- **Spacing:** `md` within the group, `lg` outer padding; all controls use `touch` minimum height.
- **States:** default, focused, invalid, disabled, loading. Validation is inline and announced through live-region semantics.
- **Accessibility:** decimal keyboard with Done IME action; labels remain visible; formula choices expose selected state.

### Summary panel
- **Structure:** gameweek badge, display score, formation, XI cost, squad cost, and budget.
- **Spacing:** `xl` padding, `md` internal rhythm.
- **States:** current and stale. Stale age appears in a separate warning banner rather than changing the score color.
- **Accessibility:** one merged summary description with un-abbreviated currency and formation.

### Leadership card
- **Structure:** role badge, player name, team, score.
- **Variants:** captain uses green; vice-captain uses pink.
- **Spacing:** `lg` padding, `sm` internal gap.
- **Accessibility:** role is spoken as “Captain” or “Vice-captain,” not only “C” or “VC.”

### Player row
- **Structure:** position marker, name/team, cost/score, opponent and three-gameweek fixture context.
- **Variants:** starter, bench, captain, vice-captain, double-gameweek.
- **Spacing:** `lg` padding with `sm` gaps; consecutive rows are separated by the default outline.
- **Accessibility:** the row merges into a natural reading sentence; decorative separators are hidden.

### State panel
- **Variants:** initial/empty, loading, validation error, network error, and no-valid-squad.
- **Structure:** direct title, actionable explanation, and retry action where recovery is possible.
- **Accessibility:** loading exposes progress semantics; errors use assertive announcements; no state relies on color alone.

### Status banner
- **Variants:** stale-cache warning and data-count information.
- **Spacing:** `md` padding, `sm` internal gap.
- **Accessibility:** stale age uses plain language and is announced when it appears.

## 6. Motion & Interaction

| Token | Duration | Easing | Usage |
|---|---:|---|---|
| `FplMotion.micro` | 120ms | standard decelerate | Press/focus feedback |
| `FplMotion.standard` | 240ms | standard | State crossfade |
| `FplMotion.emphasis` | 400ms | emphasized decelerate | Result reveal |

Rules:
- Animate only opacity and transform-equivalent Compose properties; never animate measured layout dimensions.
- Honor the platform “remove animations” setting by presenting final states immediately.
- Optimization progress is indeterminate and honest; no fake percent-complete indicator.
- Disable Optimize while a request is active, but keep Refresh and error recovery behavior explicit.

## 7. Depth Strategy

Use **borders-only depth**. Purple-derived tonal surfaces establish hierarchy, while a single 1dp `outline` keyline separates cards and inputs. Compose elevation is always 0dp; no drop shadows, glows, glass effects, or decorative gradients. Selected/pressed hierarchy comes from semantic container colors and opacity, never extra elevation.
