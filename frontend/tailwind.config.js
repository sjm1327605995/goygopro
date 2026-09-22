/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './*.html', './src/**/*.{ts,tsx}', './smoke/**/*.{js,jsx}'],
  corePlugins: {
    // gfw 复古皮肤（css/gframe-window.css、style.css）依赖浏览器默认样式
    // （body 背景、列表/按钮默认外观等），preflight 会打乱全部旧界面，
    // 试点阶段只引工具类，不开 reset。
    preflight: false,
  },
  theme: {
    extend: {},
  },
  plugins: [],
};
