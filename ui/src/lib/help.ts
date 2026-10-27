// The help of the viewer: how it works and how to add things. Longer texts than the UI dictionaries,
// so they live here, one list of topics per locale with the same topics and the same shape.
// `code` in a text is shown as code; nothing else is markup.

import type { Locale } from './i18n/messages';

export const helpTopics = [
  'overview',
  'library',
  'projects',
  'servers',
  'push',
  'reading',
  'shortcuts',
] as const;
export type HelpTopic = (typeof helpTopics)[number];

export type HelpBlock =
  | { kind: 'text'; text: string }
  | { kind: 'steps'; items: string[] }
  | { kind: 'list'; items: string[] }
  | { kind: 'code'; text: string }
  | { kind: 'tip'; text: string };

export interface HelpSection {
  id: HelpTopic;
  title: string;
  /** One line under the title in the list of topics. */
  summary: string;
  blocks: HelpBlock[];
}

const installViewer =
  'curl -fsSL https://github.com/hightemp/wapp_php_debugger/releases/latest/download/install.sh | sh';
const installLibrary =
  'curl -fsSLO https://github.com/hightemp/wapp_php_debugger/releases/latest/download/wapp-debugger.php';
const boot = `require __DIR__ . '/wapp-debugger.php';
WappDebugger\\Debugger::boot(__DIR__ . '/dws_debugger.json');`;
const config = `{
    "enabled": true,
    "logs_dir": "tmp/logs"
}`;
const logging = `__d(DWS_INFO, 'Order created', ['order' => $order]);
$total = __log($price) * $quantity;   // a value right inside an expression
__start('db'); /* … */ __stop('db');  // a timer`;
const push = `"push_url": "http://viewer.lan:8821/ingest/v2",
"push_token": "…"`;

const en: HelpSection[] = [
  {
    id: 'overview',
    title: 'How it works',
    summary: 'The library writes, the viewer reads and shows',
    blocks: [
      {
        kind: 'text',
        text: 'wapp-debugger has two parts. The PHP library in your project writes every request (its messages, timers, errors, SQL and the outcome) into a log file of its own, one JSONL file per request. The viewer reads these files, keeps them in its database and shows new requests the moment they happen.',
      },
      {
        kind: 'steps',
        items: [
          'Connect the library to the project: two lines at the start of the request and `dws_debugger.json` next to it.',
          'Add the project here: its logs directory on this computer or on a server.',
          'Open a page or run a script: the request appears in the feed with everything it logged.',
        ],
      },
      {
        kind: 'text',
        text: 'The logs reach the viewer in one of three ways: from a directory on this computer, over SSH from another server (nothing to install there), or over HTTP from the library itself when there is no SSH. When the viewer cannot reach the application at all (containers, pods), the library or an agent pushes the logs to the viewer.',
      },
      {
        kind: 'tip',
        text: 'The panel on the left holds the projects and servers; the feed shows requests of the chosen project, a server or all of them; a request opens its messages, and a message its context and stack.',
      },
    ],
  },
  {
    id: 'library',
    title: 'Connect the library',
    summary: 'PHP 7.4+, one file, no dependencies',
    blocks: [
      { kind: 'text', text: 'Put the library into the root of your project:' },
      { kind: 'code', text: installLibrary },
      {
        kind: 'text',
        text: 'Then, as early as possible in the request (the front controller, the CLI entry point):',
      },
      { kind: 'code', text: boot },
      {
        kind: 'text',
        text: 'and `dws_debugger.json` in the project root. The logs directory must be writable by PHP:',
      },
      { kind: 'code', text: config },
      {
        kind: 'text',
        text: 'Errors, exceptions and fatal errors are logged by themselves. From the code:',
      },
      { kind: 'code', text: logging },
      {
        kind: 'tip',
        text: 'Laravel, Symfony, WordPress and 1C-Bitrix have ready integrations in `client-php/integrations`. The viewer reads logs of library v1 too.',
      },
      { kind: 'text', text: 'The viewer itself (Linux, macOS):' },
      { kind: 'code', text: installViewer },
    ],
  },
  {
    id: 'projects',
    title: 'Add a project',
    summary: 'A logs directory on this computer or a server',
    blocks: [
      {
        kind: 'text',
        text: 'A project is one application: its root and the directory its library writes logs to.',
      },
      {
        kind: 'steps',
        items: [
          '**Add project** (+ in the projects panel).',
          'Choose the server: this computer, or one added under **Servers**.',
          'Give the name, the project root and the logs directory (`logs_dir` of `dws_debugger.json`, relative to the root or absolute).',
          'Save: the viewer imports what is already there and follows new files.',
        ],
      },
      {
        kind: 'text',
        text: 'Or let the viewer find projects: **Servers → Find projects** walks the given directories for `dws_debugger.json` and adds a project in one click.',
      },
      {
        kind: 'list',
        items: [
          'The **Debugger** switch of a project turns logging on and off: it flips `enabled` in `dws_debugger.json`.',
          '**Import a request** takes an exported request, a log file of the library, a PHP error log, a Laravel log or an nginx error log.',
          'Project settings keep the retention (days, megabytes) and make a push token.',
        ],
      },
    ],
  },
  {
    id: 'servers',
    title: 'Servers: SSH and HTTP',
    summary: 'Projects on stage and prod',
    blocks: [
      {
        kind: 'text',
        text: '**Servers → Add server**, then add a project to that server. **Check** on a server shows each step: the connection, the logs directories, the config and the PHP version.',
      },
      {
        kind: 'list',
        items: [
          '**SSH** is the main way: nothing to open or install on the server. The host is a name or an alias from `~/.ssh/config` (user, port, keys and ProxyJump come from there); keys come from the config or ssh-agent. A new host shows its fingerprint to confirm.',
          '**HTTP** is for servers without SSH (shared hosting): set `http_token` (32+ characters) in `dws_debugger.json` of the project and add a server with the address of the application and that token. Use `https://`.',
          'The **environment** (dev, stage, prod) colors the server; with the frame setting on, stage and prod frame the window. Clearing logs and removing a project there ask for its name.',
          '**Read-only** servers allow neither deleting logs nor switching the debugger.',
        ],
      },
      {
        kind: 'tip',
        text: '**Export / import** gives servers and projects as YAML without secrets, to keep in git or hand to a colleague.',
      },
    ],
  },
  {
    id: 'push',
    title: 'Push logs to the viewer',
    summary: 'For containers and servers out of reach',
    blocks: [
      {
        kind: 'text',
        text: 'When the viewer cannot reach the application, the logs come to the viewer.',
      },
      {
        kind: 'steps',
        items: [
          'Start the viewer with a receiver: `wapp-debugger serve --ingest-addr 0.0.0.0:8821`.',
          'In the project settings, **Make a push token**: it is shown once with the keys for `dws_debugger.json`.',
          'With these keys the library sends its events instead of writing files.',
        ],
      },
      { kind: 'code', text: push },
      {
        kind: 'text',
        text: 'Or leave the library writing files and run `wapp-debugger agent --logs-dir … --push …` next to them: it sends new lines as they are written.',
      },
      {
        kind: 'tip',
        text: 'With pushing on, `jsTag()` of the library adds JavaScript errors of the page to its PHP request.',
      },
    ],
  },
  {
    id: 'reading',
    title: 'Reading requests',
    summary: 'Filters, views, comparison',
    blocks: [
      {
        kind: 'list',
        items: [
          'The filter line takes words and conditions: `level:error`, `tag:sql`, `status:5xx`, `duration>500`, `method:POST`, `pinned:yes`.',
          'A request has views: **Messages** (with level chips and a search), **Timeline** of timers and SQL, **SQL** with slow and repeated queries, **Superglobals**.',
          'A message shows its context as a tree, the code around its line and the stack; links open the file in your IDE.',
          '**Pin** and **Note** mark a request; **Export** saves it as a file to import elsewhere; **Copy as curl** and **Open again** repeat it.',
          'Open requests become tabs; ⇄ on a tab compares it with the current one.',
          '**Pause** holds the feed still; what comes in meanwhile is counted.',
        ],
      },
    ],
  },
  {
    id: 'shortcuts',
    title: 'Keyboard',
    summary: 'Move without the mouse',
    blocks: [
      {
        kind: 'list',
        items: [
          '`j` / `k`: the next or previous request; `J` / `K`: the next or previous message.',
          '`Esc` closes the message, then the request.',
          '`/` goes to the filter line, `g` `p` to the projects.',
          '`Ctrl+K` (`⌘K` on macOS) opens the commands: projects, servers, filters, themes.',
          '`?` lists the shortcuts.',
        ],
      },
    ],
  },
];

const ru: HelpSection[] = [
  {
    id: 'overview',
    title: 'Как это работает',
    summary: 'Библиотека пишет, просмотрщик читает и показывает',
    blocks: [
      {
        kind: 'text',
        text: 'wapp-debugger состоит из двух частей. PHP-библиотека в вашем проекте записывает каждый запрос (сообщения, таймеры, ошибки, SQL и итог) в свой лог-файл: один JSONL-файл на запрос. Просмотрщик читает эти файлы, хранит их в своей базе и показывает новые запросы в тот же момент.',
      },
      {
        kind: 'steps',
        items: [
          'Подключите библиотеку к проекту: две строки в начале запроса и `dws_debugger.json` рядом.',
          'Добавьте проект здесь: его каталог логов на этом компьютере или на сервере.',
          'Откройте страницу или запустите скрипт: запрос появится в ленте со всем, что он записал.',
        ],
      },
      {
        kind: 'text',
        text: 'Логи попадают в просмотрщик одним из трёх путей: из каталога на этом компьютере, по SSH с другого сервера (там ничего ставить не нужно) или по HTTP от самой библиотеки, когда SSH нет. Если просмотрщик вообще не может достучаться до приложения (контейнеры, поды), библиотека или агент сами отправляют логи в просмотрщик.',
      },
      {
        kind: 'tip',
        text: 'Слева — проекты и серверы; лента показывает запросы выбранного проекта, сервера или всех сразу; запрос открывает свои сообщения, сообщение — контекст и стек.',
      },
    ],
  },
  {
    id: 'library',
    title: 'Подключить библиотеку',
    summary: 'PHP 7.4+, один файл, без зависимостей',
    blocks: [
      { kind: 'text', text: 'Положите библиотеку в корень проекта:' },
      { kind: 'code', text: installLibrary },
      {
        kind: 'text',
        text: 'Затем как можно раньше в запросе (фронт-контроллер, точка входа CLI):',
      },
      { kind: 'code', text: boot },
      {
        kind: 'text',
        text: 'и `dws_debugger.json` в корне проекта. Каталог логов должен быть доступен PHP на запись:',
      },
      { kind: 'code', text: config },
      { kind: 'text', text: 'Ошибки, исключения и фатальные ошибки пишутся сами. Из кода:' },
      { kind: 'code', text: logging },
      {
        kind: 'tip',
        text: 'Для Laravel, Symfony, WordPress и 1С-Битрикс есть готовые интеграции в `client-php/integrations`. Логи библиотеки v1 просмотрщик тоже читает.',
      },
      { kind: 'text', text: 'Сам просмотрщик (Linux, macOS):' },
      { kind: 'code', text: installViewer },
    ],
  },
  {
    id: 'projects',
    title: 'Добавить проект',
    summary: 'Каталог логов на этом компьютере или на сервере',
    blocks: [
      {
        kind: 'text',
        text: 'Проект — это одно приложение: его корень и каталог, куда библиотека пишет логи.',
      },
      {
        kind: 'steps',
        items: [
          '**Добавить проект** (+ в панели проектов).',
          'Выберите сервер: этот компьютер или добавленный в **Серверах**.',
          'Укажите название, корень проекта и каталог логов (`logs_dir` из `dws_debugger.json`, от корня или абсолютный).',
          'Сохраните: просмотрщик загрузит то, что уже есть, и будет следить за новыми файлами.',
        ],
      },
      {
        kind: 'text',
        text: 'Или пусть просмотрщик найдёт проекты сам: **Серверы → Найти проекты** ищет `dws_debugger.json` в указанных каталогах и добавляет проект одним щелчком.',
      },
      {
        kind: 'list',
        items: [
          'Переключатель **Отладчик** включает и выключает запись: он меняет `enabled` в `dws_debugger.json`.',
          '**Импорт запроса** принимает экспортированный запрос, лог-файл библиотеки, лог ошибок PHP, лог Laravel или лог ошибок nginx.',
          'В настройках проекта — срок хранения (дни, мегабайты) и токен для отправки логов.',
        ],
      },
    ],
  },
  {
    id: 'servers',
    title: 'Серверы: SSH и HTTP',
    summary: 'Проекты на stage и prod',
    blocks: [
      {
        kind: 'text',
        text: '**Серверы → Добавить сервер**, затем добавьте проект на этот сервер. **Проверить** у сервера показывает каждый шаг: подключение, каталоги логов, конфиг и версию PHP.',
      },
      {
        kind: 'list',
        items: [
          '**SSH** — основной способ: на сервере ничего не нужно открывать и ставить. Хост — имя или алиас из `~/.ssh/config` (пользователь, порт, ключи и ProxyJump берутся оттуда); ключи — из конфига или ssh-agent. Новый хост покажет отпечаток ключа для подтверждения.',
          '**HTTP** — для серверов без SSH (виртуальный хостинг): задайте `http_token` (от 32 символов) в `dws_debugger.json` проекта и добавьте сервер с адресом приложения и этим токеном. Используйте `https://`.',
          '**Окружение** (dev, stage, prod) окрашивает сервер; с включённой рамкой stage и prod обводят окно. Очистка логов и удаление проекта там спрашивают его название.',
          'Сервер **только для чтения** не даёт удалять логи и переключать отладчик.',
        ],
      },
      {
        kind: 'tip',
        text: '**Экспорт / импорт** отдаёт серверы и проекты в YAML без секретов: хранить в git или передать коллеге.',
      },
    ],
  },
  {
    id: 'push',
    title: 'Отправка логов в просмотрщик',
    summary: 'Для контейнеров и недоступных серверов',
    blocks: [
      {
        kind: 'text',
        text: 'Когда просмотрщик не может достучаться до приложения, логи приходят к нему сами.',
      },
      {
        kind: 'steps',
        items: [
          'Запустите просмотрщик с приёмником: `wapp-debugger serve --ingest-addr 0.0.0.0:8821`.',
          'В настройках проекта — **Создать токен**: он показывается один раз вместе с ключами для `dws_debugger.json`.',
          'С этими ключами библиотека отправляет события вместо записи файлов.',
        ],
      },
      { kind: 'code', text: push },
      {
        kind: 'text',
        text: 'Или оставьте библиотеке запись файлов и запустите рядом `wapp-debugger agent --logs-dir … --push …`: он отправляет новые строки по мере записи.',
      },
      {
        kind: 'tip',
        text: 'При включённой отправке `jsTag()` библиотеки добавляет JavaScript-ошибки страницы к её PHP-запросу.',
      },
    ],
  },
  {
    id: 'reading',
    title: 'Как читать запросы',
    summary: 'Фильтры, виды, сравнение',
    blocks: [
      {
        kind: 'list',
        items: [
          'Строка фильтра принимает слова и условия: `level:error`, `tag:sql`, `status:5xx`, `duration>500`, `method:POST`, `pinned:yes`.',
          'У запроса есть виды: **Сообщения** (с фильтром по уровням и поиском), **Шкала времени** таймеров и SQL, **SQL** с медленными и повторными запросами, **Суперглобальные**.',
          'Сообщение показывает контекст деревом, код вокруг строки и стек; ссылки открывают файл в IDE.',
          '**Закрепить** и **Заметка** помечают запрос; **Экспорт** сохраняет его в файл для импорта в другом месте; **Копировать как curl** и **Открыть снова** повторяют его.',
          'Открытые запросы становятся вкладками; ⇄ на вкладке сравнивает её с текущей.',
          '**Пауза** останавливает ленту; что пришло за это время, считается.',
        ],
      },
    ],
  },
  {
    id: 'shortcuts',
    title: 'Клавиатура',
    summary: 'Без мыши',
    blocks: [
      {
        kind: 'list',
        items: [
          '`j` / `k`: следующий или предыдущий запрос; `J` / `K`: следующее или предыдущее сообщение.',
          '`Esc` закрывает сообщение, затем запрос.',
          '`/` — к строке фильтра, `g` `p` — к проектам.',
          '`Ctrl+K` (`⌘K` в macOS) открывает команды: проекты, серверы, фильтры, темы.',
          '`?` — список сочетаний клавиш.',
        ],
      },
    ],
  },
];

export const help: Record<Locale, HelpSection[]> = { en, ru };

export type HelpSpan = { kind: 'text' | 'code' | 'strong'; text: string };

/** A text of the help split into plain text, `code` and **strong** parts. */
export function helpSpans(text: string): HelpSpan[] {
  const spans: HelpSpan[] = [];
  const pattern = /`([^`]+)`|\*\*([^*]+)\*\*/g;
  let last = 0;
  for (const match of text.matchAll(pattern)) {
    if (match.index > last) {
      spans.push({ kind: 'text', text: text.slice(last, match.index) });
    }
    spans.push(
      match[1] !== undefined
        ? { kind: 'code', text: match[1] }
        : { kind: 'strong', text: match[2] ?? '' },
    );
    last = match.index + match[0].length;
  }
  if (last < text.length) {
    spans.push({ kind: 'text', text: text.slice(last) });
  }
  return spans;
}
