/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,js}'],
  theme: {
    extend: {
      colors: {
        pfsense: {
          nav: '#1e3a5f',
          bar: '#2c5282',
          accent: '#3182ce',
        },
      },
    },
  },
  plugins: [],
}
