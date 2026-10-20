/*
 * wapp-debugger JS client: errors of a page go to the viewer next to the PHP request that made it.
 *
 *   <script src="http://VIEWER:8821/ingest/debugger.js" data-token="PUSH_TOKEN"
 *           data-request-id="<?= $debugger->requestId() ?>" async></script>
 *
 * Debugger::jsTag() of the PHP library prints this tag. Uncaught errors, unhandled rejections and
 * console.error/warn (data-console="error,warn,info" to change) become a request of lang "js", a
 * child of the PHP request, in log format v2. The token is visible to anyone who opens the page:
 * use it on dev and stage only.
 */
(function (global) {
  'use strict';

  var VERSION = '1.0.0';
  var CROCKFORD = '0123456789ABCDEFGHJKMNPQRSTVWXYZ';

  function ulid(now) {
    var time = '';
    for (var t = now, i = 0; i < 10; i++, t = Math.floor(t / 32)) {
      time = CROCKFORD.charAt(t % 32) + time;
    }
    var random = new Uint8Array(16);
    global.crypto.getRandomValues(random);
    var tail = '';
    for (var j = 0; j < 16; j++) {
      tail += CROCKFORD.charAt(random[j] % 32);
    }
    return time + tail;
  }

  // "    at render (https://x/app.js:120:15)" (Chrome, Edge) and "render@https://x/app.js:120:15"
  // (Firefox, Safari) into frames of the format
  function parseStack(stack) {
    var frames = [];
    String(stack || '')
      .split('\n')
      .forEach(function (line) {
        var match =
          /^\s*at (?:(.+?) \()?(.+?):(\d+):\d+\)?$/.exec(line) ||
          /^(.*?)@(.+?):(\d+):\d+$/.exec(line);
        if (!match) {
          return;
        }
        var frame = { file: match[2], line: Number(match[3]) };
        if (match[1]) {
          frame['function'] = match[1];
        }
        frames.push(frame);
      });
    return frames;
  }

  function plain(value) {
    if (value instanceof Error) {
      return String(value.stack || value);
    }
    try {
      return JSON.parse(JSON.stringify(value));
    } catch {
      return String(value);
    }
  }

  function text(value) {
    return typeof value === 'string'
      ? value
      : value instanceof Error
        ? value.message
        : JSON.stringify(plain(value));
  }

  function defaultSend(url, body, last) {
    // A beacon outlives the page; text/plain and no headers keep the request simple (no CORS preflight)
    if (
      last &&
      global.navigator &&
      global.navigator.sendBeacon &&
      global.navigator.sendBeacon(url, body)
    ) {
      return;
    }
    if (global.fetch) {
      global
        .fetch(url, { method: 'POST', body: body, keepalive: true, mode: 'cors' })
        .catch(function () {});
    }
  }

  function start(options) {
    var now = options.now || Date.now;
    var elapsed =
      options.elapsed ||
      function () {
        return global.performance ? global.performance.now() : 0;
      };
    var send = options.send || defaultSend;
    var url =
      options.endpoint +
      (options.endpoint.indexOf('?') < 0 ? '?' : '&') +
      'token=' +
      encodeURIComponent(options.token);
    var requestId = (options.id || ulid)(now());
    var seq = 0;
    var lastTs = 0;
    var buffer = [];
    var timer = null;
    var ended = false;
    var restore = [];

    function emit(type, fields) {
      if (ended) {
        return;
      }
      lastTs = Math.max(lastTs, now());
      var event = { v: 2, type: type, request_id: requestId, seq: ++seq, ts: lastTs };
      for (var key in fields) {
        if (Object.prototype.hasOwnProperty.call(fields, key) && fields[key] !== undefined) {
          event[key] = fields[key];
        }
      }
      buffer.push(JSON.stringify(event));
      if (timer === null && type !== 'request_end') {
        timer = setTimeout(flush, 500);
      }
    }

    function flush(last) {
      if (timer !== null) {
        clearTimeout(timer);
        timer = null;
      }
      if (buffer.length === 0) {
        return;
      }
      var body = buffer.join('\n') + '\n';
      buffer = [];
      try {
        send(url, body, last === true);
      } catch {
        // the page goes on whatever happens to its logs
      }
    }

    function exception(error, fallback) {
      var frames = error && error.stack ? parseStack(error.stack) : [];
      if (frames.length === 0 && fallback && fallback.file) {
        frames = [fallback];
      }
      emit('error', {
        kind: 'exception',
        level: 'error',
        message:
          error && error.message
            ? String(error.message)
            : String((fallback && fallback.message) || 'Script error'),
        exception_class: (error && error.name) || 'Error',
        source: frames[0] ? { file: frames[0].file, line: frames[0].line } : undefined,
        trace: frames.length > 0 ? frames : undefined,
      });
    }

    function listen(target, name, handler) {
      target.addEventListener(name, handler);
      restore.push(function () {
        target.removeEventListener(name, handler);
      });
    }

    emit('request_start', {
      sapi: 'web',
      lang: 'js',
      library_version: VERSION,
      parent_request_id: options.parentId || undefined,
      method: 'GET',
      url: options.url || (global.location ? String(global.location.href) : ''),
      is_ajax: false,
    });

    listen(global, 'error', function (event) {
      if (event instanceof ErrorEvent || event.error) {
        exception(event.error, {
          file: event.filename,
          line: event.lineno,
          message: event.message,
        });
      }
    });
    listen(global, 'unhandledrejection', function (event) {
      emit('message', {
        level: 'error',
        message: text(event.reason),
        tags: ['unhandledrejection'],
      });
    });
    (options.console || ['error', 'warn']).forEach(function (method) {
      var console = global.console;
      var original = console && console[method];
      if (typeof original !== 'function') {
        return;
      }
      var level =
        { error: 'error', warn: 'warning', info: 'info', log: 'message', debug: 'debug' }[method] ||
        'message';
      console[method] = function () {
        var args = Array.prototype.slice.call(arguments);
        emit('message', {
          level: level,
          message: args.map(text).join(' '),
          tags: ['console'],
          context: { args: args.map(plain) },
        });
        return original.apply(this, arguments);
      };
      restore.push(function () {
        console[method] = original;
      });
    });

    var client = {
      requestId: requestId,
      log: function (level, message, context) {
        emit('message', { level: level, message: String(message), context: context });
      },
      error: function (error) {
        exception(error);
      },
      flush: function () {
        flush(false);
      },
      end: function () {
        if (ended) {
          return;
        }
        emit('request_end', { duration_ms: Math.max(0, Math.round(elapsed() * 1000) / 1000) });
        ended = true;
        restore.forEach(function (undo) {
          undo();
        });
        flush(true);
      },
    };
    if (global.addEventListener && !options.send) {
      global.addEventListener('pagehide', client.end);
    }
    return client;
  }

  global.WappDebugger = { start: start, parseStack: parseStack, version: VERSION };

  // The <script data-token> tag starts it by itself
  var script = global.document && global.document.currentScript;
  if (script && script.dataset && script.dataset.token) {
    global.WappDebugger.client = start({
      endpoint: script.src.replace(/debugger\.js(\?.*)?$/, 'v2'),
      token: script.dataset.token,
      parentId: script.dataset.requestId,
      console: script.dataset.console ? script.dataset.console.split(',') : undefined,
    });
  }
})(typeof window !== 'undefined' ? window : this);
