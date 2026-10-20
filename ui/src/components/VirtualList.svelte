<script lang="ts" generics="T">
  // A list that renders only the rows near the viewport of its scrolling ancestor. Rows must be of
  // one height: it is measured on the first rendered row. Without layout (tests) the first rows show.
  import type { Snippet } from 'svelte';
  import { visibleRange } from '../lib/virtual';

  let {
    items,
    key,
    row,
    label,
    overscan = 15,
    initial = 100,
    class: className = '',
  }: {
    items: readonly T[];
    key: (item: T) => string | number;
    /** Renders the <li> of an item; index is its place in the whole list. */
    row: Snippet<[T, number]>;
    label?: string;
    overscan?: number;
    initial?: number;
    class?: string;
  } = $props();

  let list = $state<HTMLElement>();
  let rowHeight = $state(0);
  let scrollTop = $state(0);
  let height = $state(0);

  function scrollParent(element: HTMLElement): HTMLElement | undefined {
    for (let node = element.parentElement; node; node = node.parentElement) {
      const overflow = getComputedStyle(node).overflowY;
      if (overflow === 'auto' || overflow === 'scroll') {
        return node;
      }
    }
    return undefined;
  }

  $effect(() => {
    const element = list;
    if (!element) {
      return;
    }
    const scroller = scrollParent(element);
    if (!scroller) {
      return;
    }
    const update = () => {
      // Where the list starts inside the scrolled content
      const offset =
        element.getBoundingClientRect().top -
        scroller.getBoundingClientRect().top +
        scroller.scrollTop;
      scrollTop = scroller.scrollTop - offset;
      height = scroller.clientHeight;
    };
    update();
    scroller.addEventListener('scroll', update, { passive: true });
    const resize = new ResizeObserver(update);
    resize.observe(scroller);
    return () => {
      scroller.removeEventListener('scroll', update);
      resize.disconnect();
    };
  });

  // The height of a row, from the first one rendered
  $effect(() => {
    void items.length;
    const first = list?.querySelector(':scope > ul > li');
    if (first instanceof HTMLElement && first.offsetHeight > 0) {
      rowHeight = first.offsetHeight;
    }
  });

  const virtual = $derived(rowHeight > 0 && height > 0);
  const range = $derived(
    virtual
      ? visibleRange({ scrollTop, height, rowHeight, count: items.length, overscan })
      : { start: 0, end: Math.min(items.length, initial) },
  );
  const visible = $derived(items.slice(range.start, range.end));
</script>

<div
  bind:this={list}
  class="virtual"
  style:height={virtual ? `${items.length * rowHeight}px` : undefined}
>
  <ul
    class={className}
    aria-label={label}
    style:transform={virtual ? `translateY(${range.start * rowHeight}px)` : undefined}
  >
    {#each visible as item, index (key(item))}
      {@render row(item, range.start + index)}
    {/each}
  </ul>
</div>

<style>
  .virtual {
    position: relative;
  }

  ul {
    margin: 0;
    padding: 0;
    list-style: none;
  }
</style>
