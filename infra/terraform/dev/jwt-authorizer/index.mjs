import { SecretsManagerClient, GetSecretValueCommand } from "@aws-sdk/client-secrets-manager";
import { parseAndVerify } from "./jwt.mjs";
const secrets = new SecretsManagerClient({});
let secretValue;
let secretLoadedAt = 0;

async function loadSigningSecret() {
  if (secretValue && Date.now() - secretLoadedAt < 60_000) return secretValue;
  const result = await secrets.send(new GetSecretValueCommand({ SecretId: process.env.JWT_SECRET_ARN }));
  const value = result.SecretString;
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
