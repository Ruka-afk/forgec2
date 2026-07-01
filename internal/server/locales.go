package server

import (
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
	"sync"
)

type LanguageInfo struct {
	Code      string
	Name      string
	NativeName string
	RTL       bool
	Flag      string
}

var SupportedLanguages = map[string]LanguageInfo{
	"en":    {Code: "en", Name: "English", NativeName: "English", RTL: false, Flag: "🇺🇸"},
	"zh":    {Code: "zh", Name: "Chinese", NativeName: "中文", RTL: false, Flag: "🇨🇳"},
	"ja":    {Code: "ja", Name: "Japanese", NativeName: "日本語", RTL: false, Flag: "🇯🇵"},
	"ko":    {Code: "ko", Name: "Korean", NativeName: "한국어", RTL: false, Flag: "🇰🇷"},
	"ar":    {Code: "ar", Name: "Arabic", NativeName: "العربية", RTL: true, Flag: "🇸🇦"},
}

var DefaultLanguage = "en"

type TranslationMap map[string]string

var translations = map[string]TranslationMap{
	"en": enTranslations,
	"zh": zhTranslations,
	"ja": jaTranslations,
	"ko": koTranslations,
	"ar": arTranslations,
}

var enTranslations = TranslationMap{
	"common.save": "Save",
	"common.cancel": "Cancel",
	"common.delete": "Delete",
	"common.edit": "Edit",
	"common.create": "Create",
	"common.search": "Search",
	"common.loading": "Loading...",
	"common.error": "Error",
	"common.success": "Success",
	"common.confirm": "Confirm",
	"common.close": "Close",
	"common.submit": "Submit",
	"common.refresh": "Refresh",
	"common.back": "Back",
	"common.next": "Next",
	"common.previous": "Previous",
	"common.actions": "Actions",
	"common.status": "Status",
	"common.name": "Name",
	"common.type": "Type",
	"common.time": "Time",
	"common.details": "Details",
	"common.yes": "Yes",
	"common.no": "No",
	"common.enabled": "Enabled",
	"common.disabled": "Disabled",
	"common.active": "Active",
	"common.inactive": "Inactive",
	"common.online": "Online",
	"common.offline": "Offline",
	"common.all": "All",
	"common.none": "None",
	"common.copy": "Copy",
	"common.download": "Download",
	"common.upload": "Upload",
	"common.settings": "Settings",
	"common.profile": "Profile",
	"common.logout": "Logout",
	"common.login": "Login",
	"common.username": "Username",
	"common.password": "Password",
	"common.remember_me": "Remember me",
	"common.forgot_password": "Forgot password?",
	"common.welcome": "Welcome",
	"common.goodbye": "Goodbye",

	"nav.dashboard": "Dashboard",
	"nav.topology": "Topology",
	"nav.implants": "Implants",
	"nav.generate": "Generate",
	"nav.listeners": "Listeners",
	"nav.builds": "Build Logs",
	"nav.ai_assistant": "AI Assistant",
	"nav.toolkit": "Toolkit",
	"nav.scanner": "Scanner",
	"nav.lateral_movement": "Lateral Movement",
	"nav.credentials": "Credentials",
	"nav.tunneling": "Tunneling",
	"nav.task_history": "Task History",
	"nav.privilege_escalation": "Privilege Escalation",
	"nav.bof": "BOF",
	"nav.automation": "Automation",
	"nav.tokens": "Tokens",
	"nav.data_collection": "Data Collection",
	"nav.timeline": "Timeline",
	"nav.templates": "Templates",
	"nav.traffic": "Traffic",
	"nav.report": "Report",
	"nav.chat": "Chat",
	"nav.users": "Users",
	"nav.audit": "Audit",
	"nav.settings": "Settings",
	"nav.infrastructure": "Infrastructure",
	"nav.operations": "Operations",
	"nav.build_deploy": "Build & Deploy",
	"nav.post_exploit": "Post-Exploit",
	"nav.system": "System",
	"nav.quick_actions": "Quick Actions",
	"nav.generate_implant": "Generate Implant",
	"nav.new_listener": "New Listener",
	"nav.more": "More",
	"nav.refresh": "Refresh",
	"nav.loot": "Loot",
	"nav.online_users": "Online Users",

	"notifications.user": "User",
	"chat.operations": "Operations Chat",
	"chat.connecting": "Connecting...",
	"chat.type_message": "Type a message...",
	"chat.send": "Send",

	"collab.no_online_users": "No users online",
	"collab.you": "you",
	"collab.load_failed": "Unable to load online users",

	"ai.title": "AI Assistant",
	"ai.settings": "Settings",
	"ai.clear": "Clear",
	"ai.export": "Export",
	"ai.ready": "Ready",
	"ai.disabled": "Not configured",
	"ai.disabled_hint": "Configure API Key in settings to enable",
	"ai.provider_label": "Provider",
	"ai.model_label": "Model",
	"ai.welcome": "Hello! I'm the ForgeC2 AI assistant.",
	"ai.welcome_hint": "I can list online implants, view targets, run commands, check credentials, manage listeners, and more.",
	"ai.input_placeholder": "Describe what you need...",
	"ai.reasoning": "Reasoning",
	"ai.thinking": "AI is thinking...",
	"ai.tool_running": "Running tool",
	"ai.tool_call": "Tool call",
	"ai.retry": "Retry",
	"ai.copy": "Copy",
	"ai.regenerate": "Regenerate",
	"ai.copied": "Copied",
	"ai.code_copied": "Code copied",
	"ai.chat_exported": "Chat exported",
	"ai.clear_confirm": "Clear all chat history?",
	"ai.config_saved": "Config saved, refresh to apply",
	"ai.you": "You",
	"ai.stopped": "Stopped",
	"ai.interrupted": "Generation interrupted (left page). Click Retry to continue.",
	"ai.quick_agents": "Online Implants",
	"ai.quick_listeners": "Listeners",
	"ai.quick_credentials": "Credentials",
	"ai.quick_operators": "Online Operators",
	"ai.prompt_redteam": "You are ForgeC2 red team assistant. List online agents, view target details, execute commands, manage credentials and listeners.",
	"ai.prompt_concise": "Security analysis assistant. Answer concisely with bullet points. Prioritize actionable steps.",
	"ai.prompt_verbose": "Senior penetration testing expert. Provide detailed technical analysis using MITRE ATT&CK terminology. Output structured reports.",
	"ai.prompt_social": "Social engineering specialist. Design phishing emails and social engineering scripts. Format: Scenario -> Script -> Notes.",

	"update.available": "New version available",
	"update.download": "Download Update",
	"task.details": "Task Details",
	"task.rerun": "Rerun",
	"task.command": "Command",
	"task.result": "Result",
	"task.error": "Error",
	"task.close": "Close",
	"nav.overview": "Overview",
	"nav.listen_short": "Listen",

	"settings.title": "Settings",
	"settings.profile": "Profile",
	"settings.appearance": "Appearance",
	"settings.security": "Security",
	"settings.shortcuts": "Shortcuts",
	"settings.notifications": "Notifications",
	"settings.server": "Server",
	"settings.agent": "Agent",
	"settings.jwt": "JWT Secret",
	"settings.malleable": "Malleable C2",
	"settings.database": "Database",
	"settings.maintenance": "Maintenance",
	"settings.config": "Config",
	"settings.about": "System Info",
	"settings.language": "Language",
	"settings.language_desc": "Choose your preferred language",

	"login.title": "Login",
	"login.username_placeholder": "Enter username",
	"login.password_placeholder": "Enter password",
	"login.totp_placeholder": "Enter 2FA code",
	"login.totp_required": "Two-factor authentication required",
	"login.invalid_credentials": "Invalid username or password",
	"login.account_disabled": "Account is disabled",
	"login.too_many_attempts": "Too many login attempts. Please try again later.",

	"login.remember_me": "Remember me",
	"login.forgot_password": "Forgot password?",
	"login.operator_login": "Operator Login",
	"login.subtitle": "Sign in to your command & control console",
	"login.access_console": "Access Console",
	"login.totp_label": "Two-Factor Code (Optional)",
	"login.authorized_only": "Authorized operators only. All access is audited.",

	"search.title": "Global Search",
	"search.subtitle": "Search across implants, listeners, credentials and more",
	"search.placeholder": "Search agents, listeners, credentials…",
	"search.hint": "Type to search implants, listeners, credentials, BOFs, users and tasks",
	"search.no_results": "No results found",
	"search.type.agent": "Implant",
	"search.type.listener": "Listener",
	"search.type.credential": "Credential",
	"search.type.bof": "BOF",
	"search.type.user": "User",
	"search.type.task": "Task",

	"traffic.title": "Traffic Monitoring",
	"traffic.subtitle": "Live view of Agent Beacon HTTP requests",
	"traffic.auto_refresh": "Auto Refresh",
	"traffic.beacon_requests": "Beacon Requests",
	"traffic.latest": "Latest",

	"dashboard.title": "Dashboard",
	"dashboard.overview": "Overview",
	"dashboard.total_agents": "Total Agents",
	"dashboard.online_agents": "Online Agents",
	"dashboard.total_listeners": "Total Listeners",
	"dashboard.pending_tasks": "Pending Tasks",
	"dashboard.completed_tasks": "Completed Tasks",
	"dashboard.failed_tasks": "Failed Tasks",

	"agents.title": "Implants",
	"agents.hostname": "Hostname",
	"agents.ip": "IP Address",
	"agents.os": "Operating System",
	"agents.arch": "Architecture",
	"agents.user": "User",
	"agents.pid": "PID",
	"agents.last_seen": "Last Seen",
	"agents.interval": "Interval",
	"agents.jitter": "Jitter",
	"agents.note": "Note",
	"agents.online": "Online",
	"agents.offline": "Offline",
	"agents.dead": "Dead",

	"listeners.title": "Listeners",
	"listeners.create": "Create Listener",
	"listeners.name": "Listener Name",
	"listeners.protocol": "Protocol",
	"listeners.host": "Host",
	"listeners.port": "Port",
	"listeners.status": "Status",
	"listeners.start": "Start",
	"listeners.stop": "Stop",
	"listeners.delete": "Delete",
	"listeners.running": "Running",
	"listeners.stopped": "Stopped",
	"listeners.error": "Error",

	"tasks.title": "Tasks",
	"tasks.command": "Command",
	"tasks.status": "Status",
	"tasks.result": "Result",
	"tasks.error": "Error",
	"tasks.pending": "Pending",
	"tasks.running": "Running",
	"tasks.completed": "Completed",
	"tasks.failed": "Failed",
	"tasks.created_at": "Created At",
	"tasks.completed_at": "Completed At",
	"tasks.rerun": "Rerun",
	"tasks.details": "Task Details",

	"notifications.title": "Notifications",
	"notifications.mark_all_read": "Mark all as read",
	"notifications.clear_all": "Clear all",
	"notifications.all": "All",
	"notifications.agent": "Agent",
	"notifications.task": "Task",
	"notifications.system": "System",
	"notifications.settings": "Notification Settings",
	"notifications.master_switch": "Master Switch",
	"notifications.desktop": "Desktop Notifications",
	"notifications.sound": "Sound",

	"audit.title": "Security Audit",
	"audit.action": "Action",
	"audit.user": "User",
	"audit.resource": "Resource",
	"audit.details": "Details",
	"audit.time": "Time",
	"audit.success": "Success",
	"audit.failure": "Failure",
	"audit.export": "Export",

	"builds.title": "Build Logs",
	"builds.subtitle": "Implant build history",
	"builds.refresh": "Refresh",
	"builds.records": "Build Records",
	"builds.total": "Total",
	"builds.time": "Time",
	"builds.platform": "Platform",
	"builds.format": "Format",
	"builds.c2_url": "C2 URL",
	"builds.filename": "Filename",
	"builds.operator": "Operator",
	"builds.status": "Status",
	"builds.details": "Details",
	"builds.success": "Success",
	"builds.failed": "Failed",
	"builds.empty": "No build records yet",
	"builds.empty_hint": "Generate an implant to see build history here",
	"builds.go_generate": "Go to Generate",
	"builds.filter": "Filter",
	"builds.all_status": "All statuses",
	"builds.all_platform": "All platforms",
	"builds.previous": "Previous",
	"builds.next": "Next",

	"users.title": "Users",
	"users.create": "Create User",
	"users.username": "Username",
	"users.role": "Role",
	"users.status": "Status",
	"users.last_login": "Last Login",
	"users.created_at": "Created At",
	"users.admin": "Admin",
	"users.operator": "Operator",
	"users.viewer": "Viewer",

	"theme.light": "Light Mode",
	"theme.dark": "Dark Mode",
	"theme.system": "System Default",
	"theme.light_desc": "Clean and bright interface",
	"theme.dark_desc": "Eye-friendly dark interface",
	"theme.system_desc": "Use system theme settings",

	"security.change_password": "Change Password",
	"security.current_password": "Current Password",
	"security.new_password": "New Password",
	"security.confirm_password": "Confirm Password",
	"security.two_factor": "Two-Factor Authentication (2FA)",
	"security.totp_desc": "Use TOTP app for second verification",
	"security.enable_2fa": "Enable 2FA",
	"security.disable_2fa": "Disable 2FA",
	"security.scan_qr": "Scan QR code or enter key manually",
	"security.manual_key": "Manual Key",
	"security.copy_key": "Copy Key",
	"security.verification_code": "Verification Code",
	"security.backup_codes": "Backup Codes",
	"security.backup_codes_warning": "Please save these backup codes for recovery when you cannot access the authenticator app",

	"server.http_port": "HTTP Port",
	"server.listen_address": "Listen Address",
	"server.log_level": "Log Level",
	"server.tls": "TLS",
	"server.tcp_transport": "TCP Transport",
	"server.tcp_address": "TCP Address",
	"server.offline_threshold": "Offline Threshold (seconds)",
	"server.session_timeout": "Session Timeout (hours)",
	"server.auto_cleanup": "Auto Cleanup (days)",
	"server.save_config": "Save Server Config",
	"server.restart_required": "Some changes require server restart to take effect",

	"agent.interval": "Heartbeat Interval (seconds)",
	"agent.jitter": "Jitter (%)",
	"agent.skip_tls": "Skip TLS Verification",
	"agent.default_ua": "Default User-Agent",
	"agent.save_config": "Save Agent Config",

	"database.size": "Database Size",
	"database.path": "Database Path",
	"database.agents": "Agents",
	"database.listeners": "Listeners",
	"database.credentials": "Credentials",
	"database.tokens": "Tokens",
	"database.socks": "SOCKS",
	"database.audit_logs": "Audit Logs",
	"database.purge_tasks": "Purge Old Tasks",
	"database.purge_audit": "Purge Audit Logs",
	"database.vacuum": "VACUUM",
	"database.backup": "Backup",
	"database.download_config": "Download Config",

	"maintenance.purge_old_tasks": "Purge Old Tasks",
	"maintenance.purge_old_tasks_desc": "Delete completed/failed tasks older than specified days",
	"maintenance.purge_audit_logs": "Purge Audit Logs",
	"maintenance.purge_audit_logs_desc": "Delete operation records older than specified days",
	"maintenance.days": "days",
	"maintenance.purge": "Purge",
	"maintenance.vacuum": "Database VACUUM",
	"maintenance.vacuum_desc": "Reclaim unused space and optimize database",
	"maintenance.backup_database": "Backup Database",
	"maintenance.backup_desc": "Create a backup copy of the database",

	"shortcuts.title": "Keyboard Shortcuts",
	"shortcuts.desc": "Customize global shortcuts, saved locally",
	"shortcuts.reset_default": "Reset to Default",
	"shortcuts.save": "Save Settings",

	"time.just_now": "just now",
	"time.minute_ago": "1 min ago",
	"time.minutes_ago": "%d mins ago",
	"time.hour_ago": "1 hour ago",
	"time.hours_ago": "%d hours ago",
	"time.day_ago": "1 day ago",
	"time.days_ago": "%d days ago",

	"unit.bytes": "B",
	"unit.kb": "KB",
	"unit.mb": "MB",
	"unit.gb": "GB",
	"unit.tb": "TB",

	"shell.regen_hint": "Agent heartbeat interval differs from server default. Regenerate the implant on the Generate page to apply the new interval.",
	"generate.regen_hint": "Changing the heartbeat interval requires regenerating the implant for it to take effect.",
}

var zhTranslations = TranslationMap{
	"common.save": "保存",
	"common.cancel": "取消",
	"common.delete": "删除",
	"common.edit": "编辑",
	"common.create": "创建",
	"common.search": "搜索",
	"common.loading": "加载中...",
	"common.error": "错误",
	"common.success": "成功",
	"common.confirm": "确认",
	"common.close": "关闭",
	"common.submit": "提交",
	"common.refresh": "刷新",
	"common.back": "返回",
	"common.next": "下一步",
	"common.previous": "上一步",
	"common.actions": "操作",
	"common.status": "状态",
	"common.name": "名称",
	"common.type": "类型",
	"common.time": "时间",
	"common.details": "详情",
	"common.yes": "是",
	"common.no": "否",
	"common.enabled": "已启用",
	"common.disabled": "已禁用",
	"common.active": "活跃",
	"common.inactive": "不活跃",
	"common.online": "在线",
	"common.offline": "离线",
	"common.all": "全部",
	"common.none": "无",
	"common.copy": "复制",
	"common.download": "下载",
	"common.upload": "上传",
	"common.settings": "设置",
	"common.profile": "个人资料",
	"common.logout": "退出",
	"common.login": "登录",
	"common.username": "用户名",
	"common.password": "密码",
	"common.remember_me": "记住我",
	"common.forgot_password": "忘记密码？",
	"common.welcome": "欢迎",
	"common.goodbye": "再见",

	"nav.dashboard": "概览",
	"nav.topology": "拓扑",
	"nav.implants": "Implant",
	"nav.generate": "生成",
	"nav.listeners": "监听器",
	"nav.builds": "构建日志",
	"nav.ai_assistant": "AI 助手",
	"nav.toolkit": "工具箱",
	"nav.scanner": "扫描",
	"nav.lateral_movement": "横向移动",
	"nav.credentials": "凭据",
	"nav.tunneling": "隧道",
	"nav.task_history": "任务历史",
	"nav.privilege_escalation": "提权",
	"nav.bof": "BOF",
	"nav.automation": "自动化",
	"nav.tokens": "Token",
	"nav.data_collection": "数据收集",
	"nav.timeline": "时间线",
	"nav.templates": "模板",
	"nav.traffic": "流量",
	"nav.report": "报告",
	"nav.chat": "聊天",
	"nav.users": "用户",
	"nav.audit": "审计",
	"nav.settings": "设置",
	"nav.infrastructure": "基础设施",
	"nav.operations": "作战",
	"nav.build_deploy": "构建与部署",
	"nav.post_exploit": "后渗透",
	"nav.system": "系统",
	"nav.quick_actions": "快捷操作",
	"nav.generate_implant": "生成 Implant",
	"nav.new_listener": "新监听器",
	"nav.more": "更多",
	"nav.refresh": "刷新",
	"nav.loot": "战利品",
	"nav.online_users": "在线用户",

	"notifications.user": "用户",
	"chat.operations": "作战聊天",
	"chat.connecting": "连接中...",
	"chat.type_message": "输入消息...",
	"chat.send": "发送",

	"collab.no_online_users": "暂无在线用户",
	"collab.you": "你",
	"collab.load_failed": "在线用户加载失败",

	"ai.title": "AI 智能助手",
	"ai.settings": "设置",
	"ai.clear": "清除",
	"ai.export": "导出",
	"ai.ready": "已就绪",
	"ai.disabled": "未配置",
	"ai.disabled_hint": "请在设置中配置 API Key 后启用",
	"ai.provider_label": "提供商",
	"ai.model_label": "模型",
	"ai.welcome": "你好！我是 ForgeC2 AI 助手。",
	"ai.welcome_hint": "我可以帮你：列出在线 Implant、查看目标详情、执行命令、查看凭据、管理监听器等。",
	"ai.input_placeholder": "输入你的需求...",
	"ai.reasoning": "推理过程",
	"ai.thinking": "AI 正在思考...",
	"ai.tool_running": "正在执行工具",
	"ai.tool_call": "工具调用",
	"ai.retry": "重试",
	"ai.copy": "复制",
	"ai.regenerate": "重新生成",
	"ai.copied": "已复制",
	"ai.code_copied": "代码已复制",
	"ai.chat_exported": "对话已导出",
	"ai.clear_confirm": "确定清除所有对话记录？",
	"ai.config_saved": "配置已保存，刷新后生效",
	"ai.you": "你",
	"ai.stopped": "已停止",
	"ai.interrupted": "生成已中断（已离开页面），可点击重试继续",
	"ai.quick_agents": "在线 Implant",
	"ai.quick_listeners": "监听器",
	"ai.quick_credentials": "凭据摘要",
	"ai.quick_operators": "在线操作员",
	"ai.prompt_redteam": "你是 ForgeC2 红队行动助手。可以列出在线 Implant、查看目标详情、执行命令、管理凭据和监听器。",
	"ai.prompt_concise": "安全分析助手。用要点简洁回答，优先给出可执行步骤。",
	"ai.prompt_verbose": "资深渗透测试专家。使用 MITRE ATT&CK 术语提供详细技术分析，输出结构化报告。",
	"ai.prompt_social": "社会工程专家。设计钓鱼邮件和社会工程脚本。格式：场景 -> 脚本 -> 注意事项。",

	"update.available": "有新版本可用",
	"update.download": "下载更新",
	"task.details": "任务详情",
	"task.rerun": "重新执行",
	"task.command": "命令",
	"task.result": "结果",
	"task.error": "错误",
	"task.close": "关闭",
	"nav.overview": "概览",
	"nav.listen_short": "监听",

	"settings.title": "设置",
	"settings.profile": "个人资料",
	"settings.appearance": "外观主题",
	"settings.security": "安全",
	"settings.shortcuts": "快捷键",
	"settings.notifications": "通知设置",
	"settings.server": "服务器",
	"settings.agent": "Agent",
	"settings.jwt": "JWT 密钥",
	"settings.malleable": "Malleable C2",
	"settings.database": "数据库",
	"settings.maintenance": "维护",
	"settings.config": "配置",
	"settings.about": "系统信息",
	"settings.language": "语言",
	"settings.language_desc": "选择您偏好的语言",

	"login.title": "登录",
	"login.username_placeholder": "请输入用户名",
	"login.password_placeholder": "请输入密码",
	"login.totp_placeholder": "请输入2FA验证码",
	"login.totp_required": "需要双因素认证",
	"login.invalid_credentials": "用户名或密码错误",
	"login.account_disabled": "账户已被禁用",
	"login.too_many_attempts": "登录尝试次数过多，请稍后再试。",
	"login.remember_me": "记住我",
	"login.forgot_password": "忘记密码？",
	"login.operator_login": "操作员登录",
	"login.subtitle": "登录您的指挥控制控制台",
	"login.access_console": "进入控制台",
	"login.totp_label": "双因素验证码（可选）",
	"login.authorized_only": "仅限授权操作员，所有访问均被审计。",

	"search.title": "全局搜索",
	"search.subtitle": "搜索 Implant、监听器、凭据等",
	"search.placeholder": "搜索 Agent、监听器、凭据…",
	"search.hint": "输入关键词搜索 Implant、监听器、凭据、BOF、用户和任务",
	"search.no_results": "未找到结果",
	"search.type.agent": "Implant",
	"search.type.listener": "监听器",
	"search.type.credential": "凭据",
	"search.type.bof": "BOF",
	"search.type.user": "用户",
	"search.type.task": "任务",

	"traffic.title": "流量监控",
	"traffic.subtitle": "实时查看 Agent Beacon HTTP 请求",
	"traffic.auto_refresh": "自动刷新",
	"traffic.beacon_requests": "Beacon 请求",
	"traffic.latest": "最近",

	"dashboard.title": "概览",
	"dashboard.overview": "总览",
	"dashboard.total_agents": "Agent 总数",
	"dashboard.online_agents": "在线 Agent",
	"dashboard.total_listeners": "监听器总数",
	"dashboard.pending_tasks": "等待中任务",
	"dashboard.completed_tasks": "已完成任务",
	"dashboard.failed_tasks": "失败任务",

	"agents.title": "Implant",
	"agents.hostname": "主机名",
	"agents.ip": "IP 地址",
	"agents.os": "操作系统",
	"agents.arch": "架构",
	"agents.user": "用户",
	"agents.pid": "进程ID",
	"agents.last_seen": "最后心跳",
	"agents.interval": "间隔",
	"agents.jitter": "抖动",
	"agents.note": "备注",
	"agents.online": "在线",
	"agents.offline": "离线",
	"agents.dead": "死亡",

	"listeners.title": "监听器",
	"listeners.create": "创建监听器",
	"listeners.name": "监听器名称",
	"listeners.protocol": "协议",
	"listeners.host": "主机",
	"listeners.port": "端口",
	"listeners.status": "状态",
	"listeners.start": "启动",
	"listeners.stop": "停止",
	"listeners.delete": "删除",
	"listeners.running": "运行中",
	"listeners.stopped": "已停止",
	"listeners.error": "错误",

	"tasks.title": "任务",
	"tasks.command": "命令",
	"tasks.status": "状态",
	"tasks.result": "结果",
	"tasks.error": "错误",
	"tasks.pending": "等待中",
	"tasks.running": "运行中",
	"tasks.completed": "已完成",
	"tasks.failed": "失败",
	"tasks.created_at": "创建时间",
	"tasks.completed_at": "完成时间",
	"tasks.rerun": "重跑",
	"tasks.details": "任务详情",

	"notifications.title": "通知中心",
	"notifications.mark_all_read": "全部已读",
	"notifications.clear_all": "清空所有",
	"notifications.all": "全部",
	"notifications.agent": "Agent",
	"notifications.task": "任务",
	"notifications.system": "系统",
	"notifications.settings": "通知设置",
	"notifications.master_switch": "通知总开关",
	"notifications.desktop": "桌面通知",
	"notifications.sound": "提示音",

	"audit.title": "安全审计",
	"audit.action": "操作",
	"audit.user": "用户",
	"audit.resource": "资源",
	"audit.details": "详情",
	"audit.time": "时间",
	"audit.success": "成功",
	"audit.failure": "失败",
	"audit.export": "导出",

	"builds.title": "构建日志",
	"builds.subtitle": "Implant 构建历史",
	"builds.refresh": "刷新",
	"builds.records": "构建记录",
	"builds.total": "共",
	"builds.time": "时间",
	"builds.platform": "平台",
	"builds.format": "格式",
	"builds.c2_url": "C2 地址",
	"builds.filename": "文件名",
	"builds.operator": "操作员",
	"builds.status": "状态",
	"builds.details": "详情",
	"builds.success": "成功",
	"builds.failed": "失败",
	"builds.empty": "暂无构建记录",
	"builds.empty_hint": "前往生成页面构建 Implant 后，记录将显示在这里",
	"builds.go_generate": "前往生成",
	"builds.filter": "筛选",
	"builds.all_status": "全部状态",
	"builds.all_platform": "全部平台",
	"builds.previous": "上一页",
	"builds.next": "下一页",

	"users.title": "用户",
	"users.create": "创建用户",
	"users.username": "用户名",
	"users.role": "角色",
	"users.status": "状态",
	"users.last_login": "最后登录",
	"users.created_at": "创建时间",
	"users.admin": "管理员",
	"users.operator": "操作员",
	"users.viewer": "查看者",

	"theme.light": "亮色模式",
	"theme.dark": "暗色模式",
	"theme.system": "跟随系统",
	"theme.light_desc": "清爽明亮的界面",
	"theme.dark_desc": "护眼的深色界面",
	"theme.system_desc": "使用系统主题设置",

	"security.change_password": "修改密码",
	"security.current_password": "当前密码",
	"security.new_password": "新密码",
	"security.confirm_password": "确认新密码",
	"security.two_factor": "双因素认证 (2FA)",
	"security.totp_desc": "使用 TOTP 应用进行二次验证",
	"security.enable_2fa": "启用双因素认证",
	"security.disable_2fa": "禁用双因素认证",
	"security.scan_qr": "扫描二维码或手动输入密钥",
	"security.manual_key": "手动密钥",
	"security.copy_key": "复制密钥",
	"security.verification_code": "验证码",
	"security.backup_codes": "备份码",
	"security.backup_codes_warning": "请保存以下备份码，用于在无法访问认证器应用时恢复访问",

	"server.http_port": "HTTP 端口",
	"server.listen_address": "监听地址",
	"server.log_level": "日志级别",
	"server.tls": "TLS",
	"server.tcp_transport": "TCP 传输",
	"server.tcp_address": "TCP 地址",
	"server.offline_threshold": "离线阈值 (秒)",
	"server.session_timeout": "会话超时 (小时)",
	"server.auto_cleanup": "自动清理 (天)",
	"server.save_config": "保存服务器配置",
	"server.restart_required": "部分修改需要重启服务器生效",

	"agent.interval": "心跳间隔 (秒)",
	"agent.jitter": "抖动值 (%)",
	"agent.skip_tls": "跳过 TLS 验证",
	"agent.default_ua": "默认 User-Agent",
	"agent.save_config": "保存 Agent 配置",

	"database.size": "数据库大小",
	"database.path": "数据库路径",
	"database.agents": "Agent",
	"database.listeners": "监听器",
	"database.credentials": "凭据",
	"database.tokens": "Token",
	"database.socks": "SOCKS",
	"database.audit_logs": "审计日志",
	"database.purge_tasks": "清理旧任务",
	"database.purge_audit": "清理审计日志",
	"database.vacuum": "VACUUM",
	"database.backup": "备份",
	"database.download_config": "下载配置",

	"maintenance.purge_old_tasks": "清理旧任务",
	"maintenance.purge_old_tasks_desc": "删除超过指定天数的已完成/失败任务",
	"maintenance.purge_audit_logs": "清理审计日志",
	"maintenance.purge_audit_logs_desc": "删除超过指定天数的操作记录",
	"maintenance.days": "天",
	"maintenance.purge": "清理",
	"maintenance.vacuum": "数据库 VACUUM",
	"maintenance.vacuum_desc": "回收未使用空间并优化数据库",
	"maintenance.backup_database": "备份数据库",
	"maintenance.backup_desc": "创建数据库的备份副本",

	"shortcuts.title": "快捷键设置",
	"shortcuts.desc": "自定义全局快捷键，设置保存在本地",
	"shortcuts.reset_default": "恢复默认",
	"shortcuts.save": "保存设置",

	"time.just_now": "刚刚",
	"time.minute_ago": "1分钟前",
	"time.minutes_ago": "%d分钟前",
	"time.hour_ago": "1小时前",
	"time.hours_ago": "%d小时前",
	"time.day_ago": "1天前",
	"time.days_ago": "%d天前",

	"unit.bytes": "B",
	"unit.kb": "KB",
	"unit.mb": "MB",
	"unit.gb": "GB",
	"unit.tb": "TB",

	"shell.regen_hint": "植入程序心跳间隔与服务器默认设置不一致。请在「生成」页面重新生成 Implant 以应用新间隔。",
	"generate.regen_hint": "更改心跳间隔后需重新生成 Implant 才能生效。",
}

var jaTranslations = TranslationMap{
	"common.save": "保存",
	"common.cancel": "キャンセル",
	"common.delete": "削除",
	"common.edit": "編集",
	"common.create": "作成",
	"common.search": "検索",
	"common.loading": "読み込み中...",
	"common.error": "エラー",
	"common.success": "成功",
	"common.confirm": "確認",
	"common.close": "閉じる",
	"common.submit": "送信",
	"common.refresh": "更新",
	"common.back": "戻る",
	"common.next": "次へ",
	"common.previous": "前へ",
	"common.actions": "アクション",
	"common.status": "ステータス",
	"common.name": "名前",
	"common.type": "タイプ",
	"common.time": "時間",
	"common.details": "詳細",
	"common.yes": "はい",
	"common.no": "いいえ",
	"common.enabled": "有効",
	"common.disabled": "無効",
	"common.online": "オンライン",
	"common.offline": "オフライン",
	"common.all": "すべて",
	"common.none": "なし",
	"common.copy": "コピー",
	"common.download": "ダウンロード",
	"common.settings": "設定",
	"common.profile": "プロフィール",
	"common.logout": "ログアウト",
	"common.login": "ログイン",
	"common.username": "ユーザー名",
	"common.password": "パスワード",
	"common.remember_me": "ログイン状態を保持",

	"nav.dashboard": "ダッシュボード",
	"nav.topology": "トポロジー",
	"nav.implants": "インプラント",
	"nav.generate": "生成",
	"nav.listeners": "リスナー",
	"nav.builds": "ビルドログ",
	"nav.settings": "設定",
	"nav.users": "ユーザー",
	"nav.audit": "監査",

	"settings.title": "設定",
	"settings.profile": "プロフィール",
	"settings.appearance": "外観",
	"settings.security": "セキュリティ",
	"settings.language": "言語",
	"settings.language_desc": "希望の言語を選択してください",

	"login.title": "ログイン",
	"login.username_placeholder": "ユーザー名を入力",
	"login.password_placeholder": "パスワードを入力",
	"login.invalid_credentials": "ユーザー名またはパスワードが無効です",

	"dashboard.title": "ダッシュボード",
	"dashboard.total_agents": "総エージェント数",
	"dashboard.online_agents": "オンラインエージェント",
	"dashboard.pending_tasks": "保留中のタスク",

	"agents.title": "インプラント",
	"agents.hostname": "ホスト名",
	"agents.ip": "IPアドレス",
	"agents.os": "OS",
	"agents.last_seen": "最終確認",
	"agents.online": "オンライン",
	"agents.offline": "オフライン",

	"listeners.title": "リスナー",
	"listeners.create": "リスナーを作成",
	"listeners.name": "リスナー名",
	"listeners.protocol": "プロトコル",
	"listeners.port": "ポート",
	"listeners.status": "ステータス",
	"listeners.running": "実行中",
	"listeners.stopped": "停止中",

	"tasks.title": "タスク",
	"tasks.command": "コマンド",
	"tasks.status": "ステータス",
	"tasks.pending": "保留中",
	"tasks.completed": "完了",
	"tasks.failed": "失敗",

	"notifications.title": "通知",
	"notifications.all": "すべて",
	"notifications.user": "ユーザー",
	"notifications.agent": "エージェント",
	"notifications.task": "タスク",
	"notifications.system": "システム",
	"notifications.settings": "通知設定",
	"chat.operations": "オペレーションチャット",
	"chat.connecting": "接続中...",
	"chat.type_message": "メッセージを入力...",
	"update.available": "新しいバージョンがあります",
	"update.download": "更新をダウンロード",
	"task.details": "タスク詳細",
	"task.rerun": "再実行",
	"task.command": "コマンド",
	"task.result": "結果",
	"task.error": "エラー",
	"task.close": "閉じる",
	"nav.overview": "概要",
	"nav.listen_short": "Listen",
	"nav.more": "その他",
	"nav.refresh": "更新",
	"nav.loot": "ルート",
	"nav.online_users": "オンラインユーザー",

	"users.title": "ユーザー",
	"users.username": "ユーザー名",
	"users.role": "役割",
	"users.status": "ステータス",

	"theme.light": "ライトモード",
	"theme.dark": "ダークモード",
	"theme.system": "システム設定",

	"security.change_password": "パスワードを変更",
	"security.current_password": "現在のパスワード",
	"security.new_password": "新しいパスワード",
	"security.confirm_password": "パスワードを確認",
	"security.two_factor": "二要素認証 (2FA)",

	"server.log_level": "ログレベル",
	"server.save_config": "サーバー設定を保存",

	"agent.interval": "ハートビート間隔 (秒)",
	"agent.jitter": "ジッター (%)",
	"agent.save_config": "エージェント設定を保存",

	"database.size": "データベースサイズ",
	"database.agents": "エージェント",
	"database.listeners": "リスナー",

	"time.just_now": "たった今",
	"time.minutes_ago": "%d分前",
	"time.hours_ago": "%d時間前",
	"time.days_ago": "%d日前",

	"shell.regen_hint": "Agent heartbeat interval differs from server default. Regenerate the implant on the Generate page to apply the new interval.",
	"generate.regen_hint": "Changing the heartbeat interval requires regenerating the implant for it to take effect.",
}

var koTranslations = TranslationMap{
	"common.save": "저장",
	"common.cancel": "취소",
	"common.delete": "삭제",
	"common.edit": "편집",
	"common.create": "생성",
	"common.search": "검색",
	"common.loading": "로딩 중...",
	"common.error": "오류",
	"common.success": "성공",
	"common.confirm": "확인",
	"common.close": "닫기",
	"common.submit": "제출",
	"common.refresh": "새로고침",
	"common.back": "뒤로",
	"common.next": "다음",
	"common.previous": "이전",
	"common.actions": "작업",
	"common.status": "상태",
	"common.name": "이름",
	"common.type": "유형",
	"common.time": "시간",
	"common.details": "세부 정보",
	"common.yes": "예",
	"common.no": "아니오",
	"common.enabled": "활성화됨",
	"common.disabled": "비활성화됨",
	"common.online": "온라인",
	"common.offline": "오프라인",
	"common.all": "전체",
	"common.none": "없음",
	"common.copy": "복사",
	"common.download": "다운로드",
	"common.settings": "설정",
	"common.profile": "프로필",
	"common.logout": "로그아웃",
	"common.login": "로그인",
	"common.username": "사용자 이름",
	"common.password": "비밀번호",
	"common.remember_me": "로그인 유지",

	"nav.dashboard": "대시보드",
	"nav.topology": "토폴로지",
	"nav.implants": "임플란트",
	"nav.generate": "생성",
	"nav.listeners": "리스너",
	"nav.builds": "빌드 로그",
	"nav.settings": "설정",
	"nav.users": "사용자",
	"nav.audit": "감사",

	"settings.title": "설정",
	"settings.profile": "프로필",
	"settings.appearance": "외관",
	"settings.security": "보안",
	"settings.language": "언어",
	"settings.language_desc": "선호하는 언어를 선택하세요",

	"login.title": "로그인",
	"login.username_placeholder": "사용자 이름 입력",
	"login.password_placeholder": "비밀번호 입력",
	"login.invalid_credentials": "잘못된 사용자 이름 또는 비밀번호입니다",

	"dashboard.title": "대시보드",
	"dashboard.total_agents": "총 에이전트 수",
	"dashboard.online_agents": "온라인 에이전트",
	"dashboard.pending_tasks": "보류 중인 작업",

	"agents.title": "임플란트",
	"agents.hostname": "호스트명",
	"agents.ip": "IP 주소",
	"agents.os": "운영 체제",
	"agents.last_seen": "마지막 활동",
	"agents.online": "온라인",
	"agents.offline": "오프라인",

	"listeners.title": "리스너",
	"listeners.create": "리스너 생성",
	"listeners.name": "리스너 이름",
	"listeners.protocol": "프로토콜",
	"listeners.port": "포트",
	"listeners.status": "상태",
	"listeners.running": "실행 중",
	"listeners.stopped": "중지됨",

	"tasks.title": "작업",
	"tasks.command": "명령",
	"tasks.status": "상태",
	"tasks.pending": "보류 중",
	"tasks.completed": "완료됨",
	"tasks.failed": "실패",

	"notifications.title": "알림",
	"notifications.all": "전체",
	"notifications.user": "사용자",
	"notifications.agent": "에이전트",
	"notifications.task": "작업",
	"notifications.system": "시스템",
	"notifications.settings": "알림 설정",
	"chat.operations": "작전 채팅",
	"chat.connecting": "연결 중...",
	"chat.type_message": "메시지 입력...",
	"update.available": "새 버전 사용 가능",
	"update.download": "업데이트 다운로드",
	"task.details": "작업 상세",
	"task.rerun": "재실행",
	"task.command": "명령",
	"task.result": "결과",
	"task.error": "오류",
	"task.close": "닫기",
	"nav.overview": "개요",
	"nav.listen_short": "Listen",
	"nav.more": "더보기",
	"nav.refresh": "새로고침",
	"nav.loot": "루트",
	"nav.online_users": "온라인 사용자",

	"users.title": "사용자",
	"users.username": "사용자 이름",
	"users.role": "역할",
	"users.status": "상태",

	"theme.light": "라이트 모드",
	"theme.dark": "다크 모드",
	"theme.system": "시스템 설정",

	"security.change_password": "비밀번호 변경",
	"security.current_password": "현재 비밀번호",
	"security.new_password": "새 비밀번호",
	"security.confirm_password": "비밀번호 확인",
	"security.two_factor": "2단계 인증 (2FA)",

	"server.log_level": "로그 레벨",
	"server.save_config": "서버 설정 저장",

	"agent.interval": "하트비트 간격 (초)",
	"agent.jitter": "지터 (%)",
	"agent.save_config": "에이전트 설정 저장",

	"database.size": "데이터베이스 크기",
	"database.agents": "에이전트",
	"database.listeners": "리스너",

	"time.just_now": "방금 전",
	"time.minutes_ago": "%d분 전",
	"time.hours_ago": "%d시간 전",
	"time.days_ago": "%d일 전",

	"shell.regen_hint": "Agent heartbeat interval differs from server default. Regenerate the implant on the Generate page to apply the new interval.",
	"generate.regen_hint": "Changing the heartbeat interval requires regenerating the implant for it to take effect.",
}

var arTranslations = TranslationMap{
	"common.save": "حفظ",
	"common.cancel": "إلغاء",
	"common.delete": "حذف",
	"common.edit": "تعديل",
	"common.create": "إنشاء",
	"common.search": "بحث",
	"common.loading": "جارٍ التحميل...",
	"common.error": "خطأ",
	"common.success": "نجاح",
	"common.confirm": "تأكيد",
	"common.close": "إغلاق",
	"common.submit": "إرسال",
	"common.refresh": "تحديث",
	"common.back": "رجوع",
	"common.next": "التالي",
	"common.previous": "السابق",
	"common.actions": "إجراءات",
	"common.status": "الحالة",
	"common.name": "الاسم",
	"common.type": "النوع",
	"common.time": "الوقت",
	"common.details": "التفاصيل",
	"common.yes": "نعم",
	"common.no": "لا",
	"common.enabled": "ممكّن",
	"common.disabled": "معطّل",
	"common.online": "متصل",
	"common.offline": "غير متصل",
	"common.all": "الكل",
	"common.none": "لا شيء",
	"common.copy": "نسخ",
	"common.download": "تنزيل",
	"common.settings": "الإعدادات",
	"common.profile": "الملف الشخصي",
	"common.logout": "تسجيل الخروج",
	"common.login": "تسجيل الدخول",
	"common.username": "اسم المستخدم",
	"common.password": "كلمة المرور",
	"common.remember_me": "تذكرني",

	"nav.dashboard": "لوحة القيادة",
	"nav.topology": "الطوبولوجيا",
	"nav.implants": "العديدات",
	"nav.generate": "إنشاء",
	"nav.listeners": "المستمعون",
	"nav.builds": "سجلات البناء",
	"nav.settings": "الإعدادات",
	"nav.users": "المستخدمون",
	"nav.audit": "التدقيق",

	"settings.title": "الإعدادات",
	"settings.profile": "الملف الشخصي",
	"settings.appearance": "المظهر",
	"settings.security": "الأمان",
	"settings.language": "اللغة",
	"settings.language_desc": "اختر لغتك المفضلة",

	"login.title": "تسجيل الدخول",
	"login.username_placeholder": "أدخل اسم المستخدم",
	"login.password_placeholder": "أدخل كلمة المرور",
	"login.invalid_credentials": "اسم المستخدم أو كلمة المرور غير صحيحة",

	"dashboard.title": "لوحة القيادة",
	"dashboard.total_agents": "إجمالي الوكلاء",
	"dashboard.online_agents": "الوكلاء المتصلون",
	"dashboard.pending_tasks": "المهام المعلقة",

	"agents.title": "العديدات",
	"agents.hostname": "اسم المضيف",
	"agents.ip": "عنوان IP",
	"agents.os": "نظام التشغيل",
	"agents.last_seen": "آخر ظهور",
	"agents.online": "متصل",
	"agents.offline": "غير متصل",

	"listeners.title": "المستمعون",
	"listeners.create": "إنشاء مستمع",
	"listeners.name": "اسم المستمع",
	"listeners.protocol": "البروتوكول",
	"listeners.port": "المنفذ",
	"listeners.status": "الحالة",
	"listeners.running": "قيد التشغيل",
	"listeners.stopped": "متوقف",

	"tasks.title": "المهام",
	"tasks.command": "الأمر",
	"tasks.status": "الحالة",
	"tasks.pending": "معلق",
	"tasks.completed": "مكتمل",
	"tasks.failed": "فشل",

	"notifications.title": "الإشعارات",
	"notifications.all": "الكل",
	"notifications.user": "المستخدم",
	"notifications.agent": "الوكيل",
	"notifications.task": "المهمة",
	"notifications.system": "النظام",
	"notifications.settings": "إعدادات الإشعارات",
	"chat.operations": "دردشة العمليات",
	"chat.connecting": "جاري الاتصال...",
	"chat.type_message": "اكتب رسالة...",
	"update.available": "إصدار جديد متاح",
	"update.download": "تنزيل التحديث",
	"task.details": "تفاصيل المهمة",
	"task.rerun": "إعادة التشغيل",
	"task.command": "الأمر",
	"task.result": "النتيجة",
	"task.error": "خطأ",
	"task.close": "إغلاق",
	"nav.overview": "نظرة عامة",
	"nav.listen_short": "Listen",
	"nav.more": "المزيد",
	"nav.refresh": "تحديث",
	"nav.loot": "الغنائم",
	"nav.online_users": "المستخدمون المتصلون",

	"users.title": "المستخدمون",
	"users.username": "اسم المستخدم",
	"users.role": "الدور",
	"users.status": "الحالة",

	"theme.light": "الوضع الفاتح",
	"theme.dark": "الوضع الداكن",
	"theme.system": "إعدادات النظام",

	"security.change_password": "تغيير كلمة المرور",
	"security.current_password": "كلمة المرور الحالية",
	"security.new_password": "كلمة المرور الجديدة",
	"security.confirm_password": "تأكيد كلمة المرور",
	"security.two_factor": "المصادقة الثنائية (2FA)",

	"server.log_level": "مستوى السجل",
	"server.save_config": "حفظ إعدادات الخادم",

	"agent.interval": "فاصل النبض (ثواني)",
	"agent.jitter": "الاهتزاز (%)",
	"agent.save_config": "حفظ إعدادات الوكيل",

	"database.size": "حجم قاعدة البيانات",
	"database.agents": "الوكلاء",
	"database.listeners": "المستمعون",

	"time.just_now": "الآن",
	"time.minutes_ago": "منذ %d دقيقة",
	"time.hours_ago": "منذ %d ساعة",
	"time.days_ago": "منذ %d يوم",

	"shell.regen_hint": "Agent heartbeat interval differs from server default. Regenerate the implant on the Generate page to apply the new interval.",
	"generate.regen_hint": "Changing the heartbeat interval requires regenerating the implant for it to take effect.",
}

var (
	i18nMutex sync.RWMutex
)

func IsLanguageSupported(code string) bool {
	_, ok := SupportedLanguages[code]
	return ok
}

func GetLanguageInfo(code string) (LanguageInfo, bool) {
	info, ok := SupportedLanguages[code]
	return info, ok
}

func GetSupportedLanguages() map[string]LanguageInfo {
	return SupportedLanguages
}

// GetTranslationsJSON returns all translations for a language as JSON for client-side __().
func GetTranslationsJSON(lang string) template.JS {
	i18nMutex.RLock()
	defer i18nMutex.RUnlock()

	if !IsLanguageSupported(lang) {
		lang = DefaultLanguage
	}
	transMap, ok := translations[lang]
	if !ok {
		transMap = translations[DefaultLanguage]
	}
	b, err := json.Marshal(transMap)
	if err != nil {
		return template.JS("{}")
	}
	return template.JS(b)
}

func GetTranslation(lang, key string) string {
	i18nMutex.RLock()
	defer i18nMutex.RUnlock()

	if !IsLanguageSupported(lang) {
		lang = DefaultLanguage
	}

	transMap, ok := translations[lang]
	if !ok {
		transMap = translations[DefaultLanguage]
	}

	if val, ok := transMap[key]; ok {
		return val
	}

	if lang != DefaultLanguage {
		if defMap, ok := translations[DefaultLanguage]; ok {
			if val, ok := defMap[key]; ok {
				return val
			}
		}
	}

	return key
}

func Translatef(lang, key string, args ...interface{}) string {
	format := GetTranslation(lang, key)
	if len(args) > 0 {
		return fmt.Sprintf(format, args...)
	}
	return format
}

func GetAllTranslationKeys() []string {
	i18nMutex.RLock()
	defer i18nMutex.RUnlock()

	keys := make(map[string]bool)
	for _, transMap := range translations {
		for k := range transMap {
			keys[k] = true
		}
	}

	result := make([]string, 0, len(keys))
	for k := range keys {
		result = append(result, k)
	}
	return result
}

func GetTranslationStats() map[string]int {
	i18nMutex.RLock()
	defer i18nMutex.RUnlock()

	stats := make(map[string]int)
	for lang, transMap := range translations {
		stats[lang] = len(transMap)
	}
	return stats
}

func GetMissingTranslations(lang string) []string {
	i18nMutex.RLock()
	defer i18nMutex.RUnlock()

	allKeys := make(map[string]bool)
	for _, transMap := range translations {
		for k := range transMap {
			allKeys[k] = true
		}
	}

	transMap, ok := translations[lang]
	if !ok {
		result := make([]string, 0, len(allKeys))
		for k := range allKeys {
			result = append(result, k)
		}
		return result
	}

	var missing []string
	for k := range allKeys {
		if _, ok := transMap[k]; !ok {
			missing = append(missing, k)
		}
	}
	return missing
}

func CheckPlaceholderConsistency(baseLang, targetLang string) map[string][]string {
	i18nMutex.RLock()
	defer i18nMutex.RUnlock()

	issues := make(map[string][]string)

	baseMap, ok := translations[baseLang]
	if !ok {
		return issues
	}

	targetMap, ok := translations[targetLang]
	if !ok {
		return issues
	}

	for key, baseVal := range baseMap {
		targetVal, ok := targetMap[key]
		if !ok {
			continue
		}

		basePlaceholders := extractPlaceholders(baseVal)
		targetPlaceholders := extractPlaceholders(targetVal)

		if len(basePlaceholders) != len(targetPlaceholders) {
			issues[key] = []string{
				fmt.Sprintf("base has %d placeholders: %v", len(basePlaceholders), basePlaceholders),
				fmt.Sprintf("target has %d placeholders: %v", len(targetPlaceholders), targetPlaceholders),
			}
		}
	}

	return issues
}

func extractPlaceholders(s string) []string {
	var placeholders []string
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+1 < len(s) {
			next := s[i+1]
			if next == '%' {
				i++
				continue
			}
			j := i + 1
			for j < len(s) && (s[j] >= '0' && s[j] <= '9' || s[j] == '.' || s[j] == '-' || s[j] == '+') {
				j++
			}
			if j < len(s) {
				placeholders = append(placeholders, s[i:j+1])
				i = j
			}
		}
	}
	return placeholders
}

func CheckHTMLTags(lang string) map[string]string {
	i18nMutex.RLock()
	defer i18nMutex.RUnlock()

	issues := make(map[string]string)

	transMap, ok := translations[lang]
	if !ok {
		return issues
	}

	for key, val := range transMap {
		if strings.Contains(val, "<") || strings.Contains(val, ">") {
			if !checkHTMLBalance(val) {
				issues[key] = "unbalanced HTML tags"
			}
		}
	}

	return issues
}

func checkHTMLBalance(s string) bool {
	openCount := 0
	closeCount := 0
	selfClosing := 0

	i := 0
	for i < len(s) {
		if s[i] == '<' {
			j := i + 1
			for j < len(s) && s[j] != '>' {
				j++
			}
			if j < len(s) {
				tag := s[i+1 : j]
				if len(tag) > 0 && tag[0] == '/' {
					closeCount++
				} else if len(tag) > 0 && tag[len(tag)-1] == '/' {
					selfClosing++
				} else if !strings.HasPrefix(tag, "!") && !strings.HasPrefix(tag, "?") {
					tagName := strings.Fields(tag)[0]
					tagName = strings.TrimRight(tagName, ">")
					voidTags := map[string]bool{
						"br": true, "hr": true, "img": true, "input": true,
						"meta": true, "link": true, "area": true, "base": true,
						"col": true, "embed": true, "source": true, "track": true,
						"wbr": true,
					}
					if voidTags[strings.ToLower(tagName)] {
						selfClosing++
					} else {
						openCount++
					}
				}
				i = j
			}
		}
		i++
	}

	return openCount == closeCount
}

func ExportTranslations(lang string) (TranslationMap, error) {
	i18nMutex.RLock()
	defer i18nMutex.RUnlock()

	transMap, ok := translations[lang]
	if !ok {
		return nil, fmt.Errorf("language not supported: %s", lang)
	}

	result := make(TranslationMap, len(transMap))
	for k, v := range transMap {
		result[k] = v
	}
	return result, nil
}

func ImportTranslations(lang string, data TranslationMap) error {
	if !IsLanguageSupported(lang) {
		return fmt.Errorf("language not supported: %s", lang)
	}

	i18nMutex.Lock()
	defer i18nMutex.Unlock()

	if translations[lang] == nil {
		translations[lang] = make(TranslationMap)
	}

	for k, v := range data {
		translations[lang][k] = v
	}

	return nil
}
