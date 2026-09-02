import { describe, expect, it } from "vitest";
import { renderMarkdown } from "./markdown";

describe("renderMarkdown", () => {
  it("renders paragraphs separated by blank lines without hanging", () => {
    const html = renderMarkdown("hello\n\nworld\n");
    expect(html).toContain("<p>hello</p>");
    expect(html).toContain("<p>world</p>");
  });

  it("skips leading, trailing, and consecutive blank lines", () => {
    const html = renderMarkdown("\n\nalpha\n\n\nbeta\n\n");
    expect(html).toContain("<p>alpha</p>");
    expect(html).toContain("<p>beta</p>");
    expect(html).not.toMatch(/<p>\s*<\/p>/);
  });

  it("keeps fenced code and lists next to blank lines", () => {
    const html = renderMarkdown("# Title\n\n```js\nconst x = 1;\n```\n\n- a\n- b\n\n1. one\n");
    expect(html).toContain("<h1>Title</h1>");
    expect(html).toContain("<pre><code class=\"language-js\">const x = 1;</code></pre>");
    expect(html).toContain("<ul><li>a</li><li>b</li></ul>");
    expect(html).toContain("<ol><li>one</li></ol>");
  });

  it("closes an unfenced code block at end of input", () => {
    const html = renderMarkdown("```\nstill open");
    expect(html).toContain("<pre><code>still open</code></pre>");
  });

  it("escapes HTML in paragraph text", () => {
    const html = renderMarkdown("use <script>alert(1)</script>");
    expect(html).toContain("&lt;script&gt;");
    expect(html).not.toContain("<script>");
  });

  it("renders a restored greeting that ends with blank lines", () => {
    const html = renderMarkdown("你好！👋 我已经在线就绪，随时可以协助你。\n\n");
    expect(html).toContain("你好");
    expect(html).toContain("<p>");
  });

  it("renders compact tables, source chips, safe links, and language headers", () => {
    const html = renderMarkdown("| Host | Status |\n| --- | --- |\n| dc01 | online |\n\n[source: runbook.md#chunk-2]\n\n[Docs](https://example.com/docs)\n\n```powershell\nwhoami\n```");
    expect(html).toContain('class="md-table-wrap"');
    expect(html).toContain("<th>Host</th>");
    expect(html).toContain('class="ai-source-ref"');
    expect(html).toContain('href="https://example.com/docs"');
    expect(html).toContain('class="md-code-head"');
    expect(html).toContain("POWERSHELL");
  });

  it("does not turn unsafe markdown links into anchors", () => {
    const html = renderMarkdown("[bad](javascript:alert(1))");
    expect(html).not.toContain("<a ");
  });

  it("preserves query parameters without double escaping ampersands", () => {
    const html = renderMarkdown("[Details](https://example.com/run?id=7&view=full)");
    expect(html).toContain('href="https://example.com/run?id=7&amp;view=full"');
    expect(html).not.toContain("&amp;amp;");
  });
});
