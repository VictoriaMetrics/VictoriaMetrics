import { describe, it, expect } from "vitest";
import { getLabelAlias } from "./metric";

describe("getLabelAlias", () => {
  it("replaces a simple placeholder with the label value", () => {
    expect(getLabelAlias({ job: "api" }, "{{job}}")).toBe("api");
  });

  it("resolves dotted placeholders such as {{service.name}}", () => {
    const fields = { "service.name": "checkout" };
    expect(getLabelAlias(fields, "{{service.name}}")).toBe("checkout");
  });

  it("resolves multiple dotted and plain placeholders in one alias", () => {
    const fields = { "service.name": "checkout", "k8s.pod.name": "pod-1", env: "prod" };
    expect(getLabelAlias(fields, "{{service.name}}/{{k8s.pod.name}} ({{env}})"))
      .toBe("checkout/pod-1 (prod)");
  });

  it("replaces a missing key with an empty string", () => {
    expect(getLabelAlias({ job: "api" }, "{{missing}}")).toBe("");
  });

  it("replaces a missing dotted key with an empty string", () => {
    expect(getLabelAlias({ job: "api" }, "svc={{service.name}}")).toBe("svc=");
  });

  it("keeps static text and replaces only the matching placeholders", () => {
    expect(getLabelAlias({ job: "api" }, "name: {{job}} / {{nope}}")).toBe("name: api / ");
  });

  it("returns the alias unchanged when there are no placeholders", () => {
    expect(getLabelAlias({ job: "api" }, "static alias")).toBe("static alias");
  });

  it("does not treat single braces as placeholders", () => {
    expect(getLabelAlias({ job: "api" }, "{job}")).toBe("{job}");
  });
});
