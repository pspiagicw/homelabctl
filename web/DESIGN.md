# homelabctl design: "matte terminal"

The visual language of the homelabctl web UI, written down so it can be reused
elsewhere (e.g. a blog). Source of truth: [`static/style.css`](static/style.css).

## Principles

- **Matte, solid colours.** Low-saturation fills, no gradients, no glossy effects.
- **Flat surfaces.** Cards are separated by a 1px outline, not drop shadows.
- **Monospace everywhere.** One typeface (JetBrains Mono) for body, headings
  and code, so the page reads like a well-formatted terminal.
- **Quiet chrome, loud state.** Labels and metadata are muted; colour is spent
  only on meaning (ok / warning / error, primary actions).
- **Square-ish corners.** 3–8px radii, never fully round.
- **Light and dark** follow the OS (`prefers-color-scheme`); every colour is a
  token with a value for both.

## Colour tokens

Semantic names are given first; the name used in homelabctl's CSS is in brackets.

| Token | Role | Light | Dark |
|---|---|---|---|
| `--bg` | page background | `#e6e7e9` | `#181a1c` |
| `--surface` | cards, panels | `#f1f2f3` | `#212427` |
| `--on-surface` | body text | `#2b2f33` | `#d3d6d9` |
| `--muted` | labels, metadata, secondary text | `#6b7178` | `#868d94` |
| `--outline` | borders, dividers | `#d0d3d7` | `#32363a` |
| `--bar` | top bar background | `#37474f` | `#263036` |
| `--on-bar` | top bar text | `#e3e7ea` | `#d3d9dd` |
| `--bar-muted` | top bar secondary text | `#9aa8b0` | `#7f8d95` |
| `--accent` | focus rings, hover borders, links | `#5c7a8a` | `#7f9cab` |
| `--positive` (`--boot`) | primary / positive button | `#4e7a52` | `#46704a` |
| `--danger` (`--shutdown`) | destructive button | `#9a4e48` | `#8c4741` |
| `--on-btn` | text on filled buttons | `#f3f4f2` | `#eceeec` |
| `--ok-bg` / `--ok-fg` | success tint / text | `#dbe5da` / `#3d6640` | `#25332a` / `#98bd9c` |
| `--warn-bg` / `--warn-fg` | warning tint / text | `#ece2cf` / `#82601f` | `#362f22` / `#d0ae74` |
| `--error-bg` / `--error-fg` | error tint / text | `#ecdad8` / `#893a34` | `#3a2625` / `#d99c97` |
| `--neutral-bg` / `--neutral-fg` (`--chip-off*`) | neutral chips, icon wells | `#dcdee1` / `#555b61` | `#2d3134` / `#a3a9af` |

Status colours always come as a pair: a pale tint for the background and a
darker (light theme) or paler (dark theme) foreground for text on it.

```css
:root {
	--bg: #e6e7e9;       --surface: #f1f2f3;   --on-surface: #2b2f33;
	--muted: #6b7178;    --outline: #d0d3d7;
	--bar: #37474f;      --on-bar: #e3e7ea;    --bar-muted: #9aa8b0;
	--accent: #5c7a8a;
	--positive: #4e7a52; --danger: #9a4e48;    --on-btn: #f3f4f2;
	--ok-bg: #dbe5da;    --ok-fg: #3d6640;
	--warn-bg: #ece2cf;  --warn-fg: #82601f;
	--error-bg: #ecdad8; --error-fg: #893a34;
	--neutral-bg: #dcdee1; --neutral-fg: #555b61;
}

@media (prefers-color-scheme: dark) {
	:root {
		--bg: #181a1c;       --surface: #212427;   --on-surface: #d3d6d9;
		--muted: #868d94;    --outline: #32363a;
		--bar: #263036;      --on-bar: #d3d9dd;    --bar-muted: #7f8d95;
		--accent: #7f9cab;
		--positive: #46704a; --danger: #8c4741;    --on-btn: #eceeec;
		--ok-bg: #25332a;    --ok-fg: #98bd9c;
		--warn-bg: #362f22;  --warn-fg: #d0ae74;
		--error-bg: #3a2625; --error-fg: #d99c97;
		--neutral-bg: #2d3134; --neutral-fg: #a3a9af;
	}
}
```

## Typography

- **Typeface:** [JetBrains Mono](https://www.jetbrains.com/lp/mono/), weights
  400 and 600 only, self-hosted as woff2 (SIL Open Font License; ship the
  licence file with the fonts). Get them from `@fontsource/jetbrains-mono`.
- **Fallback stack:** `"JetBrains Mono", ui-monospace, "SF Mono", Menlo, Consolas, "Liberation Mono", monospace`
- **Base:** 14px, line-height 1.55, `-webkit-font-smoothing: antialiased`.

| Use | Size | Weight | Extra |
|---|---|---|---|
| Page title (top bar) | 17px | 600 | letter-spacing 0.2px |
| Card / section heading | 17px | 600 | |
| Body | 14px | 400 | |
| Secondary text, descriptions | 12.5–13px | 400 | `--muted` |
| Label / eyebrow above a heading | 11px | 400–600 | UPPERCASE, letter-spacing 1.2px, `--muted` |
| Chips, tags | 10–11px | 600 | UPPERCASE, letter-spacing 0.8px |
| Footer, timestamps | 11–12px | 400 | `--muted` |

```css
@font-face {
	font-family: "JetBrains Mono";
	src: url("/fonts/jetbrains-mono-400.woff2") format("woff2");
	font-weight: 400;
	font-display: swap;
}
@font-face {
	font-family: "JetBrains Mono";
	src: url("/fonts/jetbrains-mono-600.woff2") format("woff2");
	font-weight: 600;
	font-display: swap;
}
body {
	margin: 0;
	background: var(--bg);
	color: var(--on-surface);
	font-family: "JetBrains Mono", ui-monospace, "SF Mono", Menlo, Consolas, "Liberation Mono", monospace;
	font-size: 14px;
	line-height: 1.55;
	-webkit-font-smoothing: antialiased;
}
```

## Shape and spacing

| | Value |
|---|---|
| Radius: cards, panels, icon wells | 8px |
| Radius: buttons | 6px |
| Radius: chips | 4px |
| Radius: small tags | 3px |
| Card padding | 18px (dense tiles: 14px) |
| Grid / stack gap | 16px (dense: 10–12px) |
| Page gutter | 16px |
| Max content width | 1320px (dashboard); use ~70ch for prose |
| Borders | 1px solid `--outline`; in-card dividers 1px **dashed** |
| Status stripe | 3–4px solid left border in a status colour |

## Components

### Top bar
Solid `--bar` strip, title in `--on-bar` with a dimmed `~$` prompt in front.

```html
<header class="app-bar"><h1><span class="prompt">~$</span> homelabctl</h1></header>
```
```css
.app-bar { background: var(--bar); color: var(--on-bar); padding: 14px 16px; }
.app-bar h1 { margin: 0; font-size: 17px; font-weight: 600; letter-spacing: 0.2px; }
.prompt { color: var(--bar-muted); margin-right: 4px; }
```

### Card
```css
.card {
	background: var(--surface);
	border: 1px solid var(--outline);
	border-radius: 8px;
	padding: 18px;
}
```
A card heading is a small uppercase **label** above a 17px/600 title:
```css
.label { font-size: 11px; color: var(--muted); text-transform: uppercase; letter-spacing: 1.2px; }
```

### Key: value list
Muted keys with a trailing colon, separated from the card body by a dashed rule.
```css
.meta {
	display: grid;
	grid-template-columns: max-content 1fr;
	gap: 6px 16px;
	margin: 0;
	padding-top: 14px;
	border-top: 1px dashed var(--outline);
	font-size: 13px;
}
.meta dt { color: var(--muted); }
.meta dt::after { content: ":"; }
.meta dd { margin: 0; }
```

### Chip
```css
.chip {
	padding: 3px 10px;
	border-radius: 4px;
	font-size: 11px;
	font-weight: 600;
	text-transform: uppercase;
	letter-spacing: 0.8px;
	background: var(--neutral-bg);
	color: var(--neutral-fg);
}
.chip-ok    { background: var(--ok-bg);    color: var(--ok-fg); }
.chip-warn  { background: var(--warn-bg);  color: var(--warn-fg); }
.chip-error { background: var(--error-bg); color: var(--error-fg); }
/* outlined variant for low-emphasis tags */
.chip-outline { background: transparent; border: 1px solid var(--outline); color: var(--muted); }
```

### Buttons
Filled buttons are solid colour, 6px radius, no shadow; hover/active only shift
brightness. A text button is transparent with an outline.
```css
.btn {
	display: inline-flex;
	align-items: center;
	justify-content: center;
	gap: 8px;
	min-height: 42px;
	padding: 10px 20px;
	border: none;
	border-radius: 6px;
	font: inherit;
	font-size: 13px;
	font-weight: 600;
	letter-spacing: 0.3px;
	color: var(--on-btn);
	background: var(--positive);
	cursor: pointer;
	transition: filter 0.15s;
}
.btn-danger { background: var(--danger); }
.btn:hover { filter: brightness(1.1); }
.btn:active { filter: brightness(0.9); }
.btn:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
.btn:disabled { cursor: not-allowed; filter: saturate(0.3); opacity: 0.5; }

.btn-text {
	padding: 6px 10px;
	border: 1px solid var(--outline);
	border-radius: 6px;
	background: transparent;
	color: var(--muted);
	font: inherit;
	font-size: 12px;
	cursor: pointer;
}
.btn-text:hover { color: var(--on-surface); border-color: var(--muted); }
```

### Link tile
A whole card as a link: icon well on the left, name/description/meta on the right.
Hover changes only the border colour.
```css
.tile {
	display: flex;
	gap: 12px;
	padding: 14px;
	color: inherit;
	text-decoration: none;
	transition: border-color 0.15s;
}
.tile:hover { border-color: var(--accent); }
.tile-icon {
	flex: none;
	display: grid;
	place-items: center;
	width: 40px;
	height: 40px;
	border-radius: 8px;
	background: var(--neutral-bg);
	color: var(--neutral-fg);
	font-weight: 600;
}
```

### Status stripe and list items
A coloured left border marks the status of a card or list entry.
```css
.stripe-ok    { border-left: 4px solid var(--ok-fg); }
.stripe-warn  { border-left: 4px solid var(--warn-fg); }
.stripe-error { border-left: 4px solid var(--error-fg); }
```

### Blinking cursor
The terminal touch next to a heading; it stops when the user prefers reduced motion.
```css
.cursor { margin-left: 2px; color: var(--muted); animation: blink 1.1s steps(1) infinite; }
@keyframes blink { 50% { opacity: 0; } }
@media (prefers-reduced-motion: reduce) { .cursor { animation: none; } }
```
```html
<h2>mains: up<span class="cursor" aria-hidden="true">_</span></h2>
```

## Applying it to a blog

The dashboard has no long-form text, so these are the natural extensions for
article pages, using the same tokens:

```css
.post { max-width: 72ch; margin: 0 auto; padding: 24px 16px; }
.post h1 { font-size: 24px; font-weight: 600; line-height: 1.3; margin: 0 0 8px; }
.post h2 { font-size: 18px; font-weight: 600; margin: 32px 0 8px; }
.post h3 { font-size: 15px; font-weight: 600; margin: 24px 0 6px; }
.post p, .post li { font-size: 14px; }

/* post date / tags line, styled like a .label */
.post-meta { font-size: 11px; color: var(--muted); text-transform: uppercase; letter-spacing: 1.2px; }

.post a { color: var(--accent); text-underline-offset: 3px; }
.post a:hover { color: var(--on-surface); }

.post pre {
	background: var(--surface);
	border: 1px solid var(--outline);
	border-radius: 8px;
	padding: 14px 16px;
	overflow-x: auto;
	font-size: 13px;
}
.post :not(pre) > code {
	background: var(--neutral-bg);
	color: var(--neutral-fg);
	padding: 1px 5px;
	border-radius: 3px;
	font-size: 0.95em;
}

.post blockquote {
	margin: 16px 0;
	padding: 4px 16px;
	border-left: 4px solid var(--accent);
	color: var(--muted);
}

.post hr { border: none; border-top: 1px dashed var(--outline); margin: 32px 0; }

.post table { border-collapse: collapse; width: 100%; font-size: 13px; }
.post th { text-align: left; color: var(--muted); font-weight: 600; }
.post th, .post td { padding: 6px 10px; border-bottom: 1px solid var(--outline); }
```

Tips for a blog:

- Keep body text at 14px only if your audience reads on desktop; for long
  articles 15px with the same 1.55 line height is easier.
- Use the status pairs (`--ok-*`, `--warn-*`, `--error-*`) for callout boxes
  ("note", "warning") instead of inventing new colours.
- Syntax highlighting: pick a low-saturation theme so code blocks don't break
  the matte look.
