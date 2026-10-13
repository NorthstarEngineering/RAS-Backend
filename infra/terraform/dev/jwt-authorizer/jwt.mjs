import { createHmac, timingSafeEqual } from "node:crypto";

const ISSUER = "restaurant-backend";
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
const ROLE_MAP = new Map([["CEO", "CEO"], ["BRANCH_MANAGER", "BRANCH_MANAGER"], ["SUPPLY_MANAGER", "SUPPLY_MANAGER"], ["BM", "BRANCH_MANAGER"]]);

function decodePart(part) {
  if (!/^[A-Za-z0-9_-]+$/.test(part)) throw new Error("invalid JWT encoding");
  return Buffer.from(part, "base64url");
}

export function parseAndVerify(rawToken, secret, nowSeconds = Math.floor(Date.now() / 1000)) {
  if (typeof rawToken !== "string" || rawToken.length > 8192 || !Buffer.isBuffer(secret) || secret.length < 32) return null;
  const parts = rawToken.split(".");
  if (parts.length !== 3) return null;
  let header, claims, suppliedSignature;
  try {
    header = JSON.parse(decodePart(parts[0]).toString("utf8"));
    claims = JSON.parse(decodePart(parts[1]).toString("utf8"));
    suppliedSignature = decodePart(parts[2]);
  } catch {
    return null;
  }
  if (header?.alg !== "HS256" || header?.typ !== "JWT") return null;
  const expected = createHmac("sha256", secret).update(`${parts[0]}.${parts[1]}`).digest();
  if (suppliedSignature.length !== expected.length || !timingSafeEqual(suppliedSignature, expected)) return null;
  if (claims?.iss !== ISSUER || typeof claims.sub !== "string" || !UUID.test(claims.sub)) return null;
  if (!Number.isInteger(claims.exp) || nowSeconds >= claims.exp) return null;
  if (claims.nbf !== undefined && (!Number.isInteger(claims.nbf) || nowSeconds < claims.nbf)) return null;
  if (claims.mcp !== undefined && typeof claims.mcp !== "boolean") return null;
  const role = ROLE_MAP.get(claims.role);
  if (!role) return null;
  if (role === "BRANCH_MANAGER") {
    if (typeof claims.branch_id !== "string" || !UUID.test(claims.branch_id)) return null;
  } else if (claims.branch_id !== undefined && claims.branch_id !== "") {
    return null;
  }
  return { sub: claims.sub, role, branch_id: claims.branch_id ?? "", mcp: claims.mcp ?? false };
}
