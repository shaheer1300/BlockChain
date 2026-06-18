/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        // Educational colour system — each role has one hue.
        wallet: '#22d3ee',     // cyan-400
        utxo: '#10b981',       // emerald-500 (unspent)
        spent: '#f59e0b',      // amber-500 (in mempool, about to be consumed)
        confirmed: '#3b82f6',  // blue-500 (mined into block)
        rejected: '#ef4444',   // red-500 (double-spend / invalid)
        block: '#8b5cf6',      // violet-500 (block in chain)
      },
      fontFamily: {
        mono: ['"JetBrains Mono"', '"Fira Code"', 'ui-monospace', 'monospace'],
      },
      animation: {
        'pulse-glow': 'pulseGlow 2s ease-in-out infinite',
        'fade-in': 'fadeIn 250ms ease-out',
        'slide-in-right': 'slideInRight 250ms ease-out',
      },
      keyframes: {
        pulseGlow: {
          '0%,100%': { boxShadow: '0 0 0 0 rgba(245, 158, 11, 0.55)' },
          '50%': { boxShadow: '0 0 0 8px rgba(245, 158, 11, 0)' },
        },
        fadeIn: {
          '0%': { opacity: '0' },
          '100%': { opacity: '1' },
        },
        slideInRight: {
          '0%': { transform: 'translateX(100%)' },
          '100%': { transform: 'translateX(0)' },
        },
      },
    },
  },
  plugins: [],
}
