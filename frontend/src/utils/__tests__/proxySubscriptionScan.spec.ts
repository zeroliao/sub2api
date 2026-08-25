import { describe, expect, it } from "vitest";
import { proxySubscriptionScanErrorCount } from "../proxySubscriptionScan";

describe("proxySubscriptionScanErrorCount", () => {
  it("returns the persisted error array length", () => {
    expect(
      proxySubscriptionScanErrorCount({
        errors: ["first failure", "second failure"],
      }),
    ).toBe(2);
  });

  it("supports legacy numeric error counts and ignores invalid values", () => {
    expect(proxySubscriptionScanErrorCount({ errors: 3 })).toBe(3);
    expect(proxySubscriptionScanErrorCount({ errors: -1 })).toBe(0);
    expect(proxySubscriptionScanErrorCount({ errors: "3" })).toBe(0);
  });
});
