# Graph Report - Personal_Inbox  (2026-09-03)

## Corpus Check
- 103 files · ~71,311 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 879 nodes · 2025 edges · 66 communities (28 shown, 38 thin omitted)
- Extraction: 89% EXTRACTED · 11% INFERRED · 0% AMBIGUOUS · INFERRED: 219 edges (avg confidence: 0.86)
- Token cost: 243,830 input · 7,200 output

## Community Hubs (Navigation)
- Auth & API Test Suite
- API Request Handlers
- Session & Connections API
- Server Bootstrap & Scheduler
- Analysis Queue
- Gmail Source Adapter
- Frontend App Shell & Icons
- Frontend Dependencies
- Claude Code Settings
- Project Rules & Spec Overview
- Store Test Fixtures
- Store: Connections & Messages
- Feed Filters & Stream Hook
- Graphify Skill Reference
- TypeScript Config
- Message Card Components
- Telegram Source Tests
- Sources Feed & HTTP API
- Importance Level UI
- Avatar & Portal Components
- Ingest Pipeline Tests
- User Store
- Sources Spec & Connection Screen
- MVP Scope Boundaries
- Me Endpoint
- Design Rules Checklist
- Graphify Project Integration
- Wiki & Obsidian Export
- Confidence Scoring Rubric
- GitHub Clone & Merge
- Monorepo Merge Flow
- Graph Build & Diff
- Avatar Color & Source Feed
- Rejected Broadsheet Design
- Glass Theme Variables
- Language Rule
- FalkorDB Export
- GraphML Export
- Neo4j Export
- SVG Export
- Token Reduction Benchmark
- Hyperedges Rule
- Extraction JSON Schema
- Semantic Similarity Rule
- Query Vocab Expansion
- Whisper Prompt Hint
- Cluster-Only Rerun
- God Nodes Analysis
- Graph Report Output
- Honesty Rules
- Install Step
- Graph Health Check
- Community Labeling Step
- Manifest & Cleanup Step
- Accessibility
- Glass Rule
- Inline SVG Icons
- Typography
- Auth Screen
- Project Root Node
- Docs Table (README)
- Repo Structure (README)

## God Nodes (most connected - your core abstractions)
1. `newEnv()` - 80 edges
2. `UTCNow()` - 25 edges
3. `allow` - 24 edges
4. `DB` - 24 edges
5. `Client` - 22 edges
6. `fail()` - 21 edges
7. `env` - 19 edges
8. `Bus` - 19 edges
9. `newWorker()` - 18 edges
10. `Ingestor` - 18 edges

## Surprising Connections (you probably didn't know these)
- `Решение №35: бэкенд переписан на Go` --shares_data_with--> `Стек (CLAUDE.md)`  [INFERRED]
  docs/04-decisions.md → CLAUDE.md
- `Режим работы: максимальная автономность` --shares_data_with--> `ТЗ продукта`  [INFERRED]
  CLAUDE.md → docs/00-product-spec.md
- `CLAUDE.md — правила проекта` --references--> `Модель данных и API`  [EXTRACTED]
  CLAUDE.md → docs/03-data-model.md
- `Стек (CLAUDE.md)` --shares_data_with--> `Технологический стек (ТЗ)`  [INFERRED]
  CLAUDE.md → docs/00-product-spec.md
- `Режим работы: максимальная автономность` --references--> `Журнал решений`  [EXTRACTED]
  CLAUDE.md → docs/04-decisions.md

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Core graphify build pipeline (Steps 0-9)** — claude_skills_graphify_skill_step0_github, claude_skills_graphify_skill_step1_install, claude_skills_graphify_skill_step2_detect, claude_skills_graphify_skill_step3_extract, claude_skills_graphify_skill_step4_build, claude_skills_graphify_skill_step5_label, claude_skills_graphify_skill_step9_cleanup [INFERRED 0.85]
- **Extraction confidence & node-ID integrity system** — claude_skills_graphify_skill_confidence_tiers, claude_skills_graphify_references_extraction_spec_confidence_score_rubric, claude_skills_graphify_references_extraction_spec_node_id_format, claude_skills_graphify_skill_honesty_rules [INFERRED 0.85]
- **Graph query interface: query/path/explain** — claude_skills_graphify_references_query_bfs_dfs_traversal, claude_skills_graphify_references_query_graphify_path, claude_skills_graphify_references_query_graphify_explain, claude_skills_graphify_references_query_save_result [EXTRACTED 1.00]
- **Линза как единый компонент в четырёх местах интерфейса** — docs_01_design_system_lens_component, docs_02_screens_summary, docs_02_screens_message_details, docs_reference_original_personal_inbox_design_spec_lens_pattern [INFERRED 0.85]
- **Согласованные границы MVP через все спецификации** — claude_mvp_boundaries, docs_00_product_spec_mvp_exclusions, docs_reference_original_personal_inbox_design_spec_mvp_exclusions, docs_reference_original_personal_inbox_opisanie_prototipa [INFERRED 0.85]
- **Документирование переезда бэкенда с Python на Go** — readme_status, docs_04_decisions_go_backend_rewrite, claude_stack, docs_00_product_spec_tech_stack [INFERRED 0.80]

## Communities (66 total, 38 thin omitted)

### Community 0 - "Auth & API Test Suite"
Cohesion: 0.05
Nodes (89): TestForgedCookieIsRejected(), TestHealthIsPublic(), TestLoginWithUnknownEmail(), TestLoginWithWrongPassword(), TestLogoutClearsSession(), TestMeRequiresAuth(), TestRegisterCreatesUserAndSession(), TestRegisterNormalizesEmailCase() (+81 more)

### Community 1 - "API Request Handlers"
Cohesion: 0.06
Nodes (35): fakeQueue, main(), Enqueuer, env, Server, New(), defaultDatabasePath(), dotEnvPath() (+27 more)

### Community 2 - "Session & Connections API"
Cohesion: 0.07
Nodes (42): credentials, session, Server, validEmail(), Server, randomState(), Server, Server (+34 more)

### Community 3 - "Server Bootstrap & Scheduler"
Cohesion: 0.06
Nodes (57): main(), connect(), newScheduler(), TestStopIsIdempotent(), TestSyncAllSkipsInactiveConnections(), TestSyncAllSurvivesBrokenSource(), atNoon(), externalURL() (+49 more)

### Community 4 - "Analysis Queue"
Cohesion: 0.07
Nodes (38): fakeAnalyzer, fakeQueue, queue, Worker, newQueue(), NewWorker(), QueueReanalysis(), goodResult() (+30 more)

### Community 5 - "Gmail Source Adapter"
Cohesion: 0.11
Nodes (40): bodyText(), decode(), header(), IsAuthError(), isStaleHistory(), partByType(), receivedAt(), stripHTML() (+32 more)

### Community 6 - "Frontend App Shell & Icons"
Cohesion: 0.09
Nodes (32): App(), readStored(), Icon(), IconName, LogoIcon(), PATHS, Props, badgeFor() (+24 more)

### Community 7 - "Frontend Dependencies"
Cohesion: 0.05
Nodes (38): dependencies, react, react-dom, react-router-dom, devDependencies, jsdom, @testing-library/jest-dom, @testing-library/react (+30 more)

### Community 8 - "Claude Code Settings"
Cohesion: 0.06
Nodes (34): hooks, PreToolUse, permissions, allow, ask, $schema, Bash(cat:*), Bash(diff:*) (+26 more)

### Community 9 - "Project Rules & Spec Overview"
Cohesion: 0.10
Nodes (30): CLAUDE.md — правила проекта, Режим работы: максимальная автономность, ТЗ продукта, Структура ленты (два уровня), Жизненный цикл сообщения (PROCESSING → DONE), Дизайн-система Liquid Glass, Таблица значений размытия, Цвета важности (CRITICAL/HIGH/NORMAL/LOW) (+22 more)

### Community 10 - "Store Test Fixtures"
Cohesion: 0.19
Nodes (27): PeriodStart(), Connection, Message, User, mustConnection(), mustMessage(), mustUser(), newDB() (+19 more)

### Community 11 - "Store: Connections & Messages"
Cohesion: 0.14
Nodes (9): Connection, DB, scanConnection(), stringFromNull(), Message, DB, Override, scanMessage() (+1 more)

### Community 12 - "Feed Filters & Stream Hook"
Cohesion: 0.14
Nodes (16): FiltersPanel(), GROUPS, Props, MessageDetails(), StreamEvent, api, ConnState, EMPTY_FILTERS (+8 more)

### Community 13 - "Graphify Skill Reference"
Cohesion: 0.09
Nodes (24): Graphify Skill Reference (root CLAUDE.md), /graphify add <url>, --watch background watcher, graphify.ingest.ingest function, MCP stdio server (--mcp), Node ID format rule, Extraction subagent prompt template, graphify claude install (native CLAUDE.md integration) (+16 more)

### Community 14 - "TypeScript Config"
Cohesion: 0.08
Nodes (23): compilerOptions, isolatedModules, jsx, lib, module, moduleResolution, noEmit, noFallthroughCasesInSwitch (+15 more)

### Community 15 - "Message Card Components"
Cohesion: 0.17
Nodes (19): Avatar(), LevelChip(), MessageCard(), Props, SourceCardItem(), AVATAR_GRADIENTS, avatarGradient(), formatSyncedAgo() (+11 more)

### Community 16 - "Telegram Source Tests"
Cohesion: 0.25
Nodes (17): botAPI(), Client, makeUpdate(), newIngestor(), TestGroupChatShowsMemberCount(), TestGroupWithoutMemberCountDegradesGracefully(), TestInvalidTokenSwitchesToReauth(), TestMessageWithoutTextIsSkipped() (+9 more)

### Community 17 - "Sources Feed & HTTP API"
Cohesion: 0.14
Nodes (14): Лента, уровень 1 — источники, HTTP API, Option, Props, Segmented(), errorMessage(), request(), tzOffsetMinutes() (+6 more)

### Community 18 - "Importance Level UI"
Cohesion: 0.23
Nodes (13): LEVEL_OPTIONS, Props, Props, LEVEL_BG, LEVEL_COLOR, LEVEL_ICON, LEVEL_INK, LEVEL_LABEL (+5 more)

### Community 19 - "Avatar & Portal Components"
Cohesion: 0.24
Nodes (9): Props, Portal(), SOURCE_GRADIENT, SOURCE_LETTER, Connection, SourceKind, ConnectionsScreen(), Props (+1 more)

### Community 20 - "Ingest Pipeline Tests"
Cohesion: 0.47
Nodes (9): incoming(), newConnection(), newIngestor(), TestDuplicateExternalIDIsSkipped(), TestSameExternalIDInOtherConnectionIsStored(), TestStoreCreatesProcessingMessage(), TestStorePublishesCreatedEvent(), TestStoreWithoutAnalysis() (+1 more)

### Community 21 - "User Store"
Cohesion: 0.43
Nodes (3): DB, User, scanUser()

### Community 22 - "Sources Spec & Connection Screen"
Cohesion: 0.40
Nodes (5): Источник Gmail (ТЗ), Источник Telegram (ТЗ), Экран Источники, Таблица connection, Решение №4: Telegram только Bot API

### Community 23 - "MVP Scope Boundaries"
Cohesion: 0.67
Nodes (3): Границы: чего в MVP нет (CLAUDE.md), Не входит в MVP (ТЗ, п.8), Не входит в MVP (дизайн-спека)

## Knowledge Gaps
- **174 isolated node(s):** `$schema`, `Read`, `Glob`, `Grep`, `Bash(ls:*)` (+169 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **38 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Экран Сводка` connect `Project Rules & Spec Overview` to `Sources Feed & HTTP API`?**
  _High betweenness centrality (0.199) - this node is a cross-community bridge._
- **Why does `HTTP API` connect `Sources Feed & HTTP API` to `Project Rules & Spec Overview`, `Message Card Components`?**
  _High betweenness centrality (0.198) - this node is a cross-community bridge._
- **Are the 73 inferred relationships involving `newEnv()` (e.g. with `TestForgedCookieIsRejected()` and `TestHealthIsPublic()`) actually correct?**
  _`newEnv()` has 73 INFERRED edges - model-reasoned connections that need verification._
- **Are the 8 inferred relationships involving `UTCNow()` (e.g. with `PeriodStart()` and `SummaryPeriodStart()`) actually correct?**
  _`UTCNow()` has 8 INFERRED edges - model-reasoned connections that need verification._
- **What connects `$schema`, `Read`, `Glob` to the rest of the system?**
  _174 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Auth & API Test Suite` be split into smaller, more focused modules?**
  _Cohesion score 0.05292929292929293 - nodes in this community are weakly interconnected._
- **Should `API Request Handlers` be split into smaller, more focused modules?**
  _Cohesion score 0.05506329113924051 - nodes in this community are weakly interconnected._