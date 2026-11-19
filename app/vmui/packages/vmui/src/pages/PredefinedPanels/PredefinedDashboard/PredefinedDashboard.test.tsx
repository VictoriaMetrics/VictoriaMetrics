import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/preact";
import PredefinedDashboard from "./PredefinedDashboard";
import { useEffect } from "preact/compat";

const mockPanelQuery = vi.fn();

vi.mock("../PredefinedPanel/PredefinedPanel", () => ({
  default: ({ title, expr }: { title?: string; expr?: string[] }) => {
    useEffect(() => {
      mockPanelQuery(title, expr);
    }, [title, expr]);

    return (
      <div
        data-testid={`panel-${title}`}
        className="mock-predefined-panel"
      >
        <span className="mock-panel-title">{title}</span>
        <span className="mock-panel-expr">{expr?.join(",")}</span>
      </div>
    );
  },
}));

describe("PredefinedDashboard", () => {
  const row1Panels = [
    { title: "Panel 1A", expr: ["metric_1a"] },
    { title: "Panel 1B", expr: ["metric_1b"] },
  ];

  const row2Panels = [
    { title: "Panel 2A", expr: ["metric_2a"] },
  ];

  const row3Panels = [
    { title: "Panel 3A", expr: ["metric_3a"] },
  ];

  beforeEach(() => {
    vi.clearAllMocks();
  });

  const renderMultiRowDashboard = () => {
    return render(
      <div>
        <PredefinedDashboard
          index={0}
          filename="dash.json"
          title="Row 1 Title"
          panels={row1Panels}
        />
        <PredefinedDashboard
          index={1}
          filename="dash.json"
          title="Row 2 Title"
          panels={row2Panels}
        />
        <PredefinedDashboard
          index={2}
          filename="dash.json"
          title="Row 3 Title"
          panels={row3Panels}
        />
      </div>
    );
  };

  /**
   * Dispatches a realistic user click sequence: mousedown -> mouseup -> click.
   * This is critical because mouseup triggers the window listener where the bug originated.
   */
  const userClick = (element: Element) => {
    fireEvent.mouseDown(element);
    fireEvent.mouseUp(element);
    fireEvent.click(element);
  };

  it("renders multiple rows with row 1 expanded and subsequent rows collapsed by default", () => {
    renderMultiRowDashboard();

    // All row headers are visible
    expect(screen.getByText("Row 1 Title")).toBeInTheDocument();
    expect(screen.getByText("Row 2 Title")).toBeInTheDocument();
    expect(screen.getByText("Row 3 Title")).toBeInTheDocument();

    // Row 1 starts expanded by default (index 0)
    expect(screen.getByTestId("panel-Panel 1A")).toBeInTheDocument();
    expect(screen.getByTestId("panel-Panel 1B")).toBeInTheDocument();
    expect(mockPanelQuery).toHaveBeenCalledWith("Panel 1A", ["metric_1a"]);
    expect(mockPanelQuery).toHaveBeenCalledWith("Panel 1B", ["metric_1b"]);

    // Rows 2 and 3 start collapsed by default (index > 0)
    expect(screen.queryByTestId("panel-Panel 2A")).not.toBeInTheDocument();
    expect(screen.queryByTestId("panel-Panel 3A")).not.toBeInTheDocument();
    expect(mockPanelQuery).not.toHaveBeenCalledWith("Panel 2A", expect.anything());
    expect(mockPanelQuery).not.toHaveBeenCalledWith("Panel 3A", expect.anything());
  });

  it("expands a collapsed row when its TITLE text is clicked and loads its panels", () => {
    renderMultiRowDashboard();

    const row2Title = screen.getByText("Row 2 Title");
    expect(screen.queryByTestId("panel-Panel 2A")).not.toBeInTheDocument();

    // Click specifically on the title text element (not chevron)
    userClick(row2Title);

    // Row 2 expands and panels become visible
    expect(screen.getByTestId("panel-Panel 2A")).toBeInTheDocument();
    expect(mockPanelQuery).toHaveBeenCalledWith("Panel 2A", ["metric_2a"]);

    // Clicking the title again collapses it
    userClick(row2Title);
    expect(screen.queryByTestId("panel-Panel 2A")).not.toBeInTheDocument();
  });

  it("expands and collapses a row when its chevron arrow is clicked", () => {
    const { container } = renderMultiRowDashboard();

    const headers = container.querySelectorAll(".vm-accordion-header");
    const row2Header = headers[1];
    const row2Chevron = row2Header.querySelector(".vm-accordion-header__arrow");
    expect(row2Chevron).toBeTruthy();

    expect(screen.queryByTestId("panel-Panel 2A")).not.toBeInTheDocument();

    // Click the chevron
    userClick(row2Chevron!);
    expect(screen.getByTestId("panel-Panel 2A")).toBeInTheDocument();
    expect(mockPanelQuery).toHaveBeenCalledWith("Panel 2A", ["metric_2a"]);

    // Click chevron again to collapse
    userClick(row2Chevron!);
    expect(screen.queryByTestId("panel-Panel 2A")).not.toBeInTheDocument();
  });

  it("expanding and collapsing one row does not affect other rows", () => {
    renderMultiRowDashboard();

    const row2Title = screen.getByText("Row 2 Title");

    // Initially: Row 1 is open, Row 2 and 3 are closed
    expect(screen.getByTestId("panel-Panel 1A")).toBeInTheDocument();
    expect(screen.queryByTestId("panel-Panel 2A")).not.toBeInTheDocument();
    expect(screen.queryByTestId("panel-Panel 3A")).not.toBeInTheDocument();

    // Expand Row 2
    userClick(row2Title);

    // Both Row 1 and Row 2 are now open, Row 3 remains closed
    expect(screen.getByTestId("panel-Panel 1A")).toBeInTheDocument();
    expect(screen.getByTestId("panel-Panel 1B")).toBeInTheDocument();
    expect(screen.getByTestId("panel-Panel 2A")).toBeInTheDocument();
    expect(screen.queryByTestId("panel-Panel 3A")).not.toBeInTheDocument();

    // Collapse Row 1 by clicking its title
    const row1Title = screen.getByText("Row 1 Title");
    userClick(row1Title);

    // Row 1 is now closed, Row 2 remains open, Row 3 remains closed
    expect(screen.queryByTestId("panel-Panel 1A")).not.toBeInTheDocument();
    expect(screen.getByTestId("panel-Panel 2A")).toBeInTheDocument();
    expect(screen.queryByTestId("panel-Panel 3A")).not.toBeInTheDocument();
  });
});
