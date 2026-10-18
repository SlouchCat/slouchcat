/** @type {import('stylelint').Config} */
export default {
  extends: ['stylelint-config-standard'],
  plugins: ['stylelint-declaration-strict-value'],
  rules: {
    // Colours come only from design tokens (CLAUDE.md rule 9).
    'color-no-hex': true,
    'color-named': 'never',
    'scale-unlimited/declaration-strict-value': [
      [
        'color',
        'background-color',
        'border-color',
        'fill',
        'stroke',
        'outline-color',
        'box-shadow',
      ],
      {
        ignoreValues: ['currentcolor', 'transparent', 'inherit', 'initial', 'unset', 'none'],
        // Only var(--sc-...) is allowed; rgb(), hsl() and friends are rejected as well.
        ignoreFunctions: false,
      },
    ],
    // CSS Modules are accessed as styles.camelCase.
    'selector-class-pattern': [
      '^[a-z][a-zA-Z0-9]*$',
      { message: 'Expected class selector to be camelCase (CSS Modules)' },
    ],
    'selector-pseudo-class-no-unknown': [true, { ignorePseudoClasses: ['global', 'local'] }],
  },
};
