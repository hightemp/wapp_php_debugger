import { describe, expect, it } from 'vitest';
import { highlight } from './highlight';

describe('highlight', () => {
  it('colours PHP and escapes everything', () => {
    const html = highlight('<?php\necho "<b>hi</b>";', 'a.php');
    expect(html).toContain('hljs-');
    expect(html).not.toContain('<b>');
    expect(html).toContain('&lt;b&gt;');
  });

  it('reads a fragment from the middle of a PHP file as code', () => {
    const html = highlight('class Item {}', 'Item.php');
    expect(html).toContain('<span class="hljs-keyword">class</span>');
    expect(html).not.toContain('&lt;?php');
  });

  it('escapes files of unknown languages', () => {
    expect(highlight('<script>', 'notes.txt')).toBe('&#60;script&#62;');
  });
});
