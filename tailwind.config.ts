import type { Config } from 'tailwindcss';

const config: Config = {
  content: [
    './app/**/*.{js,ts,jsx,tsx,mdx}',
    './components/**/*.{js,ts,jsx,tsx,mdx}',
  ],
  theme: {
    extend: {
      colors: {
        white: '#ffffff',
        altBackground: '#f8fafc',
        textPrimary: '#111827',
        textSecondary: '#6b7280',
        accent: '#6366f1',
        cardSky: '#e0f2fe',
        cardSun: '#fef9c3',
        cardMint: '#dcfce7',
        cardLavender: '#f3e8ff',
      },
    },
  },
  plugins: [],
};

export default config;
