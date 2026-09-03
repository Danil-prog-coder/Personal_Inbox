# Graph Report - Personal_Inbox  (2026-09-03)

## Corpus Check
- 100 files · ~71,311 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 945 nodes · 2102 edges · 48 communities (37 shown, 6 thin omitted)
- Extraction: 92% EXTRACTED · 8% INFERRED · 0% AMBIGUOUS · INFERRED: 176 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `7be42b0f`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- testing.T
- net/http.ResponseWriter
- Client
- 4. Лента, уровень 2 — сообщения источника
- Bus
- UTCNow
- gmail_test.go
- allow
- types.ts
- store_test.go
- App.tsx
- levels.ts
- telegram_test.go
- devDependencies
- format.ts
- compilerOptions
- components.test.tsx
- FeedScreen.tsx
- Personal Inbox — спецификация для дизайнера
- MessageDetails.tsx
- ConnectionsScreen.tsx
- What You Must Do When Invoked
- Personal Inbox — экраны, состояния и тексты
- meUpdate
- Personal Inbox — ТЗ продукта
- Personal Inbox — дизайн-система (Liquid Glass)
- Load
- Personal Inbox — описание прототипа
- personalinbox
- Personal Inbox
- graphify reference: extra exports and benchmark
- Personal Inbox — модель данных и API
- Personal Inbox — журнал решений
- graphify reference: query, path, explain
- Personal Inbox
- 6. Обработка сообщения моделью
- graphify reference: add a URL and watch a folder
- graphify reference: commit hook and native CLAUDE.md integration
- graphify reference: incremental update and cluster-only
- graphify reference: GitHub clone and cross-repo merge
- graphify reference: transcribe video and audio
- .claude/CLAUDE.md
- extraction-spec.md

## God Nodes (most connected - your core abstractions)
1. `newEnv()` - 80 edges
2. `UTCNow()` - 25 edges
3. `allow` - 24 edges
4. `DB` - 24 edges
5. `Client` - 22 edges
6. `fail()` - 21 edges
7. `env` - 19 edges
8. `Bus` - 19 edges
9. `Ingestor` - 18 edges
10. `newWorker()` - 18 edges

## Surprising Connections (you probably didn't know these)
- `newEnv()` --calls--> `New()`  [INFERRED]
  backend/internal/api/helpers_test.go → backend/internal/api/server.go
- `TestDistributionIncludesZeros()` --calls--> `Distribution()`  [INFERRED]
  backend/internal/store/store_test.go → backend/internal/store/queries.go
- `newScheduler()` --calls--> `New()`  [INFERRED]
  backend/internal/scheduler/scheduler_test.go → backend/internal/scheduler/scheduler.go
- `TestSummaryWindows()` --calls--> `SummaryPeriodStart()`  [INFERRED]
  backend/internal/store/store_test.go → backend/internal/store/queries.go
- `newDB()` --calls--> `Open()`  [INFERRED]
  backend/internal/store/store_test.go → backend/internal/store/db.go

## Import Cycles
- None detected.

## Communities (48 total, 6 thin omitted)

### Community 0 - "testing.T"
Cohesion: 0.05
Nodes (89): TestForgedCookieIsRejected(), TestHealthIsPublic(), TestLoginWithUnknownEmail(), TestLoginWithWrongPassword(), TestLogoutClearsSession(), TestMeRequiresAuth(), TestRegisterCreatesUserAndSession(), TestRegisterNormalizesEmailCase() (+81 more)

### Community 1 - "net/http.ResponseWriter"
Cohesion: 0.07
Nodes (42): credentials, session, Server, validEmail(), Server, randomState(), Server, Server (+34 more)

### Community 2 - "Client"
Cohesion: 0.05
Nodes (39): fakeQueue, main(), Enqueuer, env, Server, New(), Config, Incoming (+31 more)

### Community 3 - "4. Лента, уровень 2 — сообщения источника"
Cohesion: 0.33
Nodes (6): 4.1 Шапка, 4.2 Карточка сообщения, 4.3 Фильтры, 4.4 Новое сообщение в реальном времени, 4.5 Пустые состояния, 4. Лента, уровень 2 — сообщения источника

### Community 4 - "Bus"
Cohesion: 0.08
Nodes (34): fakeAnalyzer, fakeQueue, queue, Worker, newQueue(), NewWorker(), QueueReanalysis(), goodResult() (+26 more)

### Community 5 - "UTCNow"
Cohesion: 0.06
Nodes (54): main(), connect(), newScheduler(), TestStopIsIdempotent(), TestSyncAllSkipsInactiveConnections(), TestSyncAllSurvivesBrokenSource(), atNoon(), externalURL() (+46 more)

### Community 6 - "gmail_test.go"
Cohesion: 0.11
Nodes (39): bodyText(), decode(), header(), IsAuthError(), partByType(), receivedAt(), stripHTML(), encode() (+31 more)

### Community 7 - "allow"
Cohesion: 0.06
Nodes (34): hooks, PreToolUse, permissions, allow, ask, $schema, Bash(cat:*), Bash(diff:*) (+26 more)

### Community 8 - "types.ts"
Cohesion: 0.12
Nodes (23): api, ApiError, errorMessage(), request(), ConnState, Density, EMPTY_FILTERS, MessageBrief (+15 more)

### Community 9 - "store_test.go"
Cohesion: 0.10
Nodes (31): DB, scanConnection(), connection, override_log, user, PeriodStart(), mustConnection(), mustMessage() (+23 more)

### Community 10 - "App.tsx"
Cohesion: 0.16
Nodes (16): App(), readStored(), IconName, LogoIcon(), badgeFor(), Props, Rail(), Tab (+8 more)

### Community 11 - "levels.ts"
Cohesion: 0.17
Nodes (16): Icon(), PATHS, Props, LevelChip(), LEVEL_BG, LEVEL_COLOR, LEVEL_ICON, LEVEL_INK (+8 more)

### Community 12 - "telegram_test.go"
Cohesion: 0.25
Nodes (17): botAPI(), Client, makeUpdate(), newIngestor(), TestGroupChatShowsMemberCount(), TestGroupWithoutMemberCountDegradesGracefully(), TestInvalidTokenSwitchesToReauth(), TestMessageWithoutTextIsSkipped() (+9 more)

### Community 13 - "devDependencies"
Cohesion: 0.05
Nodes (38): dependencies, react, react-dom, react-router-dom, devDependencies, jsdom, @testing-library/jest-dom, @testing-library/react (+30 more)

### Community 14 - "format.ts"
Cohesion: 0.17
Nodes (15): Avatar(), Props, MessageCard(), AVATAR_GRADIENTS, avatarGradient(), formatTime(), initials(), MONTHS (+7 more)

### Community 15 - "compilerOptions"
Cohesion: 0.08
Nodes (23): compilerOptions, isolatedModules, jsx, lib, module, moduleResolution, noEmit, noFallthroughCasesInSwitch (+15 more)

### Community 16 - "components.test.tsx"
Cohesion: 0.21
Nodes (7): FiltersPanel(), GROUPS, Props, Option, Props, Segmented(), Filters

### Community 17 - "FeedScreen.tsx"
Cohesion: 0.33
Nodes (9): Props, SourceCardItem(), StreamEvent, messagesCount(), plural(), SOURCE_GRADIENT, SourceCard, FeedScreen() (+1 more)

### Community 19 - "Personal Inbox — спецификация для дизайнера"
Cohesion: 0.07
Nodes (27): 10. Данные для наполнения макетов (ориентир), 1.1 Регистрация / вход, 1.2 Задание критериев важности (первый запуск), 1. Онбординг, 2.1 Экран "Источники" — три состояния подключения, 2.2 "Добавить источник", 2.3 Процесс подключения Gmail (OAuth), 2.4 Процесс подключения Telegram (+19 more)

### Community 20 - "MessageDetails.tsx"
Cohesion: 0.31
Nodes (8): Props, LEVEL_OPTIONS, MessageDetails(), Props, visibleCategory(), Level, Message, message

### Community 21 - "ConnectionsScreen.tsx"
Cohesion: 0.31
Nodes (7): Portal(), formatSyncedAgo(), SOURCE_LABEL, Connection, ConnectionsScreen(), Props, SERVICES

### Community 22 - "What You Must Do When Invoked"
Cohesion: 0.08
Nodes (24): For /graphify add and --watch, For /graphify query, For the commit hook and native CLAUDE.md integration, For --update and --cluster-only, /graphify, Honesty Rules, Interpreter guard for subcommands, Part A - Structural extraction for code files (+16 more)

### Community 23 - "Personal Inbox — экраны, состояния и тексты"
Cohesion: 0.17
Nodes (12): 1. Вход и регистрация, 2. Критерии важности (онбординг), 3. Лента, уровень 1 — источники, 5. Детали сообщения, 6. Сводка, 7.1 «Добавить источник», 7. Источники, 8.1 Критерии важности (+4 more)

### Community 31 - "Personal Inbox — ТЗ продукта"
Cohesion: 0.17
Nodes (12): 1. Что это, 2. Технологический стек — зафиксировано, 3. Язык и локализация, 4. Носители и темы, 5.1 Gmail, 5.2 Telegram, 5.3 Источники из планов развития, 5. Источники (+4 more)

### Community 32 - "Personal Inbox — дизайн-система (Liquid Glass)"
Cohesion: 0.17
Nodes (12): 10. Доступность, 11. Чек-лист перед тем, как считать UI-задачу сделанной, 1. Железные правила, 2. Цвета важности, 3. Размытие — таблица значений, 4. Линза — главный жест, 5. Остальное движение, 6. Типографика (+4 more)

### Community 33 - "Load"
Cohesion: 0.50
Nodes (7): defaultDatabasePath(), dotEnvPath(), env(), flag(), Load(), loadDotEnv(), repoRoot()

### Community 34 - "Personal Inbox — описание прототипа"
Cohesion: 0.18
Nodes (10): 1. Что решает продукт, 2. Визуальная система, 3. Экраны, 4. Движение и обратная связь, 5. Данные прототипа, 6. Решения по открытым вопросам спецификации, 7. Как смотреть, 8. Чего в прототипе нет (+2 more)

### Community 39 - "Personal Inbox"
Cohesion: 0.22
Nodes (9): graphify — граф проекта, обязательный вход в код, Personal Inbox, Границы: чего в MVP нет, Дизайн: правила, которые не обсуждаются, Если возник вопрос, ⛔ Прочитать до первой строки кода, Режим работы: максимальная автономность, Стек (+1 more)

### Community 40 - "graphify reference: extra exports and benchmark"
Cohesion: 0.22
Nodes (8): graphify reference: extra exports and benchmark, Step 6b - Wiki (only if --wiki flag), Step 7 - Neo4j export (only if --neo4j or --neo4j-push flag), Step 7a - FalkorDB export (only if --falkordb or --falkordb-push flag), Step 7b - SVG export (only if --svg flag), Step 7c - GraphML export (only if --graphml flag), Step 7d - MCP server (only if --mcp flag), Step 8 - Token reduction benchmark (only if total_words > 5000)

### Community 41 - "Personal Inbox — модель данных и API"
Cohesion: 0.14
Nodes (13): 1. Перечисления, 2. Таблицы, 3. Категория и срок — свободные строки, 4. Фильтр «Период», 5. Периоды сводки, 6.1 Как новые сообщения попадают на фронт, 6. HTTP API, 7. Демо-данные (+5 more)

### Community 42 - "Personal Inbox — журнал решений"
Cohesion: 0.25
Nodes (7): Personal Inbox — журнал решений, Интерфейс, Как добавлять сюда записи, Принято при реализации бэкенда, Принято при реализации фронтенда, Продукт и стек, Функциональность

### Community 43 - "graphify reference: query, path, explain"
Cohesion: 0.33
Nodes (5): For /graphify explain, For /graphify path, graphify reference: query, path, explain, Step 0 — Constrained query expansion (REQUIRED before traversal), Step 1 — Traversal

### Community 44 - "Personal Inbox"
Cohesion: 0.29
Nodes (7): Personal Inbox, Граф проекта, Документация, Запуск, Референс дизайна, Статус, Структура

### Community 46 - "6. Обработка сообщения моделью"
Cohesion: 0.40
Nodes (5): 6.1 Жизненный цикл, 6.2 Что уходит в модель, 6.3 Хранение, 6.4 Смена критериев, 6. Обработка сообщения моделью

### Community 48 - "graphify reference: add a URL and watch a folder"
Cohesion: 0.50
Nodes (3): For /graphify add, For --watch, graphify reference: add a URL and watch a folder

### Community 49 - "graphify reference: commit hook and native CLAUDE.md integration"
Cohesion: 0.50
Nodes (3): For git commit hook, For native CLAUDE.md integration, graphify reference: commit hook and native CLAUDE.md integration

### Community 50 - "graphify reference: incremental update and cluster-only"
Cohesion: 0.50
Nodes (3): For --cluster-only, For --update (incremental re-extraction), graphify reference: incremental update and cluster-only

## Knowledge Gaps
- **248 isolated node(s):** `graphify`, `$schema`, `Read`, `Glob`, `Grep` (+243 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 298 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **6 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `UTCNow()` connect `UTCNow` to `testing.T`, `net/http.ResponseWriter`, `Client`, `Bus`, `gmail_test.go`, `store_test.go`?**
  _High betweenness centrality (0.025) - this node is a cross-community bridge._
- **Why does `env` connect `Client` to `testing.T`, `Bus`, `UTCNow`?**
  _High betweenness centrality (0.025) - this node is a cross-community bridge._
- **Why does `newEnv()` connect `testing.T` to `Client`, `Bus`, `UTCNow`?**
  _High betweenness centrality (0.023) - this node is a cross-community bridge._
- **Are the 73 inferred relationships involving `newEnv()` (e.g. with `TestForgedCookieIsRejected()` and `TestHealthIsPublic()`) actually correct?**
  _`newEnv()` has 73 INFERRED edges - model-reasoned connections that need verification._
- **Are the 8 inferred relationships involving `UTCNow()` (e.g. with `PeriodStart()` and `SummaryPeriodStart()`) actually correct?**
  _`UTCNow()` has 8 INFERRED edges - model-reasoned connections that need verification._
- **What connects `graphify`, `$schema`, `Read` to the rest of the system?**
  _248 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `testing.T` be split into smaller, more focused modules?**
  _Cohesion score 0.05292929292929293 - nodes in this community are weakly interconnected._