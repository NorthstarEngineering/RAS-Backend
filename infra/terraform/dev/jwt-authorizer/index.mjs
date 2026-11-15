import { SSMClient, GetParameterCommand } from "@aws-sdk/client-ssm";
import { parseAndVerify } from "./jwt.mjs";
const ssm = new SSMClient({});
let secretValue;
let secretLoadedAt = 0;

async function loadSigningSecret() {
  if (secretValue && Date.now() - secretLoadedAt < 60_000) return secretValue;
  const result = await ssm.send(new GetParameterCommand({ Name: process.env.APP_SECRET_PARAMETER_NAME, WithDecryption: true }));
  let value;
  try { value = JSON.parse(result.Parameter?.Value || "").JWT_SECRET; } catch { value = undefined; }
  if (typeof value !== "string" || value.length < 32) {
    throw new Error("JWT signing secret is not configured");
  }
  secretValue = Buffer.from(value, "utf8");
  secretLoadedAt = Date.now();
  return secretValue;
}

export async function handler(event) {
  const authorization = event?.headers?.authorization ?? event?.headers?.Authorization;
  const match = typeof authorization === "string" && /^Bearer\s+([^\s]+)$/i.exec(authorization.trim());
  if (!match) return { isAuthorized: false };
  const secret = await loadSigningSecret();
  const claims = parseAndVerify(match[1], secret);
  if (!claims) return { isAuthorized: false };
  return { isAuthorized: true, context: claims };
}
