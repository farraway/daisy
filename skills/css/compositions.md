# Layout compositions

Each composition does one thing. Components never set their own margin or
width — they configure these instead, via the custom properties each exposes.

## flow — vertical rhythm

```css
.flow > * + * {
  margin-block-start: var(--flow-space, 1em);
}
```

## wrapper — max-width centred container

```css
.wrapper {
  max-inline-size: var(--wrapper-max-width, 85rem);
  margin-inline: auto;
  padding-inline: var(--gutter);
  position: relative;
}
```

## cluster — horizontal wrapping group

```css
.cluster {
  display: flex;
  flex-wrap: wrap;
  gap: var(--gutter, var(--space-m));
  justify-content: var(--cluster-horizontal-alignment, flex-start);
  align-items: var(--cluster-vertical-alignment, center);
}
```

## sidebar — intrinsic two-column

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

## grid — auto-fill responsive

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

## repel — push items apart

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

## region — vertical section spacing

```css
.region {
  padding-block: var(--region-space, var(--space-m));
  position: relative;
}
```

## switcher — container-based switch

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

## Extended set

Reach for these when the core eight don't fit:

- **cover** — vertically centred content with optional header/footer
- **frame** — fixed aspect-ratio container (images, video)
- **reel** — horizontal scroll carousel
- **imposter** — centred overlay (dialogs, modals)
- **icon** — inline icon alignment with text
