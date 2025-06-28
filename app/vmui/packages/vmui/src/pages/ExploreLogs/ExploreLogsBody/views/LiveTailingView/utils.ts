export class LogFlowAnalyzer {
  private threshold: number;
  private windowSize: number;
  private minHighCount: number;
  private minNormalCount: number;
  private window: number[];
  private state: "normal" | "high";

  /**
   * @param {number} threshold - The threshold value used for state evaluation. Defaults to 200.
   * @param {number} windowSize - The size of the window used for tracking data. Defaults to 10.
   * @param {number} minHighCount - The minimum number of high occurrences needed for state transition. Defaults to 6.
   * @param {number} minNormalCount - The minimum number of normal occurrences needed for state reset. Defaults to 2.
   * @return {void}
   */
  constructor(threshold: number = 200, windowSize: number = 10, minHighCount: number = 6, minNormalCount: number = 2) {
    this.threshold = threshold;
    this.windowSize = windowSize;
    this.minHighCount = minHighCount;
    this.minNormalCount = minNormalCount;
    this.window = [];
    this.state = "normal";
  }

  update(logCount: number): "normal" | "high" {
    this.window.push(logCount);
    if (this.window.length > this.windowSize) {
      this.window.shift();
    }

    const highCount = this.window.filter((x) => x > this.threshold).length;

    if (this.state === "normal") {
      if (highCount >= this.minHighCount) {
        this.state = "high";
      }
    } else if (this.state === "high") {
      if (highCount < this.minNormalCount) {
        this.state = "normal";
      }
    }

    return this.state;
  }
}
