import js from '@eslint/js';
import svelte from 'eslint-plugin-svelte';
import globals from 'globals';
import ts from 'typescript-eslint';
import svelteConfig from './svelte.config.js';

// Венгерская нотация запрещена (п. 4.4 плана): aItems, sName, fnReload и т. п.
const hungarian = { regex: '^(a|s|i|b|o|m|f|fn)[A-Z]', match: false };

export default ts.config(
  js.configs.recommended,
  ...ts.configs.strict,
  ...svelte.configs.recommended,
  {
    languageOptions: { globals: { ...globals.browser, ...globals.node } },
  },
  {
    files: ['**/*.svelte', '**/*.svelte.ts'],
    languageOptions: {
      parserOptions: {
        projectService: true,
        extraFileExtensions: ['.svelte'],
        parser: ts.parser,
        svelteConfig,
      },
    },
  },
  {
    rules: {
      '@typescript-eslint/naming-convention': [
        'error',
        {
          selector: ['variable', 'function', 'parameter', 'method', 'accessor'],
          format: null,
          custom: hungarian,
        },
        // Поля DTO из OpenAPI остаются snake_case (п. 10.17), поэтому для свойств формат не навязывается
        { selector: ['property', 'classProperty'], format: null, custom: hungarian },
      ],
    },
  },
  { ignores: ['node_modules/', '../server/web/dist/'] },
);
