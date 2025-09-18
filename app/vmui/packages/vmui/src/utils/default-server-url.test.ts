import { describe, it, expect } from "vitest";
import { getDefaultURL } from "./default-server-url";

describe("test server urls", () => {
  describe("getDefaultURL()", () => {
    it("/select/0/vmui/", () => {
      const result = getDefaultURL("https://localhost:1111/select/0/vmui/");
      expect(result).toBe("https://localhost:1111/select/0/prometheus");
    });

    it("/any/path/prefix/select/multitenant/vmui/#/rules?q=test", () => {
      const result = getDefaultURL("http://test/any/path/prefix/select/multitenant/vmui/#/rules?q=test");
      expect(result).toBe("http://test/any/path/prefix/select/multitenant/prometheus");
    });

    it("/test/select/1:1/prometheus/graph/", () => {
      const result = getDefaultURL("https://domain.com/test/select/1:1/prometheus/graph/");
      expect(result).toBe("https://domain.com/test/select/1:1/prometheus");
    });

    it("https://play.vm.com/#/rules?q=test", () => {
      const result = getDefaultURL("https://play.vm.com/#/rules?q=test");
      expect(result).toBe("https://play.vm.com");
    });
  });
});
