// 该文件定义共享 ESLint 基础配置。
module.exports = {
  root: false,
  env: {
    browser: true,
    es2022: true,
    node: true,
  },
  parserOptions: {
    ecmaVersion: "latest",
    sourceType: "module",
  },
};
