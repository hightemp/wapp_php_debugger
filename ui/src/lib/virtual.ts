// Virtual lists render only the rows near the viewport: thousands of requests or messages stay fast.

export interface Viewport {
  /** How far the top of the list is scrolled past the top of the viewport; negative while below it. */
  scrollTop: number;
  height: number;
  rowHeight: number;
  count: number;
  /** Rows rendered beyond the viewport on each side, so fast scrolling shows no gaps. */
  overscan: number;
}

/** The rows [start, end) to render. */
export function visibleRange(view: Viewport): { start: number; end: number } {
  // Scrolled past the end (the list just got shorter): keep its last rows
  const first = Math.min(view.count, Math.floor(Math.max(0, view.scrollTop) / view.rowHeight));
  const last = Math.ceil((view.scrollTop + view.height) / view.rowHeight);
  return {
    start: Math.max(0, first - view.overscan),
    end: Math.max(0, Math.min(view.count, last + view.overscan)),
  };
}
