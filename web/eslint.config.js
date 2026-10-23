// @ts-check
import js from '@eslint/js';
import prettier from 'eslint-config-prettier';
import i18next from 'eslint-plugin-i18next';
import jsxA11y from 'eslint-plugin-jsx-a11y';
import react from 'eslint-plugin-react';
import reactHooks from 'eslint-plugin-react-hooks';
import { defineConfig, globalIgnores } from 'eslint/config';
import globals from 'globals';
import tseslint from 'typescript-eslint';

// Attributes that carry user-visible text. JSX text is always checked; any other attribute
// (className, variant, to, ...) may hold literal strings.
const textAttributes = [
  'alt',
  'aria-label',
  'aria-description',
  'aria-placeholder',
  'aria-roledescription',
  'aria-valuetext',
  'caption',
  'label',
  'placeholder',
  'title',
];

export default defineConfig([
  globalIgnores(['dist', 'dev-dist', 'coverage', 'storybook-static']),
  {
    files: ['**/*.{ts,tsx}'],
    extends: [
      js.configs.recommended,
      tseslint.configs.recommendedTypeChecked,
      react.configs.flat.recommended,
      react.configs.flat['jsx-runtime'],
      reactHooks.configs.flat.recommended,
      jsxA11y.flatConfigs.recommended,
    ],
    languageOptions: {
      parserOptions: {
        project: ['./tsconfig.json', './tsconfig.node.json'],
        tsconfigRootDir: import.meta.dirname,
      },
      globals: globals.browser,
    },
    settings: { react: { version: 'detect' } },
    rules: {
      // react-aria components manage autoFocus accessibly (dialogs, the first field of a form page).
      'jsx-a11y/no-autofocus': ['error', { ignoreNonDOM: true }],
      // TanStack Router guards throw redirect() results by design.
      '@typescript-eslint/only-throw-error': [
        'error',
        { allow: [{ from: 'package', package: '@tanstack/router-core', name: 'Redirect' }] },
      ],
    },
  },
  {
    files: ['src/**/*.tsx'],
    // Tests and stories hold sample data and developer labels, not UI text of the app.
    ignores: ['src/**/*.test.tsx', 'src/**/*.stories.tsx'],
    plugins: { i18next },
    rules: {
      'i18next/no-literal-string': [
        'error',
        {
          mode: 'jsx-only',
          'jsx-attributes': { include: textAttributes },
          message: 'UI text belongs in src/locales/{de,en}.json',
        },
      ],
    },
  },
  {
    files: ['src/features/**/*.{ts,tsx}'],
    rules: {
      'no-restricted-imports': [
        'error',
        {
          patterns: [
            {
              regex: '^@/ui/(?!aria$).+|^(\\.\\./)+ui(/.*)?$',
              message:
                "Features import shared UI only from '@/ui' (react-aria parts from '@/ui/aria').",
            },
          ],
          paths: [
            {
              name: 'react-aria-components',
              importNames: ['Form'],
              message:
                "Import Form from '@/ui/aria': it lets a server's field message go when the reader corrects that field.",
            },
          ],
        },
      ],
    },
  },
  {
    files: ['*.config.{js,ts}'],
    languageOptions: { globals: globals.node },
  },
  {
    files: ['*.config.js'],
    extends: [tseslint.configs.disableTypeChecked],
  },
  prettier,
]);
