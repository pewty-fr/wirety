/** @type {import('tailwindcss').Config} */
export default {
  darkMode: 'class',
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        primary: {
          50: '#fff7ed',
          100: '#ffedd5',
          200: '#fed7aa',
          300: '#fdba74',
          400: '#fb923c',
          500: '#FF7F2A', // main branding orange
          600: '#ea580c',
          700: '#c2410c',
          800: '#9a3412',
          900: '#7c2d12',
        },
        accent: {
          blue: '#1A237E', // deep blue
          yellow: '#FFD600',
          green: '#43A047',
          amber: '#FFB300',
          red: '#E53935',
        },
        gray: {
          50: '#f9fafb',
          100: '#f3f4f6',
          200: '#e5e7eb',
          300: '#d1d5db',
          400: '#9ca3af',
          500: '#6b7280',
          600: '#4b5563',
          700: '#374151',
          800: '#1f2937',
          900: '#111827',
        },
        white: '#fff',
        dark: '#181A20',
      },
      backgroundImage: {
        'gradient-orange': 'linear-gradient(90deg, #FF7F2A 0%, #fb923c 100%)',
        'gradient-blue': 'linear-gradient(90deg, #1A237E 0%, #43A047 100%)',
        'gradient-dark': 'linear-gradient(90deg, #181A20 0%, #374151 100%)',
      },
    },
  },
  plugins: [],
}

