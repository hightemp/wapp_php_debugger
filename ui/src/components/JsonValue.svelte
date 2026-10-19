<script lang="ts">
  let { value }: { value: unknown } = $props();

  const type = $derived(
    value === null
      ? 'null'
      : Array.isArray(value)
        ? 'array'
        : (typeof value as 'string' | 'number' | 'boolean' | 'object'),
  );
  // Strings are shown quoted so that the string "null" never looks like null
  const text = $derived(
    type === 'string'
      ? JSON.stringify(value)
      : type === 'array'
        ? '[]'
        : type === 'object'
          ? '{}'
          : String(value),
  );
</script>

<span class="value" data-type={type}>{text}</span>

<style>
  .value {
    overflow-wrap: anywhere;
    white-space: pre-wrap;
  }

  [data-type='string'] {
    color: var(--syntax-string);
  }

  [data-type='number'] {
    color: var(--syntax-number);
  }

  [data-type='boolean'] {
    color: var(--syntax-keyword);
  }

  [data-type='null'] {
    color: var(--syntax-null);
    font-style: italic;
  }
</style>
