import js from '@eslint/js'
import pluginVue from 'eslint-plugin-vue'
import prettier from '@vue/eslint-config-prettier'
import globals from 'globals'

export default [
  // 全局忽略
  {
    ignores: ['dist/**', 'node_modules/**', '*.config.js', '*.config.cjs', 'src/auto-imports.d.ts', 'src/components.d.ts'],
  },
  // 基础推荐规则
  js.configs.recommended,
  // Vue 3 推荐规则
  ...pluginVue.configs['flat/recommended'],
  // Prettier 兼容（关闭冲突规则）
  prettier,
  // 自定义规则 + 浏览器全局变量
  {
    languageOptions: {
      globals: {
        ...globals.browser,
        ...globals.es2021,
      },
    },
    rules: {
      // 关闭多词组件名限制
      'vue/multi-word-component-names': 'off',
      // 允许单标签自闭合
      'vue/html-self-closing': [
        'warn',
        {
          html: { void: 'always', normal: 'never', component: 'always' },
        },
      ],
      // 未使用变量警告（非错误），忽略 _ 前缀
      'no-unused-vars': ['warn', { argsIgnorePattern: '^_' }],
      // 允许空 catch 块（常见的静默错误处理模式）
      'no-empty': ['error', { allowEmptyCatch: true }],
      // console 允许
      'no-console': 'off',
      // Vue script 缩进（与 Prettier 一致）
      'vue/script-indent': 'off',
      // 允许 v-html
      'vue/no-v-html': 'off',
    },
  },
]
