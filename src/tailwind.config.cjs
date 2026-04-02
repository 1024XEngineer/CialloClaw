/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ['./index.html', './src/**/*.{vue,ts,tsx}'],
  theme: {
    extend: {
      colors: {
        ink: '#1f2430',
        cream: '#fff7ea',
        peach: '#ffc89e',
        mint: '#b5f3d7',
        sky: '#8ac6ff',
        berry: '#ff9db2',
      },
      boxShadow: {
        pixel: '0 0 0 2px rgba(31,36,48,0.9), 6px 6px 0 rgba(31,36,48,0.22)',
      },
      borderRadius: {
        pixel: '18px',
      },
      fontFamily: {
        display: ['Microsoft YaHei', 'PingFang SC', 'sans-serif'],
      },
    },
  },
  plugins: [],
}
