import assert from "node:assert/strict";
import test from "node:test";
import { handler } from "./request-proxy.mjs";

test("routes known API paths to the service ports and forwards request data", async (t) => {
  const originalFetch = globalThis.fetch;
  t.after(() => { globalThis.fetch = originalFetch; });
  process.env.BACKEND_PRIVATE_IP = "10.40.10.12";

  const cases = [
    ["/health", 9001],
    ["/api/v1/auth/login", 9001],
    ["/api/v1/users", 9001],
    ["/api/v1/branches/7", 9002],
    ["/api/v1/employees/8", 9003],
    ["/api/v1/attendance", 9003],
    ["/api/v1/overtime/9", 9003],
  ];

  for (const [path, port] of cases) {
    let forwarded;
    globalThis.fetch = async (url, options) => {
      forwarded = { url: String(url), options };
      return new Response("ok", { status: 200, headers: { "content-type": "text/plain" } });
    };

    const response = await handler({
      rawPath: path,
      rawQueryString: "page=2",
      headers: { authorization: "Bearer example-token", host: "gateway.invalid" },
      requestContext: { http: { method: "GET" }, requestId: "request-123" },
    });

    assert.equal(forwarded.url, `http://10.40.10.12:${port}${path}?page=2`);
    assert.equal(forwarded.options.headers.authorization, "Bearer example-token");
    assert.equal(forwarded.options.headers.host, undefined);
    assert.equal(forwarded.options.headers["x-request-id"], "request-123");
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
