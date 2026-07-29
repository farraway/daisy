---
name: css
description: CUBE CSS and Every Layout conventions — cascade layers, design tokens, layout compositions, logical properties, fluid sizing. Use when writing or modifying CSS, or when asked about stylesheet architecture, layout primitives, design tokens, spacing, or colour schemes.
short_description: CUBE CSS and Every Layout conventions
---

Follow the CUBE CSS methodology (Composition, Utility, Block, Exception) and
Every Layout principles.

The **composition** is the unit that carries layout. Blocks are small, own no
external spacing, and configure compositions rather than reimplementing them.
Layout source for the eight core compositions is in
[`compositions.md`](compositions.md) — read it when building or modifying
layout, not for token, colour, or utility questions.

## Principles

- **Work with the cascade** — global styles do most of the work;
  compositions handle layout; blocks stay small and focused.
- **Logical properties** — `margin-inline-*`, `padding-block-*`,
  `max-inline-size`, `inline-size` rather than their physical equivalents
  (`margin-left`, `padding-top`, `max-width`).
- **Content-driven sizing** — prefer suggestions (`min-block-size`,
  `flex-basis`, `max-inline-size`) over fixed dimensions.
- **Fluid-first** — `clamp()` for typography and spacing; `rem` for block
  spacing, `em` for inline, `ch` for measure. Reserve `px` for hairlines and
  other genuinely fixed details, never for typography or layout.
- **Measure axiom** — constrain `max-inline-size` on text elements (`20ch`
  for h1, `35ch` for h2–h3, `--measure` for prose). Container elements stay
  unconstrained.
- **Composition over inheritance** — compose from small primitives with
  single responsibilities. Layout belongs to the composition, never to the
  block.
- **Progressive enhancement** — cascade fallbacks; reach for `@supports`
  only when a fallback can't carry it.
- **Plan before building** — core build first, flair pass second. Try simple
  compositions first, and know when to stop.

## Architecture

**Cascade layer order:** `reset → globals → compositions → utilities →
blocks → exceptions`.

**Design tokens, three layers.** Every value comes from a token; hardcoded
colours, spacing, and type sizes are the thing this structure exists to
prevent.

- **Raw tokens** — generated values (`--size-step-N`, `--space-*`,
  `--color-*`)
- **Semantic variables** — intent layer (`--gutter`, `--stroke`,
  `--radius-*`, `--transition-*`)
- **Component properties** — block-level API (`--button-bg`,
  `--card-padding`)

## Global styles

- Target unclassed elements with `:not([class])` so blocks aren't fighting
  globals.
- Expose custom properties with fallbacks so blocks can override:
  `var(--hr-stroke, var(--stroke))`.
- Use `:where()` on global utility-like rules to keep specificity at zero.
- Use `text-wrap: balance` on headings.
- Use `:focus-visible` for keyboard focus outlines, not `:focus`.

## Blocks

- Expose configurable values as custom properties with defaults.
- Configure compositions (set `--flow-space`, `--gutter`, alignment) rather
  than reimplementing layout.
- Include a header comment documenting purpose, custom properties with
  defaults, and exceptions.
- Let the parent composition own the block's margin and width.

## Exceptions (variants)

- Use data attributes (`data-state`, `data-variant`, `data-layout`) rather
  than BEM modifier classes.
- Exception rules only reassign custom properties — they never duplicate CSS
  rules.
- Use `filter: brightness()` or `transform: scale()` for hover and active
  states, so they work across colour variants.

## Prose

Prose configures `--flow-space` for content elements within a flow
composition. It adjusts spacing only; layout stays with the composition.

Target content elements with `:is(h1, h2, h3, h4) + *:not([class])`
patterns. Usage: `<article class="prose wrapper flow">`.

## Utilities

Keep specificity at zero with `:where()`. Common utilities: text sizes, text
colours, `visually-hidden`, `indent`.

## Colour schemes

Data-attribute theming: `<body data-theme="name">` swaps `--color-*` custom
properties. The same pattern applies at component level (`data-variant`) and
site level (`data-theme`). Exception rules reassign the properties;
everything downstream reads `var(--color-*)`.

## Accessibility

- `:focus-visible` for keyboard focus, not `:focus`.
- `scroll-margin-block` on `:target` for hash-link offset.
- `:has(:focus-visible)` to disable sticky positioning when a child has
  focus.
- `max-inline-size` in `ch` units for readable line lengths.
