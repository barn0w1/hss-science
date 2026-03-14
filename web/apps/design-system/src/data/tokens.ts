export type Page = 'overview' | 'colors' | 'typography' | 'shape' | 'motion'

export interface PrimitiveColorToken {
  variable: string
  value: string
  group: string
  shade: number
  textOnColor: 'light' | 'dark'
}

export interface SemanticColorToken {
  variable: string
  label: string
  value: string
  textOnColor: 'light' | 'dark'
  pairedWith?: string
}

export interface SemanticColorGroup {
  title: string
  description: string
  tokens: SemanticColorToken[]
}

export interface ShapeToken {
  variable: string
  value: string
  label: string
  description: string
}

export interface TypescaleToken {
  variable: string
  value: string
  label: string
  group: 'Display' | 'Headline' | 'Title' | 'Body' | 'Label'
  size: 'Large' | 'Medium' | 'Small'
}

export interface MotionDurationToken {
  variable: string
  value: string
  ms: number
  label: string
}

export interface MotionEasingToken {
  variable: string
  value: string
  label: string
  description: string
}

// ─── Primitive Colors ────────────────────────────────────────────────────────

export const primitiveColors: PrimitiveColorToken[] = [
  { variable: '--palette-blue-10',  value: '#001b3d', group: 'Blue',    shade: 10,  textOnColor: 'light' },
  { variable: '--palette-blue-40',  value: '#2563eb', group: 'Blue',    shade: 40,  textOnColor: 'light' },
  { variable: '--palette-blue-80',  value: '#aac7ff', group: 'Blue',    shade: 80,  textOnColor: 'dark'  },
  { variable: '--palette-blue-95',  value: '#dce8ff', group: 'Blue',    shade: 95,  textOnColor: 'dark'  },
  { variable: '--palette-blue-99',  value: '#f8f9ff', group: 'Blue',    shade: 99,  textOnColor: 'dark'  },

  { variable: '--palette-neutral-10', value: '#1a1c1e', group: 'Neutral', shade: 10, textOnColor: 'light' },
  { variable: '--palette-neutral-40', value: '#5f6368', group: 'Neutral', shade: 40, textOnColor: 'light' },
  { variable: '--palette-neutral-90', value: '#e3e3e3', group: 'Neutral', shade: 90, textOnColor: 'dark'  },
  { variable: '--palette-neutral-95', value: '#f1f1f1', group: 'Neutral', shade: 95, textOnColor: 'dark'  },
  { variable: '--palette-neutral-99', value: '#fafafa', group: 'Neutral', shade: 99, textOnColor: 'dark'  },

  { variable: '--palette-red-40',  value: '#ba1a1a', group: 'Red',   shade: 40, textOnColor: 'light' },
  { variable: '--palette-red-90',  value: '#ffdad6', group: 'Red',   shade: 90, textOnColor: 'dark'  },

  { variable: '--palette-green-40', value: '#146c2e', group: 'Green', shade: 40, textOnColor: 'light' },
  { variable: '--palette-green-90', value: '#b7f2c6', group: 'Green', shade: 90, textOnColor: 'dark'  },
]

// ─── Semantic Colors ─────────────────────────────────────────────────────────

export const semanticColorGroups: SemanticColorGroup[] = [
  {
    title: 'Primary',
    description: 'The brand color used for interactive elements and emphasis.',
    tokens: [
      { variable: '--color-primary',               label: 'Primary',               value: '#2563eb', textOnColor: 'light' },
      { variable: '--color-on-primary',            label: 'On Primary',            value: '#ffffff', textOnColor: 'dark',  pairedWith: '--color-primary' },
      { variable: '--color-primary-container',     label: 'Primary Container',     value: '#dce8ff', textOnColor: 'dark'  },
      { variable: '--color-on-primary-container',  label: 'On Primary Container',  value: '#001b3d', textOnColor: 'light', pairedWith: '--color-primary-container' },
    ],
  },
  {
    title: 'Surface',
    description: 'Background layers for cards, sheets, and page backgrounds.',
    tokens: [
      { variable: '--color-surface',          label: 'Surface',          value: '#fafafa', textOnColor: 'dark'  },
      { variable: '--color-surface-variant',  label: 'Surface Variant',  value: '#f1f1f1', textOnColor: 'dark'  },
      { variable: '--color-on-surface',       label: 'On Surface',       value: '#1a1c1e', textOnColor: 'light', pairedWith: '--color-surface' },
      { variable: '--color-on-surface-muted', label: 'On Surface Muted', value: '#5f6368', textOnColor: 'light', pairedWith: '--color-surface' },
    ],
  },
  {
    title: 'Outline',
    description: 'Used for borders, dividers, and decorative lines.',
    tokens: [
      { variable: '--color-outline',         label: 'Outline',         value: '#5f6368', textOnColor: 'light' },
      { variable: '--color-outline-variant', label: 'Outline Variant', value: '#e3e3e3', textOnColor: 'dark'  },
    ],
  },
  {
    title: 'Error',
    description: 'Status color for destructive actions and error states.',
    tokens: [
      { variable: '--color-error',           label: 'Error',           value: '#ba1a1a', textOnColor: 'light' },
      { variable: '--color-error-container', label: 'Error Container', value: '#ffdad6', textOnColor: 'dark'  },
    ],
  },
  {
    title: 'Success',
    description: 'Status color for confirmations and positive states.',
    tokens: [
      { variable: '--color-success',           label: 'Success',           value: '#146c2e', textOnColor: 'light' },
      { variable: '--color-success-container', label: 'Success Container', value: '#b7f2c6', textOnColor: 'dark'  },
    ],
  },
]

// ─── Shape ───────────────────────────────────────────────────────────────────

export const shapeTokens: ShapeToken[] = [
  { variable: '--shape-none', value: '0px',    label: 'None', description: 'No rounding — square corners' },
  { variable: '--shape-xs',   value: '4px',    label: 'XS',   description: 'Subtle rounding for dense UI' },
  { variable: '--shape-sm',   value: '8px',    label: 'SM',   description: 'Gentle rounding for chips' },
  { variable: '--shape-md',   value: '12px',   label: 'MD',   description: 'Default for cards and inputs' },
  { variable: '--shape-lg',   value: '16px',   label: 'LG',   description: 'Pronounced rounding for dialogs' },
  { variable: '--shape-xl',   value: '28px',   label: 'XL',   description: 'Large surfaces like drawers' },
  { variable: '--shape-2xl',  value: '32px',   label: '2XL',  description: 'Very large rounding for FABs' },
  { variable: '--shape-full', value: '9999px', label: 'Full', description: 'Pill shape — fully rounded' },
]

// ─── Typography ──────────────────────────────────────────────────────────────

export const typescaleTokens: TypescaleToken[] = [
  { variable: '--typescale-display-large',   value: '57px', label: 'Display Large',   group: 'Display',  size: 'Large'  },
  { variable: '--typescale-display-medium',  value: '45px', label: 'Display Medium',  group: 'Display',  size: 'Medium' },
  { variable: '--typescale-display-small',   value: '36px', label: 'Display Small',   group: 'Display',  size: 'Small'  },

  { variable: '--typescale-headline-large',  value: '32px', label: 'Headline Large',  group: 'Headline', size: 'Large'  },
  { variable: '--typescale-headline-medium', value: '28px', label: 'Headline Medium', group: 'Headline', size: 'Medium' },
  { variable: '--typescale-headline-small',  value: '24px', label: 'Headline Small',  group: 'Headline', size: 'Small'  },

  { variable: '--typescale-title-large',     value: '22px', label: 'Title Large',     group: 'Title',    size: 'Large'  },
  { variable: '--typescale-title-medium',    value: '16px', label: 'Title Medium',    group: 'Title',    size: 'Medium' },
  { variable: '--typescale-title-small',     value: '14px', label: 'Title Small',     group: 'Title',    size: 'Small'  },

  { variable: '--typescale-body-large',      value: '16px', label: 'Body Large',      group: 'Body',     size: 'Large'  },
  { variable: '--typescale-body-medium',     value: '14px', label: 'Body Medium',     group: 'Body',     size: 'Medium' },
  { variable: '--typescale-body-small',      value: '12px', label: 'Body Small',      group: 'Body',     size: 'Small'  },

  { variable: '--typescale-label-large',     value: '14px', label: 'Label Large',     group: 'Label',    size: 'Large'  },
  { variable: '--typescale-label-medium',    value: '12px', label: 'Label Medium',    group: 'Label',    size: 'Medium' },
  { variable: '--typescale-label-small',     value: '11px', label: 'Label Small',     group: 'Label',    size: 'Small'  },
]

// ─── Motion ──────────────────────────────────────────────────────────────────

export const motionDurationTokens: MotionDurationToken[] = [
  { variable: '--motion-duration-short1',  value: '50ms',  ms: 50,  label: 'Short 1'  },
  { variable: '--motion-duration-short2',  value: '100ms', ms: 100, label: 'Short 2'  },
  { variable: '--motion-duration-medium1', value: '200ms', ms: 200, label: 'Medium 1' },
  { variable: '--motion-duration-medium2', value: '300ms', ms: 300, label: 'Medium 2' },
  { variable: '--motion-duration-long1',   value: '450ms', ms: 450, label: 'Long 1'   },
  { variable: '--motion-duration-long2',   value: '600ms', ms: 600, label: 'Long 2'   },
]

export const motionEasingTokens: MotionEasingToken[] = [
  {
    variable: '--motion-easing-standard',
    value: 'cubic-bezier(0.2, 0, 0, 1)',
    label: 'Standard',
    description: 'Default easing for most transitions',
  },
  {
    variable: '--motion-easing-standard-decel',
    value: 'cubic-bezier(0, 0, 0, 1)',
    label: 'Standard Decel',
    description: 'Elements entering the screen',
  },
  {
    variable: '--motion-easing-standard-accel',
    value: 'cubic-bezier(0.3, 0, 1, 1)',
    label: 'Standard Accel',
    description: 'Elements leaving the screen',
  },
  {
    variable: '--motion-easing-emphasized',
    value: 'cubic-bezier(0.2, 0, 0, 1)',
    label: 'Emphasized',
    description: 'High-attention transitions and hero elements',
  },
  {
    variable: '--motion-easing-spring',
    value: 'cubic-bezier(0.175, 0.885, 0.32, 1.275)',
    label: 'Spring',
    description: 'Bouncy feel for playful interactions',
  },
]
