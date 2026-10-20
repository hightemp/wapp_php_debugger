import { describe, expect, it } from 'vitest';
import { diffValues } from './diff';

describe('diffValues', () => {
  it('lists what was added, removed and changed, by path', () => {
    const before = { user: { id: 7, cart: [1, 2] }, step: 'cart', token: 'a' };
    const after = { user: { id: 7, cart: [1, 2, 3] }, step: 'pay', coupon: null };
    expect(diffValues(before, after)).toEqual([
      { path: "['coupon']", kind: 'added', after: 'null' },
      { path: "['step']", kind: 'changed', before: '"cart"', after: '"pay"' },
      { path: "['token']", kind: 'removed', before: '"a"' },
      { path: "['user']['cart'][2]", kind: 'added', after: '3' },
    ]);
  });

  it('has nothing to say about equal values', () => {
    expect(diffValues({ a: [1, { b: 2 }] }, { a: [1, { b: 2 }] })).toEqual([]);
  });

  it('compares a value with nothing', () => {
    expect(diffValues(undefined, { a: 1 })).toEqual([{ path: "['a']", kind: 'added', after: '1' }]);
    expect(diffValues({ a: 1 }, undefined)).toEqual([
      { path: "['a']", kind: 'removed', before: '1' },
    ]);
  });

  it('tells a changed type: an array that became an object', () => {
    expect(diffValues({ a: [] }, { a: {} })).toEqual([
      { path: "['a']", kind: 'changed', before: '[]', after: '{}' },
    ]);
  });
});
