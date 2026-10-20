import { describe, expect, it } from 'vitest';
import { copyText, matchingPaths, pathKey, phpPath } from './jsonPath';

const value = {
  item_id: 42,
  user: { id: 7, name: 'Test', api_key: '***' },
  rows: [{ price: 9.5 }, 'second'],
};

describe('phpPath', () => {
  it('writes the path the PHP way', () => {
    expect(phpPath(['user', 'id'], [false, false])).toBe("['user']['id']");
    expect(phpPath(['rows', '0', 'price'], [false, true, false])).toBe("['rows'][0]['price']");
    expect(phpPath(["it's"], [false])).toBe("['it\\'s']");
  });
});

describe('matchingPaths', () => {
  it('keeps matching nodes and the way to them', () => {
    const paths = matchingPaths(value, 'API');
    expect([...paths].sort()).toEqual([pathKey(['user']), pathKey(['user', 'api_key'])].sort());
  });

  it('matches values too, numbers included', () => {
    expect(matchingPaths(value, '9.5').has(pathKey(['rows', '0', 'price']))).toBe(true);
    expect(matchingPaths(value, 'second').has(pathKey(['rows', '1']))).toBe(true);
  });

  it('is empty when nothing matches', () => {
    expect(matchingPaths(value, 'nothing').size).toBe(0);
  });
});

describe('copyText', () => {
  it('copies strings as they are and the rest as JSON', () => {
    expect(copyText('Test')).toBe('Test');
    expect(copyText(42)).toBe('42');
    expect(copyText(null)).toBe('null');
    expect(copyText({ id: 7 })).toBe('{\n  "id": 7\n}');
  });
});
