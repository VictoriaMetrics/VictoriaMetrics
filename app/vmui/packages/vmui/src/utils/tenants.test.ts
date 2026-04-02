import { describe, it, expect } from "vitest";

import {
  replaceTenantId,
  getTenantIdFromUrl,
  getUrlWithoutTenant,
} from "./tenants";

describe("tenant url helpers", () => {
  describe("getTenantIdFromUrl", () => {
    it("returns accountID", () => {
      expect(getTenantIdFromUrl("http://vmselect:8481/select/0/vmui/")).toBe("0");
    });

    it("returns accountID:projectID", () => {
      expect(getTenantIdFromUrl("http://vmselect:8481/select/12:7/vmui/")).toBe("12:7");
    });

    it("returns empty string if tenant is missing", () => {
      expect(getTenantIdFromUrl("http://vmselect:8481/select/vmui/")).toBe("");
    });

    it("returns empty string for unrelated paths", () => {
      expect(getTenantIdFromUrl("http://vmselect:8481/foo/bar")).toBe("");
    });

    it("returns accountID when url ends right after tenant", () => {
      expect(getTenantIdFromUrl("http://vmselect:8481/select/0")).toBe("0");
    });
  });

  describe("replaceTenantId", () => {
    it("replaces accountID with another accountID", () => {
      expect(
        replaceTenantId("http://vmselect:8481/select/0/vmui/", "2")
      ).toBe("http://vmselect:8481/select/2/vmui/");
    });

    it("replaces accountID with accountID:projectID", () => {
      expect(
        replaceTenantId("http://vmselect:8481/select/0/prometheus/", "1:9")
      ).toBe("http://vmselect:8481/select/1:9/prometheus/");
    });

    it("keeps the rest of the path intact", () => {
      expect(
        replaceTenantId("http://vmselect:8481/select/3:4/prometheus/api/v1/query", "7")
      ).toBe("http://vmselect:8481/select/7/prometheus/api/v1/query");
    });

    it("does not change url if it doesn't match expected pattern", () => {
      expect(
        replaceTenantId("http://vmselect:8481/foo/bar", "2")
      ).toBe("http://vmselect:8481/foo/bar");
    });
  });

  describe("getUrlWithoutTenant", () => {
    it("removes /select/<tenant>/... and returns base url", () => {
      expect(
        getUrlWithoutTenant("http://vmselect:8481/select/0/vmui/")
      ).toBe("http://vmselect:8481");
    });

    it("removes /select/<tenant>/... for accountID:projectID and returns base url", () => {
      expect(
        getUrlWithoutTenant("http://vmselect:8481/select/5:6/prometheus/")
      ).toBe("http://vmselect:8481");
    });

    it("works with deep paths and returns base url", () => {
      expect(
        getUrlWithoutTenant("http://vmselect:8481/select/1:2/prometheus/api/v1/query")
      ).toBe("http://vmselect:8481");
    });

    it("does not change url if it doesn't match expected pattern", () => {
      expect(
        getUrlWithoutTenant("http://vmselect:8481/foo/bar")
      ).toBe("http://vmselect:8481/foo/bar");
    });

    it("removes url ending right after tenant", () => {
      expect(
        getUrlWithoutTenant("http://vmselect:8481/select/0")
      ).toBe("http://vmselect:8481");
    });
  });
});
