module.exports = {
  root: true,
  parser: '@typescript-eslint/parser',
  parserOptions: {
    ecmaVersion: 2020,
    sourceType: 'module',
    ecmaFeatures: {
      jsx: true,
    },
  },
  settings: {
    react: {
      version: '18.3',
    },
  },
  extends: [
    'eslint:recommended',
    'plugin:react/recommended',
    'plugin:react-hooks/recommended',
    'plugin:@typescript-eslint/recommended',
  ],
  rules: {
    'react/react-in-jsx-scope': 'off',
    'react/prop-types': 'off',
    '@typescript-eslint/no-explicit-any': 'warn',
    '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_' }],

    // FE-1: 禁止裸 hex 颜色（通过正则检测字符串字面量中的 hex）
    'no-restricted-syntax': [
      'error',
      {
        selector: 'Literal[value=/#[0-9a-fA-F]{3,8}/]',
        message: 'FE-1 违规：禁止裸 hex 颜色，必须使用 CSS 变量令牌（如 var(--color-primary)）',
      },
      {
        selector: 'TemplateElement[value.raw=/#[0-9a-fA-F]{3,8}/]',
        message: 'FE-1 违规：禁止裸 hex 颜色，必须使用 CSS 变量令牌',
      },
    ],
  },
  ignorePatterns: ['dist', 'build', 'node_modules', '*.config.js', '*.config.ts'],
}
