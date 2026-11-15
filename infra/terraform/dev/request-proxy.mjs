const backendPort = (path) => {
  if (path === "/health" || path.startsWith("/api/v1/auth/") || path === "/api/v1/auth" || path.startsWith("/api/v1/users") || path === "/api/v1/users") return 9001;
  if (path.startsWith("/api/v1/branches") || path === "/api/v1/branches") return 9002;
  if (["/api/v1/employees", "/api/v1/attendance", "/api/v1/overtime"].some((prefix) => path === prefix || path.startsWith(`${prefix}/`))) return 9003;
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
  const port = backendPort(path);
  if (!port) return { statusCode: 404, headers: { "content-type": "application/json" }, body: JSON.stringify({ error: "not found" }) };

  const headers = { ...(event.headers || {}) };
  for (const name of ["host", "connection", "content-length", "transfer-encoding", "accept-encoding"]) delete headers[name];
  headers["x-request-id"] = event.requestContext?.requestId || headers["x-request-id"] || "unknown";
  const query = event.rawQueryString ? `?${event.rawQueryString}` : "";
  const target = `http://${process.env.BACKEND_PRIVATE_IP}:${port}${path}${query}`;
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
