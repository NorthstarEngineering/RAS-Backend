const backendTarget = (path) => {
  const serviceHealth = new Map([
    ["/api/v1/auth/health", [9001, "/health"]],
    ["/api/v1/branches/health", [9002, "/health"]],
    ["/api/v1/attendance/health", [9003, "/health"]],
    ["/api/v1/central-kitchen/health", [9004, "/health"]],
    ["/api/v1/menu/health", [9005, "/health"]],
    ["/api/v1/sales/health", [9006, "/health"]],
    ["/api/v1/cost/health", [9007, "/health"]],
    ["/api/v1/transfers/health", [9008, "/healthz"]],
    ["/api/v1/reports/health", [9009, "/health"]],
    ["/api/v1/stock/health", [9010, "/healthz"]],
  ]);
  const health = serviceHealth.get(path);
  if (health) return { port: health[0], path: health[1] };
  if (path === "/health" || ["/api/v1/auth", "/api/v1/users"].some((prefix) => path === prefix || path.startsWith(`${prefix}/`))) return { port: 9001, path };
  if (["/api/v1/branches"].some((prefix) => path === prefix || path.startsWith(`${prefix}/`))) return { port: 9002, path };
  if (["/api/v1/employees", "/api/v1/attendance", "/api/v1/overtime"].some((prefix) => path === prefix || path.startsWith(`${prefix}/`))) return { port: 9003, path };

  const namespaced = [
    ["/api/v1/menu", 9005, "/api/v1/menu"],
    ["/api/v1/central-kitchen", 9004, "/api/v1"],
    ["/api/v1/stock", 9010, "/api/v1/inventory"],
    ["/api/v1/cost", 9007, "/api/v1/cost"],
    ["/api/v1/sales", 9006, "/api/v1/sales"],
    ["/api/v1/reports", 9009, "/api/v1/reports"],
  ];
  for (const [prefix, port, destination] of namespaced) {
    if (path === prefix || path.startsWith(`${prefix}/`)) return { port, path: `${destination}${path.slice(prefix.length)}` };
  }
  if (path === "/api/v1/transfers" || path.startsWith("/api/v1/transfers/")) return { port: 9008, path };
  return null;
};

export const handler = async (event) => {
  const rawPath = event.rawPath || "/";
  const stage = event.requestContext?.stage;
  const stagePrefix = stage && stage !== "$default" ? `/${stage}` : null;
  const path = stagePrefix && (rawPath === stagePrefix || rawPath.startsWith(`${stagePrefix}/`))
    ? rawPath.slice(stagePrefix.length) || "/"
    : rawPath;
  const method = event.requestContext?.http?.method || "GET";
  if (method === "OPTIONS") {
    return { statusCode: 204, headers: {}, body: "" };
  }
  const targetRoute = backendTarget(path);
  if (!targetRoute) return { statusCode: 404, headers: { "content-type": "application/json" }, body: JSON.stringify({ error: "not found" }) };

  const headers = Object.fromEntries(Object.entries(event.headers || {}).map(([name, value]) => [name.toLowerCase(), value]));
  for (const name of ["host", "connection", "content-length", "transfer-encoding", "accept-encoding", "x-user-id", "x-user-role", "x-branch-id"]) delete headers[name];
  const identity = event.requestContext?.authorizer?.lambda || {};
  if (identity.sub) headers["x-user-id"] = identity.sub;
  if (identity.role) headers["x-user-role"] = identity.role;
  if (identity.branch_id) headers["x-branch-id"] = identity.branch_id;
  headers["x-request-id"] = event.requestContext?.requestId || headers["x-request-id"] || "unknown";
  const query = event.rawQueryString ? `?${event.rawQueryString}` : "";
  const target = `http://${process.env.BACKEND_PRIVATE_IP}:${targetRoute.port}${targetRoute.path}${query}`;
  const body = event.body == null ? undefined : event.isBase64Encoded ? Buffer.from(event.body, "base64") : event.body;

  try {
    const response = await fetch(target, { method, headers, body: ["GET", "HEAD"].includes(method) ? undefined : body, signal: AbortSignal.timeout(27000) });
    const responseHeaders = Object.fromEntries(response.headers.entries());
    for (const name of ["connection", "content-length", "transfer-encoding", "content-encoding"]) delete responseHeaders[name];
    const bytes = Buffer.from(await response.arrayBuffer());
    const emptyStatus = [204, 304].includes(response.status);
    return { statusCode: response.status, headers: responseHeaders, body: emptyStatus ? "" : bytes.toString("base64"), isBase64Encoded: !emptyStatus };
  } catch {
    return { statusCode: 502, headers: { "content-type": "application/json" }, body: JSON.stringify({ error: "backend unavailable" }) };
  }
};
