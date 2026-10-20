// Syntax highlighting of the code preview: highlight.js core with the languages a PHP project shows,
// bundled with the UI (no CDN). The output escapes the code, so it is safe to put into the page.
import hljs from 'highlight.js/lib/core';
import javascript from 'highlight.js/lib/languages/javascript';
import json from 'highlight.js/lib/languages/json';
import php from 'highlight.js/lib/languages/php';
import sql from 'highlight.js/lib/languages/sql';
import xml from 'highlight.js/lib/languages/xml';

hljs.registerLanguage('php', php);
hljs.registerLanguage('javascript', javascript);
hljs.registerLanguage('json', json);
hljs.registerLanguage('sql', sql);
hljs.registerLanguage('xml', xml);

const byExtension: Record<string, string> = {
  php: 'php',
  phtml: 'php',
  inc: 'php',
  js: 'javascript',
  json: 'json',
  sql: 'sql',
  html: 'xml',
  xml: 'xml',
  tpl: 'xml',
};

/** Escapes text that no grammar knows, so it can share the code path of highlighted HTML. */
function escape(text: string): string {
  return text.replace(/[&<>"']/g, (char) => `&#${char.charCodeAt(0)};`);
}

/** HTML of the code: highlighted by the language of the file name, else escaped plain text. */
export function highlight(code: string, file: string): string {
  const language = byExtension[file.split('.').pop()?.toLowerCase() ?? ''];
  if (!language) {
    return escape(code);
  }
  // A fragment starting mid-file: PHP needs its opening tag to be read as code
  const lonePhp = language === 'php' && !code.trimStart().startsWith('<?');
  const html = hljs.highlight(lonePhp ? `<?php\n${code}` : code, {
    language,
    ignoreIllegals: true,
  }).value;
  return lonePhp ? html.replace(/^<span class="hljs-meta">&lt;\?php<\/span>\n/, '') : html;
}
