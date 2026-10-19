# Terminology & Naming — ForgeC2 UI

Status: **U7 waves 1–3 landed, body-copy sweep at its second domain (2026-10-19).** This file is the contract for what the
console calls things — in English, in Chinese, in the sidebar, in the page titles, in
the address bar, and in the body copy the operator actually reads — and for the
per-page emoji identity.
`frontend/src/lib/i18n/en.ts` / `zh.ts` remain the runtime source; this document is
what the source is checked against.

## Core vocabulary

| Concept | UI word (EN) | UI word (ZH) | Never in chrome |
| --- | --- | --- | --- |
| A connected implant | **Beacon** | **Beacon** | Session, Agent, Implant |
| Server-side C2 endpoint | **Listener** | **监听器** | — |
| Generated artifact | **Payload** | **载荷** | Implant (as a noun for the file) |
| One dispatched command | **Task** | **任务** | Job |
| Callback / task / alert stream | **Event** | **事件** | Feed, Timeline |
| Collected artifacts | **Loot** | **战利品** | Screenshot Collection |
| A human console user | **Operator** | **操作员** | User (except user *management*) |

The backend contract is out of scope: the HTTP API, sqlite schema and
`api/openapi.yaml` keep saying `agents` — including the dual-use `/agents/:id/…`
endpoints the SPA still calls — and UI code keeps `agents.*` i18n keys. Client-side
URLs say **beacons** since wave 2 (`/beacons/:id/…`), and only code that mirrors the
API keeps the agent word (`agentId`, `lib/api-paths.ts`, detail sub-components). The
vocabulary above governs **operator-facing copy and URLs**.

## Naming rules

1. **A nav label equals its page title (h1).** The two exceptions are deliberate:
   nav `Dashboard` vs title `Operations Dashboard`, and nav `Proxy Chains` vs the
   chain workspace card `Multi-Hop Proxy Chain`.
2. **Professional terms beat abbreviations** in chrome: *Payload Generator*,
   *Privilege Escalation*, *Post-Exploitation Toolkit*, *User Management*,
   *Asset Management*, *NTLM Relay & Coercion*, *Network Topology*.
3. **i18n keys are immutable.** A rename changes values only; keys such as
   `nav.beacons`, `agents.title`, `loot.page_title` keep their historical names.
   Both language files change in the same commit (the 4727-key parity gate enforces it).
4. **Chinese mirrors meaning, not word order.** e.g. `Privilege Escalation` →
   「权限提升」, `Token Store` → 「令牌仓库」, `Loot` → 「战利品」.
5. **The entity a sentence is about decides the noun.** A *running* implant is a
   beacon ("Waiting for the implant to return this result" → "Waiting for the
   beacon…"); the *generated artifact* is a payload ("Generate Implant" →
   **Generate Payload**, "Deploy your first implant" → "…your first payload"); the
   periodic contact is a **check-in** in English and 回连 in Chinese ("Agent will
   exit on next beacon" → "The beacon will exit on its next check-in"), never
   "next beacon" *as an event*. Chinese says **Beacon** for the running implant
   (代理 / 植入体 / 会话 retire from that sense).
6. **A banned synonym is banned for the beacon sense, not as a word.** "Session"
   stays where it names something real: the `User-Agent` HTTP header (`用户代理`),
   the AMSI-session technique, host logon/network sessions (recon category
   `Sessions`), the Windows logon session (`Logon Create`), remote-desktop
   sessions, the console session lifetime in the media panel, and WeChat 会话
   (*chat*). Technique and platform names (`AMSI Session Bypass`) keep their own
   terminology.
7. **Non-beacon nouns stay literal.** A *format descriptor* names the format, not
   the entity: `GUI implant` → **GUI executable** / GUI 可执行文件, beside
   `sideload` / `script one-liner` / `ELF · amd64/arm`. And where the sentence is
   about the code or the build, a subjectless clause beats reintroducing a banned
   noun: "After implant changes, regenerate the payload" → "Regenerate the payload
   after code changes"; "Not available on macOS implants" → "…in macOS builds".

## Emoji identity

One emoji per page, used **identically** in the sidebar label, the page h1, the
breadcrumb and `document.title` (all four surfaces read the same i18n value).

Rules:

- Decoration only — never the sole carrier of meaning; the label always stands alone.
- Windows-safe single codepoints: no ZWJ sequences, no flag emoji.
- Inside chips/icon slots the emoji is `aria-hidden` so it stays out of accessible names.
- Adding a page means adding exactly one catalog row below **and** the matching h1 key.

### Catalog (nav label = page title)

| Key | Emoji | EN | ZH |
| --- | --- | --- | --- |
| nav.dashboard | 📊 | Dashboard | 作战仪表盘 |
| nav.beacons | 🐝 | Beacons | Beacon 管理 |
| nav.listeners | 📡 | Listeners | 监听器管理 |
| nav.events | 🔔 | Events | 事件 |
| nav.generate | 📦 | Payload Generator | 载荷生成器 |
| nav.infrastructure | 🏗️ | Infrastructure | 基础设施 |
| nav.dns | 🌐 | DNS C2 | DNS C2 |
| nav.domain_fronting | 🎭 | Domain Fronting | 域前置 |
| nav.automation | 🤖 | Automation | 自动化 |
| nav.macros | 🎬 | Task Macros | 任务宏 |
| nav.bof | 📚 | BOF Library | BOF 库 |
| nav.plugins | 🧩 | Plugins | 插件 |
| nav.opsec | 🛡️ | OPSEC Guard | OPSEC 守卫 |
| nav.lateral | 🔀 | Lateral Movement | 横向移动 |
| nav.privesc | ⬆️ | Privilege Escalation | 权限提升 |
| nav.pivoting | 🚇 | Pivoting | 跳板 |
| nav.token_store | 🎫 | Token Store | 令牌仓库 |
| nav.scanner | 🛰️ | Network Scanner | 网络扫描器 |
| nav.scripting | 📜 | Script Console | 脚本控制台 |
| nav.toolkit | 🧰 | Post-Exploitation Toolkit | 后渗透工具包 |
| nav.loot | 💰 | Loot | 战利品 |
| nav.credentials | 🔑 | Credentials | 凭据管理 |
| nav.audit | 📋 | Audit Log | 审计日志 |
| nav.traffic | 🚦 | Traffic Log | 流量日志 |
| nav.report | 📝 | Report | 报告 |
| nav.asset_organization | 🗂️ | Asset Management | 资产管理 |
| nav.ai | 🧠 | AI Assistant | AI 助手 |
| nav.integrations | 🔌 | Integrations | 集成中心 |
| nav.campaign | 🎯 | Campaigns | 战役管理 |
| nav.attack | 🗺️ | MITRE ATT&CK Coverage | MITRE ATT&CK 覆盖率 |
| nav.bloodhound | 🐕 | BloodHound | BloodHound |
| nav.chat | 💬 | Operator Chat | 操作员聊天 |
| nav.phishing | 🎣 | Phishing | 钓鱼攻击 |
| nav.circuit_breaker | 🧯 | Circuit Breaker | 断路器 |
| nav.cloud | ☁️ | Cloud Credential Theft | 云凭据窃取 |
| nav.ntlm | 🥷 | NTLM Relay & Coercion | NTLM 中继与强制认证 |
| nav.topology | 🕸️ | Network Topology | 网络拓扑 |
| nav.chain | ⛓️ | Proxy Chains | 代理链 |
| nav.settings | ⚙️ | Settings | 系统设置 |
| nav.users | 👥 | User Management | 用户管理 |
| nav.roles | 🎖️ | Roles | 角色 |
| nav.tags | 🏷️ | Tags | 标签管理 |
| nav.groups | 📁 | Beacon Groups | Beacon 分组 |
| nav.autotag | ✨ | Auto-Tag Rules | 自动标签规则 |
| nav.files | 📄 | File Browser | 文件浏览器 |

Pages without a nav entry reuse the emoji of the family they belong to (agent detail
tabs, settings sections). Tiles and panels stay plain: the emoji marks a **page
identity**, not every heading.

## Scope & roadmap

- **L1 — copy (wave 1: chrome, wave 3: body copy per page domain):** nav, page
  titles, dashboard tiles and the subtitles the rename touched landed in wave 1.
  Body copy — per-page descriptions, toasts, empty states — is swept one page
  domain per wave; the **Beacons domain** landed first (100 flagged values: 87
  rewritten per language, 13 kept under rule 6) with the shared status banner, then
  the **Payload domain** — payload generator, build records, profiles — (33 flagged:
  24 rewritten, 5 kept, 4 deferred to the automation card). Domains still open:
  dashboard, events/tasks, automation, listeners, settings, users,
  credentials/NTLM/cloud, pivoting/topology/chains, AI, and the long tail (see the
  batch map in `docs/UIUX_AUDIT.md`).
- **L2 — frontend structure (done, wave 2):** route URLs and page directory/component
  names follow the vocabulary — `/agents` → `/beacons`, `/timeline` → `/events`,
  `/generate` → `/payload-generator`, `/privesc` → `/privilege-escalation`,
  `/chain` → `/proxy-chains`, `/asset-organization` → `/asset-management`, and the
  `/agents/:id/…` detail family → `/beacons/:id/…`. Old URLs redirect client-side
  (table below). Backend paths stay as they are.
- **L3 — contract (out of scope):** HTTP API, DB schema, OpenAPI, generated clients.

### Route renames (L2, landed wave 2)

| Old URL | New URL | Notes |
| --- | --- | --- |
| `/agents`, `/agents/:id` + its 8 tabs | `/beacons`, `/beacons/:id/…` | tab segments keep their names: `config`, `files`, `persistence`, `remote-desktop`, `screen`, `shell`, `token`, `traffic` |
| `/timeline` | `/events` | `?tab=tasks`-style state survives the hop |
| `/generate` | `/payload-generator` | |
| `/privesc` | `/privilege-escalation` | |
| `/chain` | `/proxy-chains` | |
| `/asset-organization` | `/asset-management` | |

Redirects live in `frontend/src/router.tsx` (`LEGACY_REDIRECTS`, rendered by
`LegacyPrefixRedirect`) and rewrite the first path segment only, so the id,
sub-route, query string and hash are preserved — `/agents?status=offline` lands on
`/beacons?status=offline`. The old shells stay listed in
`frontend/vite-plugins/staticExport.ts` (pinned by `static-export-routes.test.ts`) so
a bookmarked URL boots the SPA before the router rewrites it.

**Boundary.** The rename stops at the URL, the page directory and the route-entry
component: anything that mirrors the backend contract keeps the `agent` word
(`agentId`, `lib/api-paths.ts`, the dual-use `/agents/:id/…` API endpoints, detail
sub-components such as `AgentHeader` / `AgentStatusBar` / `AgentInteractDock`). The
API is the contract; the URL is the label.

## Changing a name

1. Edit the value in **both** `en.ts` and `zh.ts` (same key, both languages).
2. Keep nav label and h1 in sync (rule 1), and keep the catalog above accurate.
3. `npm run check` (i18n parity + tokens + bundle) → unit tests → rebuild the static
   export → regenerate the win32 visual baselines locally with
   `--update-snapshots=all` (plain `--update-snapshots` skips sub-tolerance drift) and
   the linux ones through `visual-baselines.yml` after pushing.
