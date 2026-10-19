<?php

declare(strict_types=1);

namespace WappDebugger;

/**
 * Turns any PHP value into JSON-safe data for the context of an event: limits depth, items
 * and string length, shows objects with all their properties, cuts recursion and masks secrets.
 */
final class Serializer
{
    private Masker $masker;
    private int $maxDepth;
    private int $maxItems;
    private int $maxStringBytes;
    /** @var array<int, true> objects on the current path, to detect recursion */
    private array $path = [];

    public function __construct(Masker $masker, int $maxDepth = 6, int $maxItems = 100, int $maxStringBytes = 2000)
    {
        $this->masker = $masker;
        $this->maxDepth = $maxDepth;
        $this->maxItems = $maxItems;
        $this->maxStringBytes = $maxStringBytes;
    }

    /**
     * @param mixed $value
     *
     * @return mixed
     */
    public function normalize($value)
    {
        $this->path = [];

        return $this->value($value, 1);
    }

    /**
     * @param mixed $value
     *
     * @return mixed
     */
    private function value($value, int $depth)
    {
        if ($value === null || is_bool($value) || is_int($value)) {
            return $value;
        }
        if (is_float($value)) {
            return is_finite($value) ? $value : (is_nan($value) ? 'NAN' : ($value > 0 ? 'INF' : '-INF'));
        }
        if (is_string($value)) {
            return $this->string($value);
        }
        if (is_array($value)) {
            return $depth > $this->maxDepth ? sprintf('[array(%d)]', count($value)) : $this->items($value, $depth);
        }
        if (is_object($value)) {
            return $this->object($value, $depth);
        }
        if (is_resource($value) || gettype($value) === 'resource (closed)') {
            return is_resource($value) ? sprintf('resource(%s)', get_resource_type($value)) : 'resource(closed)';
        }

        return gettype($value);
    }

    /**
     * @param iterable<array-key, mixed> $items
     *
     * @return array<array-key, mixed>
     */
    private function items(iterable $items, int $depth): array
    {
        $result = [];
        $count = 0;
        $skipped = 0;
        foreach ($items as $key => $item) {
            if ($count >= $this->maxItems) {
                ++$skipped;
                continue;
            }
            ++$count;
            $result[$key] = is_string($key) && $this->masker->isSecret($key) ? Masker::MASK : $this->value($item, $depth + 1);
        }
        if ($skipped > 0) {
            $result['…'] = sprintf('+%d more', $skipped);
        }

        return $result;
    }

    /**
     * @return array<array-key, mixed>|string
     */
    private function object(object $object, int $depth)
    {
        $class = get_class($object);
        if ($object instanceof \Closure) {
            $function = new \ReflectionFunction($object);

            return sprintf('Closure defined in %s:%d', (string) $function->getFileName(), (int) $function->getStartLine());
        }
        if ($object instanceof \DateTimeInterface) {
            return ['__class' => $class, 'value' => $object->format(DATE_ATOM)];
        }
        $id = spl_object_id($object);
        if (isset($this->path[$id])) {
            return sprintf('*RECURSION %s*', $class);
        }
        if ($depth > $this->maxDepth) {
            return sprintf('[object %s]', $class);
        }

        $this->path[$id] = true;
        try {
            if ($object instanceof \Throwable) {
                return ['__class' => $class, 'message' => $this->string($object->getMessage()), 'code' => $object->getCode(),
                    'file' => $object->getFile(), 'line' => $object->getLine()];
            }
            $properties = [];
            // The array cast shows every property; private ones come as "\0Class\0name", protected as "\0*\0name"
            foreach ((array) $object as $name => $value) {
                $name = (string) $name;
                $properties[$name[0] === "\0" ? substr($name, (int) strrpos($name, "\0") + 1) : $name] = $value;
            }

            return ['__class' => $class] + $this->items($properties, $depth);
        } finally {
            unset($this->path[$id]);
        }
    }

    private function string(string $value): string
    {
        $length = strlen($value);
        if ($length > $this->maxStringBytes) {
            $cut = substr($value, 0, $this->maxStringBytes);
            // Do not leave half of a multibyte character at the end
            for ($i = 0; $i < 3 && preg_match('//u', $cut) !== 1; ++$i) {
                $cut = substr($cut, 0, -1);
            }
            $value = $cut . sprintf('… (+%d bytes)', $length - strlen($cut));
        }
        if (preg_match('//u', $value) !== 1) {
            // Invalid UTF-8 would make the whole event unencodable: replace the broken bytes with U+FFFD
            $value = (string) json_decode((string) json_encode($value, JSON_INVALID_UTF8_SUBSTITUTE));
        }

        return $value;
    }
}
