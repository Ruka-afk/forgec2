/**
 * Canonical frontend API paths.
 *
 * Prefer `/api/...` or `/api/v1/...` when the backend exposes a dedicated JSON API.
 * Dual-use page routes (SPA + JSON via Accept) stay bare — do not invent `/api/*`.
 * Agent command/detail routes live under `/agents/:id/*` (not `/api/agents/:id/*`).
 */

export const paths = {
  agents: {
    /** JSON list — dedicated REST under /api */
    list: (query = "") => (query ? `/api/agents?${query}` : "/api/agents"),
    unlinked: "/api/agents/unlinked",
    /** Batch ops (auth group, not under /api/agents) */
    batch: "/agents/batch",
    bulkTask: "/agents/bulk/task",
    batchDelete: "/agents/batch/delete",
    batchTags: "/agents/batch/tags",
    bulkResults: (query = "page=1&pageSize=10") => `/agents/bulk/results?${query}`,
    /** Dual-use agent detail + commands */
    one: (id: string) => `/agents/${id}`,
    tasks: (id: string) => `/agents/${id}/tasks`,
    task: (id: string, taskId: string | number) => `/agents/${id}/tasks/${taskId}`,
    inject: (id: string) => `/agents/${id}/inject`,
    injectMethods: (id: string) => `/agents/${id}/inject/methods`,
    timeline: (id: string, query = "") => `/api/agents/${id}/timeline${query ? `?${query}` : ""}`,
    cancelTask: (id: string, taskId: string | number) => `/agents/${id}/tasks/${taskId}/cancel`,
    rerunTask: (id: string, taskId: string | number) => `/agents/${id}/task/${taskId}/rerun`,
    kill: (id: string) => `/agents/${id}/kill`,
    migrate: (id: string) => `/agents/${id}/migrate`,
    uninstall: (id: string) => `/agents/${id}/uninstall`,
    note: (id: string) => `/agents/${id}/note`,
    diagnose: (id: string) => `/agents/${id}/diagnose`,
    setSleep: (id: string) => `/agents/${id}/set_sleep`,
    killDate: (id: string) => `/agents/${id}/kill_date`,
    command: (id: string) => `/agents/${id}/command`,
    screenshots: (id: string, query = "") =>
      query ? `/api/agents/${id}/screenshots?${query}` : `/api/agents/${id}/screenshots`,
    processes: (id: string) => `/api/agents/${id}/processes`,
    processTree: (id: string) => `/api/agents/${id}/process-tree`,
    /** GET current frame (json). Pass "" for no query string. */
    screenshot: (id: string, query: string | null = "format=json") =>
      query ? `/agents/${id}/screenshot?${query}` : `/agents/${id}/screenshot`,
    /** POST queue screenshot task */
    screenshotTask: (id: string) => `/agents/${id}/screenshot`,
    screenshotWindow: (id: string) => `/agents/${id}/screenshot_window`,
    screenStart: (id: string) => `/agents/${id}/screen/start`,
    screenStop: (id: string) => `/agents/${id}/screen/stop`,
    remoteInput: (id: string) => `/api/agents/${id}/input`,
    /** Generic `/agents/:id/...` builder (suffix may omit leading /). */
    cmd: (id: string, suffix: string) => {
      const s = suffix.startsWith("/") ? suffix : `/${suffix}`;
      return `/agents/${id}${s}`;
    },
    filesLs: (id: string) => `/agents/${id}/files/ls`,
    filesRead: (id: string) => `/agents/${id}/files/read`,
    filesDelete: (id: string) => `/agents/${id}/files/delete`,
    /** Queue implant→server exfil (path only — do not attach a file). */
    filesExfil: (id: string) => `/agents/${id}/files/pull`,
    filesExfilGet: (id: string, filename: string) =>
      `/agents/${id}/files/exfil/${encodeURIComponent(filename)}`,
    /** Push a teamserver file onto the implant. */
    filesPush: (id: string) => `/agents/${id}/files/push`,
    /** Agent fetches a URL onto its own disk. */
    download: (id: string) => `/agents/${id}/download`,
    drives: (id: string) => `/agents/${id}/drives`,
    find: (id: string) => `/agents/${id}/find`,
    config: (id: string) => `/agents/${id}/config`,
    persistence: (id: string) => `/agents/${id}/persistence`,
    runEvasion: (id: string) => `/agents/${id}/run_evasion`,
    profileRotate: (id: string) => `/agents/${id}/profile-rotate`,
    modulesDeploy: (id: string) => `/agents/${id}/modules/deploy`,
    trafficProfile: (id: string) => `/agents/${id}/traffic-profile`,
    trafficAdapt: (id: string) => `/agents/${id}/traffic-profile/adapt`,
    trafficAutoAdapt: (id: string) => `/agents/${id}/traffic-profile/auto-adapt`,
    rportfwdStatus: (id: string) => `/agents/${id}/rportfwd/status`,
    rportfwdStart: (id: string) => `/agents/${id}/rportfwd/start`,
    rportfwdStop: (id: string) => `/agents/${id}/rportfwd/stop`,
    socks: (id: string) => `/agents/${id}/socks`,
    socksRelayStart: (id: string) => `/agents/${id}/socks_relay/start`,
    socksRelayStop: (id: string) => `/agents/${id}/socks_relay/stop`,
    socksRelayStatus: (id: string) => `/agents/${id}/socks_relay/status`,
    cookieExport: (id: string) => `/agents/${id}/cookie_export`,
    cookieProxyStart: (id: string) => `/agents/${id}/cookie_proxy/start`,
    cookieProxyStop: (id: string) => `/agents/${id}/cookie_proxy/stop`,
    cookieProxy: (id: string) => `/agents/${id}/cookie_proxy`,
    cookieProxyJar: (id: string) => `/agents/${id}/cookie_proxy/jar`,
    cookieProxyNetscape: (id: string) => `/agents/${id}/cookie_proxy/netscape`,
    sccmRecon: (id: string) => `/agents/${id}/sccm_recon`,
    entraPrt: (id: string) => `/agents/${id}/entra_prt`,
    fileHunt: (id: string) => `/agents/${id}/file_hunt`,
    screenTriggerStart: (id: string) => `/agents/${id}/screen_trigger/start`,
    screenTriggerStop: (id: string) => `/agents/${id}/screen_trigger/stop`,
    usbEnum: (id: string) => `/agents/${id}/usb_enum`,
    usbDrop: (id: string) => `/agents/${id}/usb_drop`,
    browserHistory: (id: string) => `/agents/${id}/browser_history`,
    sessionRecon: (id: string) => `/agents/${id}/session_recon`,
    tunStart: (id: string) => `/agents/${id}/tun/start`,
    tunStop: (id: string) => `/agents/${id}/tun/stop`,
    tun: (id: string) => `/agents/${id}/tun`,
    chain: (id: string) => `/agents/${id}/chain`,
    chainSet: (id: string) => `/agents/${id}/chain/set`,
    chainClear: (id: string) => `/agents/${id}/chain/clear`,
    tokenList: (id: string) => `/agents/${id}/token/list?format=json`,
    tokenListProcs: (id: string) => `/agents/${id}/token/list_procs`,
    tokenSteal: (id: string) => `/agents/${id}/token/steal`,
    tokenMake: (id: string) => `/agents/${id}/token/make`,
    tokenRevert: (id: string) => `/agents/${id}/token/revert`,
    tokenWhoami: (id: string) => `/agents/${id}/token/whoami`,
    tokenOne: (id: string, tokenId: string | number) => `/agents/${id}/token/${tokenId}`,
    tokenImpersonate: (id: string, tokenId: string | number) =>
      `/agents/${id}/token/${tokenId}/impersonate`,
    tokenNote: (id: string, tokenId: string | number) => `/agents/${id}/token/${tokenId}/note`,
    coerce: (id: string, coerceType: string) => `/agents/${id}/coerce/${coerceType}`,
    relayStart: (id: string) => `/agents/${id}/relay/start`,
    relayStop: (id: string) => `/agents/${id}/relay/stop`,
  },
  listeners: {
    list: "/api/listeners",
    one: (id: string) => `/api/listeners/${id}`,
    enable: (id: string) => `/api/listeners/${id}/enable`,
    disable: (id: string) => `/api/listeners/${id}/disable`,
  },
  credentials: {
    /** Vault + agents/agents-scoped lists come from the dual-use /credentials
     *  page route (JSON via Accept, snake aliases like vault_entries). */
    list: (query = "format=json") => (query ? `/credentials?${query}` : "/credentials"),
    /** Agent-scoped lookup goes through the dual-use /credentials page route,
     *  which supports agent_id filtering and returns a `total` count. */
    byAgent: (agentId: string, limit = 1) =>
      `/credentials?agent_id=${encodeURIComponent(agentId)}&limit=${limit}`,
    /** CRUD mutations stay on dual-use /credentials/* (not under /api) */
    add: "/credentials/add",
    one: (id: string | number) => `/credentials/${id}`,
    confirm: (id: string | number) => `/credentials/${id}/confirm`,
    usage: (id: string | number) => `/credentials/${id}/usage`,
    batchTags: "/credentials/batch/tags",
    batchVerify: "/credentials/batch/verify",
  },
  config: {
    reload: "/config/reload",
  },
  /**
   * Users page is dual-use at /users (renderPageOrJSON) — not /api/users.
   */
  users: {
    list: "/users",
    add: "/users/add",
    edit: (id: string) => `/users/${id}/edit`,
    toggle: (id: string) => `/users/${id}/toggle`,
    one: (id: string) => `/users/${id}`,
    password: (id: string) => `/users/${id}/password`,
    forceLogout: (id: string) => `/users/${id}/force-logout`,
    sessions: (id: string) => `/users/${id}/sessions`,
    revokeSession: (id: string, sessionId: string | number) => `/users/${id}/sessions/${sessionId}/revoke`,
    revokeAllSessions: (id: string) => `/users/${id}/sessions/revoke-all`,
  },
  /**
   * Notifications — /notifications only (no /api prefix).
   */
  notifications: {
    root: "/notifications",
    list: (query = "page=1&pageSize=20") => `/notifications?${query}`,
    one: (id: string | number) => `/notifications/${id}`,
    markRead: (id: string | number) => `/notifications/${id}/read`,
    readAll: "/notifications/read-all",
  },
  /**
   * Agent groups — /groups only.
   */
  groups: {
    list: "/groups",
    one: (id: string) => `/groups/${id}`,
  },
  /**
   * Paginated audit JSON is GET /audit/logs (not /api/audit/logs).
   * REST envelope also at /api/v1/audit with a different shape.
   */
  audit: {
    logs: (query: string) => `/audit/logs?${query}`,
  },
  /**
   * Build logs dual-use at /builds (renderPageOrJSON) — not /api/builds.
   */
  builds: {
    list: (query = "") => (query ? `/builds?${query}` : "/builds"),
    download: (id: string) => `/builds/${id}/download`,
    effectiveness: (days = 30) => `/api/builds/effectiveness?days=${days}`,
  },
  /**
   * Settings page dual-use at /settings.
   */
  settings: {
    root: "/settings",
    agent: "/settings/agent",
    server: "/settings/server",
    malleable: "/settings/malleable",
    password: "/settings/password",
    jwtRegenerate: "/settings/jwt/regenerate",
    dbVacuum: "/settings/db/vacuum",
    dbBackup: "/settings/db/backup",
    dbBackups: "/settings/db/backups",
    dbRestore: "/settings/db/restore",
    dbBackupsDownload: (name: string) =>
      `/settings/db/backups/download?name=${encodeURIComponent(name)}`,
    configDownload: "/settings/config/download",
    maintenancePurge: "/settings/maintenance/purge",
    purge: (type: string) => `/settings/purge/${type}`,
    certs: "/settings/certs",
    certsRegenerate: "/settings/certs/regenerate",
    certsUpload: "/settings/certs/upload",
    webhooks: "/settings/webhooks",
    webhooksTest: "/settings/webhooks/test",
    apiKeys: "/api/api-keys",
    apiKey: (id: string | number) => `/api/api-keys/${id}`,
    apiKeyRotate: (id: string | number) => `/api/api-keys/${id}/rotate`,
    savedViews: (page = "") => (page ? `/api/saved-views?page=${encodeURIComponent(page)}` : "/api/saved-views"),
    savedView: (id: string | number) => `/api/saved-views/${id}`,
    notificationRoutes: "/api/notification-routes",
    notificationRoute: (id: string | number) => `/api/notification-routes/${id}`,
    notificationRouteTest: (id: string | number) => `/api/notification-routes/${id}/test`,
    beaconKey: "/settings/beacon-key",
    totpStatus: "/settings/totp/status",
    totpGenerate: "/settings/totp/generate",
    totpEnable: "/settings/totp/enable",
    totpDisable: "/settings/totp/disable",
    killSwitch: "/admin/killswitch",
    killSwitchStatus: "/admin/killswitch/status",
  },
  generate: {
    profiles: "/api/generate/profiles",
    profileSave: "/api/generate/profile",
    profileImport: "/api/generate/profile/import",
    profileDelete: (name: string) => `/api/generate/profile/${encodeURIComponent(name)}`,
    buildStatus: (id: string | number) => `/generate/builds/${id}`,
    buildDownload: (id: string | number) => `/generate/builds/${id}/download`,
    delivery: "/generate/delivery",
    oneLiner: "/generate/one-liner",
  },
  macros: {
    list: "/api/macros",
    one: (id: string | number) => `/api/macros/${id}`,
    run: (id: string | number) => `/api/macros/${id}/run`,
    runs: (limit = 50) => `/api/macro-runs?limit=${limit}`,
    runOne: (id: string | number) => `/api/macro-runs/${id}`,
    stopRun: (id: string | number) => `/api/macro-runs/${id}/stop`,
  },
  identity: {
    deviceCode: "/api/identity/device-code",
    deviceCodePoll: (id: string) => `/api/identity/device-code/${id}/poll`,
    consent: "/api/identity/consent",
    consentStatus: (id: string) => `/api/identity/consent/${id}`,
    consentExchange: (id: string) => `/api/identity/consent/${id}/exchange`,
  },
  automation: {
    rules: "/api/automation/rules",
    rule: (id: string | number) => `/api/automation/rules/${id}`,
    ruleToggle: (id: string | number) => `/api/automation/rules/${id}/toggle`,
    webhooks: "/api/webhooks",
    webhook: (id: string | number) => `/api/webhooks/${id}`,
    webhookTest: "/api/webhooks/test",
    alertRules: "/api/monitor/alert-rules",
    alertRule: (id: string | number) => `/api/monitor/alert-rules/${id}`,
    alertAck: (id: string | number) => `/api/monitor/alerts/${id}/acknowledge`,
    alertResolve: (id: string | number) => `/api/monitor/alerts/${id}/resolve`,
  },
  monitor: {
    alerts: (query = "") => (query ? `/api/monitor/alerts?${query}` : "/api/monitor/alerts"),
  },
  plugins: {
    list: "/api/plugins",
    create: "/api/plugins",
    importJson: "/api/plugins/import?format=json",
    one: (id: string | number) => `/api/plugins/${id}`,
    export: (id: string | number) => `/api/plugins/${id}/export`,
    install: (id: string | number) => `/api/plugins/${id}/install`,
    toggle: (id: string | number) => `/api/plugins/${id}/toggle`,
    execute: (id: string | number) => `/api/plugins/${id}/execute`,
    update: (id: string | number) => `/api/plugins/${id}/update`,
    checkUpdates: "/api/plugins/check-updates",
    reviews: (id: string | number) => `/api/plugins/${id}/reviews`,
    dependencies: (id: string | number) => `/api/plugins/${id}/dependencies`,
    rating: (id: string | number) => `/api/plugins/${id}/rating`,
  },
  scheduler: {
    oneshotList: "/api/scheduler/oneshot",
    oneshot: (id: string | number) => `/api/scheduler/oneshot/${id}`,
  },
  listenersHealth: "/api/listeners/health",
  ai: {
    root: "/ai",
    config: "/ai/config",
    sessions: "/ai/sessions",
    session: (id: string | number) => `/ai/sessions/${id}`,
    sessionMessages: (id: string | number) => `/ai/sessions/${id}/messages`,
		sessionBranch: (id: string | number) => `/ai/sessions/${id}/branch`,
		runs: "/api/ai/runs",
		run: (id: string) => `/api/ai/runs/${id}`,
		runEvents: (id: string) => `/api/ai/runs/${id}/events`,
		runCancel: (id: string) => `/api/ai/runs/${id}/cancel`,
		profiles: "/api/ai/profiles",
		intents: "/api/ai/intents",
		intentApprove: (id: string) => `/api/ai/intents/${id}/approve`,
		intentReject: (id: string) => `/api/ai/intents/${id}/reject`,
		knowledgeCollections: "/api/ai/knowledge/collections",
		knowledgeSearch: "/api/ai/knowledge/search",
		attachments: (sessionId: string | number) => `/api/ai/sessions/${sessionId}/attachments`,
		attachment: (id: string) => `/api/ai/attachments/${id}`,
    pendingTasks: "/api/ai/pending-tasks",
    status: "/api/ai/status",
    analyzeResult: "/api/ai/analyze-result",
    suggestNextSteps: "/api/ai/suggest-next-steps",
    nlQuery: "/api/ai/nl-query",
    generatePlaybook: "/api/ai/generate-playbook",
    savePlaybook: "/api/ai/save-playbook",
  },
  bof: {
    list: "/api/bof/list",
    results: "/api/bof/results",
    repos: "/api/bof/repos",
    upload: "/api/bof/upload?format=json",
    one: (id: string | number) => `/api/bof/${id}`,
    run: (id: string | number) => `/api/bof/${id}/run`,
    edit: (id: string | number) => `/api/bof/${id}/edit`,
    reposImport: "/api/bof/repos/import",
    reposRate: (id: string | number) => `/api/bof/repos/${id}/rate`,
  },
  bloodhound: {
    list: "/bloodhound/list",
    upload: "/bloodhound/upload",
    collect: "/bloodhound/collect",
    one: (id: string | number) => `/bloodhound/${id}`,
    download: (id: string | number) => `/bloodhound/${id}/download`,
  },
  chrome: {
    agents: "/api/chrome/agents",
    agentTasks: (agentId: string) => `/chrome/agents/${agentId}/tasks`,
  },
  chat: {
    channels: "/chat/channels",
    send: "/chat/send",
    history: (channel: string) => `/chat/history?channel=${encodeURIComponent(channel)}`,
  },
  circuitBreaker: {
    detail: "/circuit-breaker/detail",
    config: "/circuit-breaker/config",
    events: "/circuit-breaker/events",
    reset: (listenerId: string | number) => `/circuit-breaker/reset/${listenerId}`,
    toggle: (listenerId: string | number) => `/circuit-breaker/toggle/${listenerId}`,
  },
  autotag: {
    rules: "/api/autotag/rules",
    rule: (id: string | number) => `/api/autotag/rules/${id}`,
    toggle: (id: string | number) => `/api/autotag/rules/${id}/toggle`,
    apply: "/api/autotag/apply",
  },
  domainFront: {
    list: "/infra/front/list",
    check: "/infra/front/check",
    config: "/infra/front/config",
  },
  mesh: {
    route: (agentId: string) => `/mesh/route/${agentId}`,
  },
  stager: {
    one: (id: string | number) => `/stager/${id}`,
  },
  modules: {
    list: "/api/modules",
    one: (name: string) => `/api/modules/${encodeURIComponent(name)}`,
  },
  /**
   * Report: overview is dual-use /report; section APIs under /api/report/*.
   */
  report: {
    overview: "/report",
    agents: (query = "") => (query ? `/api/report/agents?${query}` : "/api/report/agents"),
    tasks: (query = "") => (query ? `/api/report/tasks?${query}` : "/api/report/tasks"),
    credentials: (query = "") =>
      query ? `/api/report/credentials?${query}` : "/api/report/credentials",
    network: (query = "") => (query ? `/api/report/network?${query}` : "/api/report/network"),
    findings: (query = "") => (query ? `/api/report/findings?${query}` : "/api/report/findings"),
    history: "/api/report/history",
    generated: (id: string | number) => `/api/report/generated/${id}`,
    download: (id: string | number, format = "html") => `/report/${encodeURIComponent(String(id))}/download?format=${encodeURIComponent(format)}`,
    generate: "/api/report/generate",
    one: (id: string) => `/api/report/${id}`,
    exportHtml: (query: string) => `/api/report/export/html?${query}`,
    iocs: (query = "days=30") => (query ? `/api/ioc?${query}` : "/api/ioc"),
    iocExport: (format: "stix2" | "csv", days = 30) => `/api/ioc/export?format=${format}&days=${days}`,
    handoverExport: (days = 30) => `/api/handover/export?days=${days}`,
  },
  dashboard: {
    /** REST under /api/v1 group */
    v1: "/api/v1/dashboard",
    agentGeo: "/api/dashboard/agent-geo",
    attackPath: "/api/dashboard/attack-path",
    credentialTypes: "/api/dashboard/credential-types",
    activityHeatmap: "/api/dashboard/activity-heatmap",
    osDistribution: "/api/dashboard/os-distribution",
    taskStatus: "/api/dashboard/task-status",
    taskGantt: (range: string) => `/api/dashboard/task-gantt?range=${range}`,
    listenerTraffic: (range: string) => `/api/dashboard/listener-traffic?range=${range}`,
    activeMissions: "/api/dashboard/active-missions",
  },
  /**
   * DUAL-USE: GET /loot — SPA vs JSON Accept. Not /api/loot.
   */
  loot: {
    page: "/loot",
    bulkDelete: "/loot/bulk-delete",
  },
  campaigns: {
    list: "/campaigns",
    one: (id: string) => `/campaigns/${id}`,
    mitre: (id: string) => `/campaigns/${id}/mitre`,
    killchain: (id: string) => `/campaigns/${id}/killchain`,
  },
  mitre: {
    templates: "/mitre/templates",
    phases: "/mitre/phases",
    timeline: (query = "") => (query ? `/mitre/timeline?${query}` : "/mitre/timeline"),
    heatmap: (range = "30d") => `/api/mitre/heatmap?range=${range}`,
  },
  v1: {
    tasks: "/api/v1/tasks",
    taskTypes: "/api/v1/task-types",
  },
  dns: {
    status: "/api/dns/status",
    start: "/api/dns/start",
    stop: "/api/dns/stop",
  },
  timeline: {
    export: "/api/timeline/export",
    data: (query = "") => (query ? `/api/timeline/data?${query}` : "/api/timeline/data"),
  },
  topology: {
    data: "/api/topology/data",
    network: "/api/topology/network",
  },
  workflows: {
    list: "/workflows",
    one: (id: string) => `/workflows/${id}`,
  },
  /** Pivot control-plane lists (not agent-cmd) */
  socks: {
    sessions: "/socks/sessions",
  },
  rportfwd: {
    status: "/rportfwd/status",
  },
  tokens: {
    list: "/tokens",
  },
  tags: {
    list: "/api/tags",
    one: (id: string | number) => `/api/tags/${id}`,
  },
  tasks: {
    list: (query = "") => (query ? `/tasks?${query}` : "/tasks"),
    /** Full single-task record (includes complete result) under REST /api/v1. */
    one: (id: string | number) => `/api/v1/tasks/${id}`,
  },
  opsec: {
    history: "/opsec/history",
    rules: "/opsec/rules",
    rulesApi: "/api/opsec/rules",
    check: "/api/opsec/check",
    rekey: "/api/opsec/rekey",
    rule: (name: string) => `/opsec/rules/${encodeURIComponent(name)}`,
  },
  privesc: {
    page: "/privesc",
    run: "/api/privesc/run",
    history: (id: string) => `/api/privesc/history/${id}`,
    execute: "/api/privesc/execute",
  },
  lateral: {
    historyAll: "/api/lateral/history/all",
    execute: "/api/lateral/execute",
  },
  scripts: {
    list: "/api/scripts",
    history: "/api/scripts/history",
    execute: "/api/scripts/execute",
    one: (id: string) => `/api/scripts/${id}`,
  },
  templates: {
    list: "/api/templates",
    one: (id: string) => `/api/templates/${id}`,
  },
  roles: {
    list: "/api/roles",
    one: (id: string | number) => `/api/roles/${id}`,
  },
  scanner: {
    page: "/scanner",
    scan: "/api/scan",
  },
  toolkit: {
    results: "/toolkit/results",
    action: (agentId: string) => `/toolkit/agents/${agentId}/action`,
  },
  traffic: {
    page: "/traffic",
  },
  tasksCollab: {
    approve: (id: string | number) => `/tasks/${id}/approve`,
    reject: (id: string | number) => `/tasks/${id}/reject`,
    claim: (id: string | number) => `/collab/tasks/${id}/claim`,
    release: (id: string | number) => `/collab/tasks/${id}/release`,
  },
  extc2: {
    configs: "/extc2/configs",
    config: (id: string | number) => `/extc2/configs/${id}`,
  },
  siem: {
    rules: "/siem/rules",
    rule: (id: string | number) => `/siem/rules/${id}`,
    toggle: (id: string | number) => `/siem/rules/${id}/toggle`,
  },
  updateCheck: "/api/update-check",
  redirectors: {
    list: "/redirectors",
    one: (id: string | number) => `/redirectors/${id}`,
    testSsh: "/redirectors/test-ssh",
  },
  infrastructure: {
    acmeProvision: "/infrastructure/acme/provision",
    profileExport: (format: string) => `/infrastructure/profile/export?format=${encodeURIComponent(format)}`,
  },
  integrations: {
    list: "/integrations",
    one: (id: string | number) => `/integrations/${id}`,
    toggle: (id: string | number) => `/integrations/${id}/toggle`,
    malleable: "/integrations/malleable",
  },
  phishing: {
    templates: "/phishing/templates",
    template: (id: string | number) => `/phishing/templates/${id}`,
    campaigns: "/phishing/campaigns",
    campaign: (id: string | number) => `/phishing/campaigns/${id}`,
    campaignStop: (id: string | number) => `/phishing/campaigns/${id}/stop`,
    captures: "/phishing/captures",
  },
  cloud: {
    steal: "/cloud/steal",
  },
  auth: {
    login: "/login",
    logout: "/logout",
    health: "/health",
    me: "/api/me",
    extend: "/api/auth/extend",
  },
} as const;

/** Paths intentionally not under /api (dual-use or agent-cmd layout). */
export const DUAL_USE_PREFIXES = [
  "/loot",
  "/agents/",
  "/users",
  "/builds",
  "/settings",
  "/notifications",
  "/groups",
  "/audit",
  "/campaigns",
  "/workflows",
] as const;
