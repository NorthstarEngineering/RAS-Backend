import assert from "node:assert/strict";
import test from "node:test";
import { handler } from "./request-proxy.mjs";

test("routes known API paths to the service ports and forwards request data", async (t) => {
  const originalFetch = globalThis.fetch;
  t.after(() => { globalThis.fetch = originalFetch; });
  process.env.BACKEND_PRIVATE_IP = "10.40.10.12";

  const cases = [
    ["/health", 9001, "/health"],
    ["/api/v1/auth/health", 9001, "/health"],
    ["/api/v1/branches/health", 9002, "/health"],
    ["/api/v1/attendance/health", 9003, "/health"],
    ["/api/v1/central-kitchen/health", 9004, "/health"],
    ["/api/v1/menu/health", 9005, "/health"],
    ["/api/v1/sales/health", 9006, "/health"],
    ["/api/v1/cost/health", 9007, "/health"],
    ["/api/v1/transfers/health", 9008, "/healthz"],
    ["/api/v1/reports/health", 9009, "/health"],
    ["/api/v1/stock/health", 9010, "/healthz"],
    ["/api/v1/auth/login", 9001, "/api/v1/auth/login"],
    ["/api/v1/users", 9001, "/api/v1/users"],
    ["/api/v1/branches/7", 9002, "/api/v1/branches/7"],
    ["/api/v1/employees/8", 9003, "/api/v1/employees/8"],
    ["/api/v1/attendance", 9003, "/api/v1/attendance"],
    ["/api/v1/overtime/9", 9003, "/api/v1/overtime/9"],
    ["/api/v1/menu/items", 9005, "/api/v1/menu/items"],
    ["/api/v1/central-kitchen/supply/requests", 9004, "/api/v1/supply/requests"],
    ["/api/v1/stock/movements", 9010, "/api/v1/inventory/movements"],
    ["/api/v1/cost/items/123", 9007, "/api/v1/cost/items/123"],
    ["/api/v1/sales/shifts/open", 9006, "/api/v1/sales/shifts/open"],
    ["/api/v1/reports/sales", 9009, "/api/v1/reports/sales"],
    ["/api/v1/transfers/stats", 9008, "/api/v1/transfers/stats"],
  ];

  for (const [path, port, upstreamPath] of cases) {
    let forwarded;
    globalThis.fetch = async (url, options) => {
      forwarded = { url: String(url), options };
      return new Response("ok", { status: 200, headers: { "content-type": "text/plain" } });
    };

    const response = await handler({
      rawPath: path,
      rawQueryString: "page=2",
      headers: {
        authorization: "Bearer example-token",
        host: "gateway.invalid",
        "x-user-id": "spoofed-user",
        "x-user-role": "CEO",
        "x-branch-id": "spoofed-branch",
      },
      requestContext: {
        http: { method: "GET" },
        requestId: "request-123",
        authorizer: { lambda: { sub: "user-123", role: "BRANCH_MANAGER", branch_id: "branch-123" } },
      },
    });

    assert.equal(forwarded.url, `http://10.40.10.12:${port}${upstreamPath}?page=2`);
    assert.equal(forwarded.options.headers.authorization, "Bearer example-token");
    assert.equal(forwarded.options.headers.host, undefined);
    assert.equal(forwarded.options.headers["x-request-id"], "request-123");
    assert.equal(forwarded.options.headers["x-user-id"], "user-123");
    assert.equal(forwarded.options.headers["x-user-role"], "BRANCH_MANAGER");
    assert.equal(forwarded.options.headers["x-branch-id"], "branch-123");
    assert.equal(response.statusCode, 200);
    assert.equal(Buffer.from(response.body, "base64").toString(), "ok");
  }
});

test("returns a generic 404 for paths outside the API routes", async (t) => {
  const originalFetch = globalThis.fetch;
  t.after(() => { globalThis.fetch = originalFetch; });
  globalThis.fetch = async () => { throw new Error("must not call backend"); };

  const response = await handler({ rawPath: "/not-an-api-route", requestContext: { http: { method: "GET" } } });
  assert.equal(response.statusCode, 404);
  assert.deepEqual(JSON.parse(response.body), { error: "not found" });
});

test("removes the HTTP API stage prefix before forwarding", async (t) => {
  const originalFetch = globalThis.fetch;
  t.after(() => { globalThis.fetch = originalFetch; });
  process.env.BACKEND_PRIVATE_IP = "10.40.10.12";
  let forwarded;
  globalThis.fetch = async (url) => {
    forwarded = String(url);
    return new Response("ok", { status: 200 });
  };

  await handler({
    rawPath: "/dev/api/v1/branches",
    requestContext: { stage: "dev", http: { method: "GET" } },
  });
  assert.equal(forwarded, "http://10.40.10.12:9002/api/v1/branches");
});

test("hides downstream errors behind a generic 502 response", async (t) => {
  const originalFetch = globalThis.fetch;
  t.after(() => { globalThis.fetch = originalFetch; });
  globalThis.fetch = async () => { throw new Error("private endpoint detail"); };

  const response = await handler({ rawPath: "/api/v1/branches", requestContext: { http: { method: "GET" } } });
  assert.equal(response.statusCode, 502);
  assert.deepEqual(JSON.parse(response.body), { error: "backend unavailable" });
  assert.equal(response.body.includes("private endpoint detail"), false);
});
test("does not forward client identity headers without verified gateway claims", async (t) => {
  const originalFetch = globalThis.fetch;
  t.after(() => { globalThis.fetch = originalFetch; });
  globalThis.fetch = async (_url, options) => {
    for (const key of ["x-user-id", "x-user-role", "x-branch-id"]) {
      assert.equal(options.headers[key], undefined);
    }
    return new Response("ok");
  };
  const response = await handler({
    rawPath: "/api/v1/menu/items",
    headers: { "X-User-ID": "unverified", "x-user-role": "CEO", "X-Branch-ID": "unverified" },
    requestContext: { http: { method: "GET" } },
  });
  assert.equal(response.statusCode, 200);
});
