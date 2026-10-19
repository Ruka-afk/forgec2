import { describe, expect, it } from "vitest";
import {
  filterWeChatMessages,
  groupWeChatConversations,
  parseWeChatHistory,
  parseWeChatHistoryWithMeta,
  sortWeChatMessages,
  toWeChatCSV,
  type WeChatMessage,
} from "./wechat-history";

describe("parseWeChatHistory", () => {
  it("parses Go and C implant JSON Lines", () => {
    const raw = [
      "=== wechat history ===",
      '{"time":"2026-09-13T10:30:00Z","sender":"me","contact":"张三","contact_id":"wxid_xxx","type":1,"content":"你好"}',
      '{"time":"2026-09-13T10:30:05Z","sender":"other","contact":"张三","contact_id":"wxid_xxx","type":1,"content":"在吗"}',
      "# total_rows=2",
    ].join("\n");
    expect(parseWeChatHistory(raw)).toEqual([
      {
        time: "2026-09-13T10:30:00Z",
        sender: "me",
        contact: "张三",
        contactId: "wxid_xxx",
        kind: "1",
        content: "你好",
      },
      {
        time: "2026-09-13T10:30:05Z",
        sender: "other",
        contact: "张三",
        contactId: "wxid_xxx",
        kind: "1",
        content: "在吗",
      },
    ]);
  });

  it("skips headers, errors and malformed rows", () => {
    const raw = [
      "=== wechat history ===",
      "=== Msg_abc.db ===",
      "query error: no such table: message",
      '{"time":"2026-09-13T10:30:00Z","sender":"me","contact":"张三","contact_id":"wxid_xxx","type":1,"content":"ok"}',
      '{"time":"broken","contact":',
      "(no matching messages found)",
      "# total_rows=1",
    ].join("\n");
    expect(parseWeChatHistory(raw)).toEqual([
      {
        time: "2026-09-13T10:30:00Z",
        sender: "me",
        contact: "张三",
        contactId: "wxid_xxx",
        kind: "1",
        content: "ok",
      },
    ]);
  });

  it("returns empty for blank or ack payloads", () => {
    expect(parseWeChatHistory("")).toEqual([]);
    expect(parseWeChatHistory(JSON.stringify({ success: true, task_id: 7 }))).toEqual([]);
  });

  it("reports diagnostics without breaking valid rows", () => {
    const meta = parseWeChatHistoryWithMeta(
      [
        "(WeChat data directory not found or not on Windows)",
        "query error: no such table: message",
        '{"time":"2026-09-13T10:30:00Z","sender":"me","contact":"张三","contact_id":"wxid_xxx","type":1,"content":"ok"}',
        '{"time":"broken","contact":',
      ].join("\n"),
    );
    expect(meta.hasNoSource).toBe(true);
    expect(meta.queryErrors).toEqual(["no such table: message"]);
    expect(meta.skipped).toBe(1);
    expect(meta.messages).toHaveLength(1);
  });

  it("groups, filters, sorts and exports conversations", () => {
    const messages: WeChatMessage[] = [
      { time: "2026-09-13T10:31:00Z", sender: "other", contact: "李四", contactId: "wxid_b", kind: "1", content: "收到" },
      { time: "2026-09-13T10:30:00Z", sender: "me", contact: "张三", contactId: "wxid_a", kind: "1", content: "你好" },
      { time: "2026-09-13T10:32:00Z", sender: "me", contact: "张三", contactId: "wxid_a", kind: "1", content: "在吗" },
    ];

    expect(groupWeChatConversations(messages)).toEqual([
      { id: "wxid_a", contact: "张三", contactId: "wxid_a", count: 2, latest: "2026-09-13T10:32:00Z" },
      { id: "wxid_b", contact: "李四", contactId: "wxid_b", count: 1, latest: "2026-09-13T10:31:00Z" },
    ]);
    expect(filterWeChatMessages(messages, "在吗", "all").map((row) => row.content)).toEqual(["在吗"]);
    expect(filterWeChatMessages(messages, "", "me").map((row) => row.contact)).toEqual(["张三", "张三"]);
    expect(sortWeChatMessages(messages, "asc").map((row) => row.time)).toEqual([
      "2026-09-13T10:30:00Z",
      "2026-09-13T10:31:00Z",
      "2026-09-13T10:32:00Z",
    ]);
    expect(toWeChatCSV(messages.slice(0, 1))).toBe(
      "Time,Sender,Contact,Contact ID,Type,Content\n2026-09-13T10:31:00Z,other,李四,wxid_b,1,收到\n",
    );
  });
});
