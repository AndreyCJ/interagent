import skipFormatting from '@vue/eslint-config-prettier/skip-formatting'
import { defineConfigWithVueTs, vueTsConfigs } from '@vue/eslint-config-typescript'
import pluginVue from 'eslint-plugin-vue'

export default defineConfigWithVueTs(
  {
    name: 'app/files-to-lint',
    files: ['**/*.{ts,mts,tsx,vue}'],
  },
  {
    name: 'app/files-to-ignore',
    ignores: ['**/dist/**', '**/coverage/**', 'wailsjs', 'e2e/**/*.ts', 'src/vite-env.d.ts'],
  },

  pluginVue.configs['flat/recommended'],
  vueTsConfigs.recommended,
  skipFormatting,

  {
    rules: {
      'vue/block-order': [
        'error',
        {
          order: ['script', 'template', 'style'],
        },
      ],
      '@typescript-eslint/no-explicit-any': 'warn',
      '@typescript-eslint/no-unused-vars': [
        'error',
        { argsIgnorePattern: '^_', varsIgnorePattern: '^_' },
      ],
    },
  },
)

// import { vueTsConfigs, withVueTs } from '@vue/eslint-config-typescript'
// import pluginVue from 'eslint-plugin-vue'
// import globals from 'globals'

// export default withVueTs(
//   // Игноры — лучше ставить первым
//   {
//     ignores: [
//       '**/dist/**',
//       '**/node_modules/**',
//       '**/coverage/**',
//       '**/*.min.js',
//       'wailsjs',
//       'e2e/**/*.ts',
//     ],
//   },

//   // Vue recommended rules
//   pluginVue.configs['flat/recommended'],

//   // TypeScript recommended rules (works with .vue + .ts)
//   vueTsConfigs.recommended,

//   // Optional: stricter TypeScript rules
//   // vueTsConfigs.strict,

//   {
//     languageOptions: {
//       globals: {
//         ...globals.browser,
//         ...globals.node, // useful for Vite config, scripts, etc.
//       },
//     },
//     rules: {
//       // Vue
//       'vue/block-order': [
//         'error',
//         {
//           order: ['script', 'template', 'style'],
//         },
//       ],
//       'vue/multi-word-component-names': 'off',
//       'vue/html-self-closing': [
//         'error',
//         {
//           html: { void: 'always', normal: 'never', component: 'always' },
//           svg: 'always',
//           math: 'always',
//         },
//       ],
//       'vue/component-name-in-template-casing': ['error', 'PascalCase'],
//       'vue/define-macros-order': [
//         'error',
//         {
//           order: ['defineOptions', 'defineProps', 'defineEmits', 'defineSlots'],
//         },
//       ],

//       // TypeScript
//       '@typescript-eslint/no-explicit-any': 'warn',
//       '@typescript-eslint/no-unused-vars': [
//         'error',
//         { argsIgnorePattern: '^_', varsIgnorePattern: '^_' },
//       ],
//       '@typescript-eslint/consistent-type-imports': [
//         'error',
//         { prefer: 'type-imports', fixStyle: 'inline-type-imports' },
//       ],

//       // General
//       'no-console': ['warn', { allow: ['warn', 'error'] }],

//       // Temporary
//       '@typescript-eslint/no-unused-expressions': 'off',
//       '@typescript-eslint/no-empty-object-type': 'off',
//     },
//   },
// )
