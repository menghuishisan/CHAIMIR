// eslint.config 文件定义前端工作区共享的 TypeScript/React 静态检查规则。

const js = require('@eslint/js')
const tsParser = require('@typescript-eslint/parser')
const tsPlugin = require('@typescript-eslint/eslint-plugin')
const globals = require('globals')
const reactPlugin = require('eslint-plugin-react')
const reactHooksPlugin = require('eslint-plugin-react-hooks')

module.exports = [
  {
    ignores: [
      '**/dist/**',
      '**/build/**',
      '**/node_modules/**',
      '**/*.config.js',
      '**/*.config.ts',
    ],
  },
  js.configs.recommended,
  {
    files: ['**/*.{js,cjs,mjs}'],
    languageOptions: {
      globals: {
        ...globals.browser,
        ...globals.node,
      },
    },
  },
  {
    files: ['**/*.{ts,tsx}'],
    languageOptions: {
      parser: tsParser,
      parserOptions: {
        ecmaVersion: 2020,
        sourceType: 'module',
        ecmaFeatures: {
          jsx: true,
        },
      },
      globals: {
        ...globals.browser,
        ...globals.node,
      },
    },
    plugins: {
      '@typescript-eslint': tsPlugin,
      react: reactPlugin,
      'react-hooks': reactHooksPlugin,
    },
    settings: {
      react: {
        version: '19.2',
      },
    },
    rules: {
      ...reactPlugin.configs.recommended.rules,
      ...tsPlugin.configs.recommended.rules,
      'no-undef': 'off',
      'react/react-in-jsx-scope': 'off',
      'react/prop-types': 'off',
      'react-hooks/rules-of-hooks': 'error',
      'react-hooks/exhaustive-deps': 'warn',
      '@typescript-eslint/no-explicit-any': 'warn',
      '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_' }],

      // FE-1:禁止裸 hex 颜色,颜色必须来自 CSS 变量令牌。
      'no-restricted-syntax': [
        'error',
        {
          selector: 'Literal[value=/#[0-9a-fA-F]{3,8}/]',
          message: 'FE-1 违规:禁止裸 hex 颜色,必须使用 CSS 变量令牌(如 var(--color-primary))',
        },
        {
          selector: 'TemplateElement[value.raw=/#[0-9a-fA-F]{3,8}/]',
          message: 'FE-1 违规:禁止裸 hex 颜色,必须使用 CSS 变量令牌',
        },

        // data-[state=...] 等字母开头的变体不会命中该规则。
        {
          selector: 'Literal[value=/-\\[(?:[0-9.#]|rgb|hsl|var\\()/]',
          message: 'FE-1 违规:禁止 Tailwind 任意值语法;尺寸和颜色必须先登记为主题令牌',
        },
        {
          selector: 'TemplateElement[value.raw=/-\\[(?:[0-9.#]|rgb|hsl|var\\()/]',
          message: 'FE-1 违规:禁止 Tailwind 任意值语法;先登记令牌再使用主题工具类',
        },
      ],
    },
  },
]
