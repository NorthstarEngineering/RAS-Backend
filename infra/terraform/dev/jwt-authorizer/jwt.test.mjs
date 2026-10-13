import test from "node:test";
import assert from "node:assert/strict";
import { createHmac } from "node:crypto";
import { parseAndVerify } from "./jwt.mjs";

const secret = Buffer.from("local-test-secret-that-is-at-least-32-bytes", "utf8");
const userID = "a51bd4f0-312a-4f78-88d3-d79157489255";
const branchID = "7ac36815-a03e-4f6a-91d9-6457afd67cd1";
const now = 1_800_000_000;

function token(claimOverrides = {}, headerOverrides = {}) {
  const header = Buffer.from(JSON.stringify({ alg: "HS256", typ: "JWT", ...headerOverrides })).toString("base64url");
  const claims = Buffer.from(JSON.stringify({ iss: "restaurant-backend", sub: userID, role: "CEO", iat: now, exp: now + 900, ...claimOverrides })).toString("base64url");
  const payload = `${header}.${claims}`;
  return `${payload}.${createHmac("sha256", secret).update(payload).digest("base64url")}`;
}

test("accepts an Auth-compatible CEO token", () => {
  assert.deepEqual(parseAndVerify(token(), secret, now), { sub: userID, role: "CEO", branch_id: "", mcp: false });
});
test("canonicalizes the legacy BM role and requires its branch claim", () => {
  assert.deepEqual(parseAndVerify(token({ role: "BM", branch_id: branchID }), secret, now), { sub: userID, role: "BRANCH_MANAGER", branch_id: branchID, mcp: false });
  assert.equal(parseAndVerify(token({ role: "BRANCH_MANAGER" }), secret, now), null);
});
test("preserves the Auth first-login claim for backend route-level enforcement", () => {
  assert.deepEqual(parseAndVerify(token({ mcp: true }), secret, now), { sub: userID, role: "CEO", branch_id: "", mcp: true });
  assert.equal(parseAndVerify(token({ mcp: "true" }), secret, now), null);
});
test("rejects expiry, not-before, wrong issuer, and invalid signatures", () => {
  assert.equal(parseAndVerify(token({ exp: now }), secret, now), null);
  assert.equal(parseAndVerify(token({ nbf: now + 1 }), secret, now), null);
  assert.equal(parseAndVerify(token({ iss: "other" }), secret, now), null);
  const [header, claims, signature] = token().split(".");
  const changedSignature = `${signature[0] === "A" ? "B" : "A"}${signature.slice(1)}`;
  assert.equal(parseAndVerify(`${header}.${claims}.${changedSignature}`, secret, now), null);
});
test("rejects unexpected algorithm, malformed subject, role, or branch claims", () => {
  assert.equal(parseAndVerify(token({}, { alg: "none" }), secret, now), null);
  assert.equal(parseAndVerify(token({ sub: "not-a-uuid" }), secret, now), null);
  assert.equal(parseAndVerify(token({ role: "UNKNOWN" }), secret, now), null);
  assert.equal(parseAndVerify(token({ branch_id: branchID }), secret, now), null);
});
