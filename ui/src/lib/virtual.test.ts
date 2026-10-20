import { describe, expect, it } from 'vitest';
import { visibleRange } from './virtual';

describe('visibleRange', () => {
  it('covers the viewport plus the overscan on both sides', () => {
    // rows of 20 px, the list scrolled by 1000 px, a viewport of 400 px: rows 50..70 are seen
    expect(
      visibleRange({ scrollTop: 1000, height: 400, rowHeight: 20, count: 10000, overscan: 5 }),
    ).toEqual({
      start: 45,
      end: 75,
    });
  });

  it('stays within the list', () => {
    expect(
      visibleRange({ scrollTop: 0, height: 400, rowHeight: 20, count: 10, overscan: 5 }),
    ).toEqual({
      start: 0,
      end: 10,
    });
    expect(
      visibleRange({ scrollTop: 5000, height: 400, rowHeight: 20, count: 100, overscan: 5 }),
    ).toEqual({
      start: 95,
      end: 100,
    });
  });

  it('starts at the top while the list is below the viewport top', () => {
    // the list begins 300 px further down the scrolled pane
    expect(
      visibleRange({ scrollTop: -300, height: 400, rowHeight: 20, count: 1000, overscan: 0 }),
    ).toEqual({
      start: 0,
      end: 5,
    });
  });
});
