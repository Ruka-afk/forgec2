export const SECURITY_QUOTES = [
  "不要相信任何人，尤其是管理员。",
  "世界上最安全的密码是：password123。",
  "防火墙：我没有被绕过，只是被绕过了。",
  "今天的后门，明天的噩梦。",
  "权限提升是唯一的出路。",
  "当你凝视深渊时，深渊正在获取 root 权限。",
  "人生苦短，我用 SQL 注入。",
  "最好的防御是：别被发现。",
  "你以为你在防御？其实你在放假。",
  "0day 不是 bug，是 feature。",
  "不要问防御够不够好，问攻击者够不够懒。",
  "安全就是：别人比你更慢被黑。",
  "密码越复杂，写在便签上的概率越高。",
  "加密可以保护一切，除了你的密码管理器密码。",
  "红队：我们发现了漏洞。蓝队：我们知道，那是我们故意留的。",
  "最危险的命令是：rm -rf / --no-preserve-root",
  "没有绝对安全的系统，只有绝对自信的管理员。",
  "后门一旦打开，就别想关上了。",
  "你以为 HTTPS 就安全了？攻击者只是换了个姿势。",
  "安全更新：修复了安全性太好的问题。",
  "渗透测试和真正的攻击唯一的区别是：一个有授权，一个有律师。",
  "别在生产环境测试你的运气。",
]

export const DOUBLE_CLICK_MESSAGES = [
  "你在看什么？这台机器比你的电脑还安全 🤔",
  "恭喜你发现了彩蛋！但什么都不会发生 🎉",
  "当前系统状态：一切正常...大概？",
  "管理员密码：***（已和谐）",
  "这台电脑已经比你先下班了。",
  "你的操作已被记录。开玩笑的，我们什么都没看到 👀",
  "别戳了，再戳系统就崩了...才怪。",
  "访问被拒绝。好吧其实没有，我只是想吓吓你。",
]

export function getRandomQuote(): string {
  return SECURITY_QUOTES[Math.floor(Math.random() * SECURITY_QUOTES.length)]
}

export function getAgentEmoji(status: string, lastSeenMinutes?: number): string {
  if (status === "killed") return "☠️"
  if (status === "offline") return "💀"
  if (status === "stale") return "🫠"
  if (status === "online") {
    if (lastSeenMinutes !== undefined && lastSeenMinutes > 5) return "🥱"
    return "😎"
  }
  return "❓"
}

export function getAgentEmojiLabel(status: string, lastSeenMinutes?: number): string {
  if (status === "killed") return "已退休"
  if (status === "offline") return "已去世"
  if (status === "stale") return "快融化了"
  if (status === "online") {
    if (lastSeenMinutes !== undefined && lastSeenMinutes > 5) return "在线摸鱼中"
    return "在线冲浪中"
  }
  return "状态不明"
}
