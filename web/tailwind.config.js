/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    fontSize: {
      xs: ['0.8125rem', { lineHeight: '1.25rem' }],
      sm: ['0.9375rem', { lineHeight: '1.5rem' }],
      base: ['1.0625rem', { lineHeight: '1.75rem' }],
      lg: ['1.1875rem', { lineHeight: '1.75rem' }],
      xl: ['1.3125rem', { lineHeight: '2rem' }],
      '2xl': ['1.5rem', { lineHeight: '2rem' }],
      '3xl': ['1.875rem', { lineHeight: '2.25rem' }],
    },
    extend: {
      colors: {
        bg: {
          DEFAULT: 'rgb(var(--bg-body) / <alpha-value>)',
          card: 'rgb(var(--bg-card) / <alpha-value>)',
          hover: 'rgb(var(--bg-hover) / <alpha-value>)',
        },
        accent: {
          DEFAULT: 'rgb(var(--accent) / <alpha-value>)',
          dim: 'rgb(var(--accent-dim) / <alpha-value>)',
          light: 'rgb(var(--accent-light) / <alpha-value>)',
        },
        theme: {
          border: 'rgb(var(--theme-border) / <alpha-value>)',
        },
        ok: '#34d399',
        warn: '#fbbf24',
        err: '#f87171',
      },
      boxShadow: {
        glow: '0 0 20px rgb(var(--accent) / 0.15)',
        'glow-sm': '0 0 10px rgb(var(--accent) / 0.1)',
        soft: '0 4px 24px rgba(0, 0, 0, 0.3)',
      },
      backgroundImage: {
        'gradient-accent': 'linear-gradient(135deg, rgb(var(--accent)), rgb(var(--accent-light)))',
        'gradient-card': 'linear-gradient(145deg, #12122a, #1a1a3e)',
      },
    },
  },
  plugins: [],
}
