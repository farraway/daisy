---
name: css
description: CUBE CSS and Every Layout conventions for CSS architecture, layout, and styling. Use when writing or modifying CSS files, discussing CSS architecture/layout/styling conventions, working with CUBE CSS or Every Layout primitives, or asked about design tokens, spacing, or colour schemes.
short_description: CUBE CSS and Every Layout conventions
---

# CUBE CSS Best Practices

## Rules

1. **No `px` for typography or layout.** Use `clamp()` for fluid sizing, `rem` for block spacing, `em` for inline, `ch` for measure.
2. **Use logical properties.** Write `margin-inline-*`, `padding-block-*`, `max-inline-size` — not `margin-left`, `padding-top`, `max-width`.
3. **Follow cascade layer order.** `reset → globals → compositions → utilities → blocks → exceptions`.
4. **Use design tokens; never hardcode values.** Colors, spacing, and type scales come from tokens.
5. **No component-specific layout CSS.** Compose from primitives; blocks must not own their external spacing.
6. **Constrain measure on text elements.** Apply `max-inline-size` to headings and prose; leave container elements unconstrained.

Follow the CUBE CSS methodology (Composition, Utility, Block, Exception) and Every Layout principles.

## 1. Principles

- **Work with the cascade** — global styles do most of the work; compositions handle layout; blocks are small and focused
- **Logical properties** — use `margin-inline-*`, `padding-block-*`, `max-inline-size`, `inline-size` instead of physical equivalents
- **Content-driven sizing** — prefer suggestions (`min-block-size`, `flex-basis`, `max-inline-size`) over fixed dimensions
- **Fluid-first** — `clamp()` for typography and spacing; `rem` for block spacing, `em` for inline, `ch` for measure; no `px` for typography or layout
- **Measure axiom** — constrain `max-inline-size` on text elements (`20ch` for h1, `35ch` for h2-h3, `--measure` for prose); container elements are unconstrained
- **Composition over inheritance** — compose from small primitives with single responsibilities; no component-specific layout CSS
- **Progressive enhancement** — cascade fallbacks; avoid `@supports` unless necessary
- **Plan before building** — core build first, flair pass second; try simple compositions first; know when to stop

## 2. Architecture

**Cascade layer order:** reset, globals, compositions, utilities, blocks, exceptions.

**Design tokens — three layers:**
- **Raw tokens:** generated values (`--size-step-N`, `--space-*`, `--color-*`)
- **Semantic variables:** intent layer (`--gutter`, `--stroke`, `--radius-*`, `--transition-*`)
- **Component properties:** block-level API (`--button-bg`, `--card-padding`)

## 3. Compositions

Layout primitives that each do one thing. Components never set their own margin or width.

### flow — vertical rhythm

```css
.flow > * + * {
  margin-block-start: var(--flow-space, 1em);
}
```

### wrapper — max-width centred container

```css
.wrapper {
  max-inline-size: var(--wrapper-max-width, 85rem);
  margin-inline: auto;
  padding-inline: var(--gutter);
  position: relative;
}
```

### cluster — horizontal wrapping group

```css
.cluster {
  display: flex;
  flex-wrap: wrap;
  gap: var(--gutter, var(--space-m));
  justify-content: var(--cluster-horizontal-alignment, flex-start);
  align-items: var(--cluster-vertical-alignment, center);
}
```

### sidebar — intrinsic two-column

```css
.sidebar {
  display: flex;
  flex-wrap: wrap;
  gap: var(--gutter, var(--space-s-l));
}

.sidebar:not([data-direction]) > :first-child {
  flex-basis: var(--sidebar-target-width, 20rem);
  flex-grow: 1;
}

.sidebar:not([data-direction]) > :last-child {
  flex-basis: 0;
  flex-grow: 999;
  min-inline-size: var(--sidebar-content-min-width, 50%);
}

.sidebar[data-direction='rtl'] > :last-child {
  flex-basis: var(--sidebar-target-width, 20rem);
  flex-grow: 1;
}

.sidebar[data-direction='rtl'] > :first-child {
  flex-basis: 0;
  flex-grow: 999;
  min-inline-size: var(--sidebar-content-min-width, 50%);
}
```

### grid — auto-fill responsive

```css
.grid {
  display: grid;
  grid-template-columns: repeat(
    var(--grid-placement, auto-fill),
    minmax(min(var(--grid-min-item-size, 16rem), 100%), 1fr)
  );
  gap: var(--gutter, var(--space-l));
}

.grid[data-layout='50-50'] {
  --grid-placement: auto-fit;
  --grid-min-item-size: clamp(16rem, 50vw, 33rem);
}

.grid[data-layout='thirds'] {
  --grid-placement: auto-fit;
  --grid-min-item-size: clamp(16rem, 33%, 20rem);
}
```

### repel — push items apart

```css
.repel {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  align-items: var(--repel-vertical-alignment, center);
  gap: var(--gutter, var(--space-m));
}

.repel[data-nowrap] {
  flex-wrap: nowrap;
}
```

### region — vertical section spacing

```css
.region {
  padding-block: var(--region-space, var(--space-m));
  position: relative;
}
```

### switcher — container-based switch

```css
.switcher {
  display: flex;
  flex-wrap: wrap;
  gap: var(--gutter, var(--space-l));
  align-items: var(--switcher-vertical-alignment, flex-start);
}

.switcher > * {
  flex-grow: 1;
  flex-basis: calc((var(--switcher-target-container-width, 40rem) - 100%) * 999);
}

.switcher > :nth-child(n + 3) {
  flex-basis: 100%;
}
```

### Extended set (use when needed)

- **cover** — vertically centred content with optional header/footer
- **frame** — fixed aspect-ratio container (images, video)
- **reel** — horizontal scroll carousel
- **imposter** — centred overlay (dialogs, modals)
- **icon** — inline icon alignment with text

## 4. Writing Patterns

### Global styles

- Target unclassed elements with `:not([class])` so blocks aren't fighting globals
- Use custom properties with fallbacks so blocks can override: `var(--hr-stroke, var(--stroke))`
- Use `:where()` on global utility-like rules to keep specificity at zero
- Use `text-wrap: balance` on headings
- Use `:focus-visible` for keyboard focus outlines, not `:focus`

### Blocks

- Expose all configurable values as custom properties with defaults
- Configure compositions (set `--flow-space`, `--gutter`, alignment) rather than reimplementing layout
- Include a header comment documenting purpose, custom properties with defaults, and exceptions
- Never set margin or width on the block itself; let the parent composition handle that

### Exceptions (variants)

- Use data attributes (`data-state`, `data-variant`, `data-layout`) instead of BEM modifier classes
- Exception rules only reassign custom properties. Never duplicate CSS rules in exceptions
- Use `filter: brightness()` or `transform: scale()` for hover/active states that work across colour variants

### Prose pattern

- Prose configures `--flow-space` for different content elements within a flow composition
- Prose does not do layout. It only adjusts spacing
- Target content elements with `:is(h1, h2, h3, h4) + *:not([class])` patterns
- Usage: `<article class="prose wrapper flow">`

### Utilities

- Use `:where()` to keep specificity at zero
- Common: text sizes, text colours, `visually-hidden`, `indent`

### Colour schemes

- Data-attribute theming: `<body data-theme="name">` swaps `--color-*` custom properties
- Same pattern at component level (`data-variant`) and site level (`data-theme`)
- Exception rules reassign properties. Everything downstream uses `var(--color-*)`

### Accessibility

- `:focus-visible` for keyboard focus, not `:focus`
- `scroll-margin-block` on `:target` for hash link offset
- `:has(:focus-visible)` to disable sticky when a child has focus
- `max-inline-size` with `ch` units for readable line lengths
