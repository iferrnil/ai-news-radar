import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { healthLevelOf, sourceUnhealthy, viewSourcesOk, type ViewOkSource } from "./view-ok.ts";

const now = 1_700_000_000;
const sectionOf = (s: ViewOkSource) => s.section || "ai";
const inAll = (k: string) => k === "ai" || k === "tech";

function src(p: Partial<ViewOkSource> & { id: number }): ViewOkSource {
  return {
    enabled: 1, supported: 1, type: "rss", section: "ai", last_error: null,
    h_runs: "1111111111", h_consec: 0, h_open: 0, ...p,
  };
}

describe("healthLevelOf / sourceUnhealthy", () => {
  it("ok runs are healthy", () => {
    assert.equal(healthLevelOf(src({ id: 1 }), now), "ok");
    assert.equal(sourceUnhealthy(src({ id: 1 }), now), false);
  });
  it("degraded / down / open / last_error are unhealthy", () => {
    assert.equal(healthLevelOf(src({ id: 1, h_runs: "1111111110" }), now), "degraded");
    assert.ok(sourceUnhealthy(src({ id: 1, h_runs: "1111111110" }), now));
    assert.equal(healthLevelOf(src({ id: 1, h_consec: 2, h_runs: "1111111100" }), now), "down");
    assert.ok(sourceUnhealthy(src({ id: 1, h_consec: 2, h_runs: "00" }), now));
    assert.equal(healthLevelOf(src({ id: 1, h_open: now + 60 }), now), "open");
    assert.ok(sourceUnhealthy(src({ id: 1, h_open: now + 60 }), now));
    assert.ok(sourceUnhealthy(src({ id: 1, last_error: "timeout", h_runs: "1111111111" }), now));
  });
});

describe("viewSourcesOk", () => {
  const healthy = [src({ id: 1, section: "ai" }), src({ id: 2, section: "tech" })];
  const mixed = [src({ id: 1, section: "ai" }), src({ id: 2, section: "ai", h_runs: "1111111110", last_error: "429" })];

  it("non-feed paths always ok", () => {
    assert.equal(viewSourcesOk("/settings", mixed, {}, sectionOf, inAll, now), true);
    assert.equal(viewSourcesOk("/sources", mixed, {}, sectionOf, inAll, now), true);
  });

  it("all-view: any degraded in-all source blocks Live", () => {
    assert.equal(viewSourcesOk("/", healthy, {}, sectionOf, inAll, now), true);
    assert.equal(viewSourcesOk("/", mixed, {}, sectionOf, inAll, now), false);
  });

  it("section-view: only current section matters", () => {
    const otherBad = [src({ id: 1, section: "ai" }), src({ id: 3, section: "biz", h_runs: "00", last_error: "x" })];
    assert.equal(viewSourcesOk("/", otherBad, { section: "ai" }, sectionOf, inAll, now), true);
    assert.equal(viewSourcesOk("/", mixed, { section: "ai" }, sectionOf, inAll, now), false);
  });

  it("source-view: only that source", () => {
    assert.equal(viewSourcesOk("/", mixed, { source: 1 }, sectionOf, inAll, now), true);
    assert.equal(viewSourcesOk("/", mixed, { source: 2 }, sectionOf, inAll, now), false);
  });

  it("empty in-view is ok", () => {
    assert.equal(viewSourcesOk("/", [], { section: "ai" }, sectionOf, inAll, now), true);
    assert.equal(viewSourcesOk("/", healthy, { section: "missing" }, sectionOf, inAll, now), true);
  });
});
