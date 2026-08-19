import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { DataTable } from "@/components/ui/data-table";

vi.mock("@/lib/i18n", () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, string>) => {
      const map: Record<string, string> = {
        "common.pagination": "Pagination",
        "common.previous_page": "Previous page",
        "common.next_page": "Next page",
        "common.page_number": "page {n}",
        "common.no_data": "No data",
      };
      return map[key] || key.replace("{n}", params?.n || "?");
    },
  }),
}));

class ResizeObserverStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}

beforeEach(() => {
  (globalThis as Record<string, unknown>).ResizeObserver = ResizeObserverStub;
});

interface Row {
  id: string;
  name: string;
  score: number;
}

const columns = [
  { id: "name", header: "Name", cell: (r: Row) => r.name, sortValue: (r: Row) => r.name },
  { id: "score", header: "Score", cell: (r: Row) => r.score, sortValue: (r: Row) => r.score },
] satisfies Parameters<typeof DataTable<Row>>[0]["columns"];

const rows: Row[] = [
  { id: "1", name: "alpha", score: 10 },
  { id: "2", name: "bravo", score: 5 },
  { id: "3", name: "charlie", score: 20 },
];

describe("DataTable", () => {
  it("renders headers and all rows", () => {
    render(<DataTable data={rows} columns={columns} rowKey={(r) => r.id} />);
    expect(screen.getByText("Name")).toBeTruthy();
    expect(screen.getByText("Score")).toBeTruthy();
    expect(screen.getByText("alpha")).toBeTruthy();
    expect(screen.getByText("bravo")).toBeTruthy();
    expect(screen.getByText("charlie")).toBeTruthy();
  });

  it("shows the empty state when there is no data", () => {
    render(<DataTable data={[]} columns={columns} rowKey={() => ""} emptyTitle="Nothing here" />);
    expect(screen.getByText("Nothing here")).toBeTruthy();
  });

  it("shows a loading skeleton while loading", () => {
    render(<DataTable data={rows} columns={columns} rowKey={(r) => r.id} loading loadingSkeletonRows={2} />);
    expect(screen.queryByText("alpha")).toBeNull();
  });

  it("sorts ascending then descending when a sortable header is clicked", () => {
    render(<DataTable data={rows} columns={columns} rowKey={(r) => r.id} />);
    const header = screen.getByText("Score");
    fireEvent.click(header);
    const cells = screen.getAllByText(/bravo|charlie|alpha/);
    expect(cells[0].textContent).toBe("bravo");
    fireEvent.click(header);
    const cellsDesc = screen.getAllByText(/bravo|charlie|alpha/);
    expect(cellsDesc[0].textContent).toBe("charlie");
  });

  it("invokes the sort callback in controlled mode", () => {
    const onSortChange = vi.fn();
    render(
      <DataTable data={rows} columns={columns} rowKey={(r) => r.id} sort={null} onSortChange={onSortChange} />,
    );
    fireEvent.click(screen.getByText("Name"));
    expect(onSortChange).toHaveBeenCalledWith({ column: "name", dir: "asc" });
  });

  it("renders pagination when pagination props are set", () => {
    render(
      <DataTable
        data={rows}
        columns={columns}
        rowKey={(r) => r.id}
        pagination={{ page: 1, pageSize: 2, total: 3, onPageChange: () => {} }}
      />,
    );
    expect(screen.getByLabelText("Next page")).toBeTruthy();
  });

  it("virtualizes large row sets and renders a subset", () => {
    const big = Array.from({ length: 120 }, (_, i) => ({ id: String(i), name: `row-${i}`, score: i }));
    render(<DataTable data={big} columns={columns} rowKey={(r) => r.id} />);
    const rendered = screen.getAllByRole("row");
    expect(rendered.length).toBeLessThan(big.length);
    expect(screen.getByText("row-0")).toBeTruthy();
    expect(screen.queryByText("row-119")).toBeNull();
  });

  it("invokes onRowClick with the clicked row", () => {
    const onRowClick = vi.fn();
    render(<DataTable data={rows} columns={columns} rowKey={(r) => r.id} onRowClick={onRowClick} />);
    fireEvent.click(screen.getByText("bravo"));
    expect(onRowClick).toHaveBeenCalledWith(rows[1]);
  });

  it("shows the error state on error", () => {
    render(<DataTable data={rows} columns={columns} rowKey={(r) => r.id} error="boom" onRetry={() => {}} />);
    expect(screen.getByText("boom")).toBeTruthy();
  });
});