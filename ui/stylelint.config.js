// Colors come only from the theme tokens (src/styles/theme.css): a color anywhere else would not
// follow the theme, the accent or a custom theme.
const colorFunctions = [
  'rgb',
  'rgba',
  'hsl',
  'hsla',
  'hwb',
  'lab',
  'lch',
  'oklab',
  'oklch',
  'color',
];

export default {
  rules: {
    'color-no-hex': true,
    'color-named': 'never',
    'function-disallowed-list': colorFunctions,
  },
  overrides: [
    { files: ['**/*.svelte'], customSyntax: 'postcss-html' },
    {
      files: ['src/styles/theme.css'],
      rules: { 'color-no-hex': null, 'color-named': null, 'function-disallowed-list': null },
    },
  ],
};
