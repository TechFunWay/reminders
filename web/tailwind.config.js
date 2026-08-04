/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{vue,js,ts,jsx,tsx}",
  ],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        // Brand palette is tokenized so another theme/bundle can ship a
        // different brand by overriding --color-brand-* in CSS. The default
        // values live in src/styles/themes.css as RGB triplets, which lets
        // Tailwind's `<alpha-value>` opacity modifier keep working.
        brand: {
          50:  'rgb(var(--color-brand-50)  / <alpha-value>)',
          100: 'rgb(var(--color-brand-100) / <alpha-value>)',
          200: 'rgb(var(--color-brand-200) / <alpha-value>)',
          300: 'rgb(var(--color-brand-300) / <alpha-value>)',
          400: 'rgb(var(--color-brand-400) / <alpha-value>)',
          500: 'rgb(var(--color-brand-500) / <alpha-value>)',
          600: 'rgb(var(--color-brand-600) / <alpha-value>)',
          700: 'rgb(var(--color-brand-700) / <alpha-value>)',
          800: 'rgb(var(--color-brand-800) / <alpha-value>)',
          900: 'rgb(var(--color-brand-900) / <alpha-value>)',
        },
        // Semantic tokens — the base set downstream apps build on. Each
        // resolves to an RGB triplet CSS variable so themes can swap them
        // without touching component code.
        background:           'rgb(var(--color-background)          / <alpha-value>)',
        foreground:           'rgb(var(--color-foreground)          / <alpha-value>)',
        surface:              'rgb(var(--color-surface)             / <alpha-value>)',
        'surface-foreground': 'rgb(var(--color-surface-foreground)  / <alpha-value>)',
        muted:                'rgb(var(--color-muted)               / <alpha-value>)',
        'muted-foreground':   'rgb(var(--color-muted-foreground)    / <alpha-value>)',
        border:               'rgb(var(--color-border)              / <alpha-value>)',
        input:                'rgb(var(--color-input)                / <alpha-value>)',
        ring:                 'rgb(var(--color-ring)                / <alpha-value>)',
        'primary':            'rgb(var(--color-brand-500)           / <alpha-value>)',
        'primary-foreground': 'rgb(255 255 255                      / <alpha-value>)',
        accent:               'rgb(var(--color-accent)              / <alpha-value>)',
        destructive:          'rgb(var(--color-destructive)         / <alpha-value>)',
      },
      fontFamily: {
        sans: ['-apple-system', 'BlinkMacSystemFont', 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', 'sans-serif'],
        display: ['-apple-system', 'BlinkMacSystemFont', 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', 'sans-serif'],
      },
      boxShadow: {
        soft: '0 2px 8px -2px rgba(16, 24, 40, 0.06), 0 4px 16px -4px rgba(16, 24, 40, 0.08)',
        card: '0 1px 3px rgba(16, 24, 40, 0.04), 0 12px 32px -8px rgba(16, 24, 40, 0.08)',
        glow: '0 0 0 1px rgb(var(--color-brand-500) / 0.1), 0 8px 24px -6px rgb(var(--color-brand-500) / 0.35)',
      },
      backgroundImage: {
        'brand-gradient':
          'linear-gradient(135deg, rgb(var(--color-brand-500)) 0%, rgb(var(--color-brand-400)) 50%, rgb(var(--color-brand-300)) 100%)',
      },
      keyframes: {
        'fade-in': {
          '0%': { opacity: '0', transform: 'translateY(8px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' },
        },
        'scale-in': {
          '0%': { opacity: '0', transform: 'scale(0.96)' },
          '100%': { opacity: '1', transform: 'scale(1)' },
        },
        shimmer: {
          '100%': { transform: 'translateX(100%)' },
        },
      },
      animation: {
        'fade-in': 'fade-in 0.4s cubic-bezier(0.16, 1, 0.3, 1)',
        'scale-in': 'scale-in 0.2s cubic-bezier(0.16, 1, 0.3, 1)',
      },
    },
  },
  plugins: [],
}
