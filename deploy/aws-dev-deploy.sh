#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "Usage: ras-backend-deploy.sh migrate <image-sha> | auth-api|branch-api|attendance-api <image-sha>" >&2
  exit 2
}
[[ $# -eq 2 ]] || usage
operation="$1"
image_tag="$2"
[[ "$image_tag" =~ ^[A-Fa-f0-9]{7,64}$ ]] || { echo "Image tag must be a commit SHA." >&2; exit 2; }
: "${AWS_REGION:?AWS_REGION is required}"
: "${APP_SECRET_ARN:?APP_SECRET_ARN is required}"
: "${DATABASE_MASTER_SECRET_ARN:?DATABASE_MASTER_SECRET_ARN is required}"
: "${DATABASE_ENDPOINT:?DATABASE_ENDPOINT is required}"
: "${DB_NAME:?DB_NAME is required}"
: "${MIGRATION_IMAGE_REPOSITORY:?MIGRATION_IMAGE_REPOSITORY is required}"

secret_json="$(aws secretsmanager get-secret-value --region "$AWS_REGION" --secret-id "$APP_SECRET_ARN" --query SecretString --output text)"
master_json="$(aws secretsmanager get-secret-value --region "$AWS_REGION" --secret-id "$DATABASE_MASTER_SECRET_ARN" --query SecretString --output text)"
for key in JWT_SECRET AUTH_DB_PASSWORD BRANCH_DB_PASSWORD ATTENDANCE_DB_PASSWORD SEED_CEO_EMAIL SEED_CEO_PASSWORD; do
  jq -e --arg key "$key" '.[$key] | strings | length > 0' <<<"$secret_json" >/dev/null || { echo "Required app secret field missing: $key" >&2; exit 1; }
done
jq -e '.JWT_SECRET | test("^[A-Za-z0-9._~-]{32,}$")' <<<"$secret_json" >/dev/null || { echo "JWT_SECRET must be at least 32 URL-safe characters." >&2; exit 1; }
jq -e '.SEED_CEO_EMAIL | test("^[^[:space:]@]+@[^[:space:]@]+\\.[^[:space:]@]+$")' <<<"$secret_json" >/dev/null || { echo "SEED_CEO_EMAIL is invalid." >&2; exit 1; }
jq -e '[.AUTH_DB_PASSWORD, .BRANCH_DB_PASSWORD, .ATTENDANCE_DB_PASSWORD, .SEED_CEO_PASSWORD, (.SEED_CEO_NAME // "CEO")] | all(strings and (test("^[A-Za-z0-9._~-]+$")))' <<<"$secret_json" >/dev/null || { echo "Application passwords and CEO name must use URL-safe characters." >&2; exit 1; }
jq -e '.username | strings | length > 0' <<<"$master_json" >/dev/null || { echo "RDS master username missing from managed secret." >&2; exit 1; }
jq -e '.password | strings | length > 0' <<<"$master_json" >/dev/null || { echo "RDS master password missing from managed secret." >&2; exit 1; }

secret_dir=/etc/ras-backend
install -d -m 0700 "$secret_dir"
umask 077
write_app_env() {
  local service="$1" role="$2" password_key="$3" port="$4" file="$5"
  jq -er --arg service "$service" --arg role "$role" --arg password_key "$password_key" --arg port "$port" --arg host "$DATABASE_ENDPOINT" --arg db "$DB_NAME" '
    . as $s |
    ($s.JWT_SECRET + $s[$password_key] + $s.SEED_CEO_EMAIL + $s.SEED_CEO_PASSWORD) as $all |
    if ($all | test("^[A-Za-z0-9._~-]+$")) then
      "APP_ENV=development\nPORT=\($port)\nDB_HOST=\($host)\nDB_PORT=5432\nDB_NAME=\($db)\nDB_USER=\($role)\nDB_PASSWORD=\($s[$password_key])\nDB_SSLMODE=require\nJWT_SECRET=\($s.JWT_SECRET)\nSEED_CEO_NAME=\($s.SEED_CEO_NAME // "CEO")\nSEED_CEO_EMAIL=\($s.SEED_CEO_EMAIL)\nSEED_CEO_PASSWORD=\($s.SEED_CEO_PASSWORD)"
    else error("Secret values must use URL-safe characters for env-file delivery") end
  ' <<<"$secret_json" >"$file"
  chmod 0600 "$file"
}

if [[ "$operation" == "migrate" ]]; then
  repo="$MIGRATION_IMAGE_REPOSITORY"
  registry="${repo%%/*}"
  aws ecr get-login-password --region "$AWS_REGION" | docker login --username AWS --password-stdin "$registry" >/dev/null
  image="$repo:$image_tag"
  docker pull "$image" >/dev/null
  master_user="$(jq -er '.username' <<<"$master_json")"
  master_password="$(jq -er '.password' <<<"$master_json")"
  printf '%s\n' \
    "PGHOST=$DATABASE_ENDPOINT" \
    "PGPORT=5432" \
    "PGDATABASE=$DB_NAME" \
    "PGUSER=$master_user" \
    "PGPASSWORD=$master_password" \
    "AUTH_DB_PASSWORD=$(jq -er '.AUTH_DB_PASSWORD' <<<"$secret_json")" \
    "BRANCH_DB_PASSWORD=$(jq -er '.BRANCH_DB_PASSWORD' <<<"$secret_json")" \
    "ATTENDANCE_DB_PASSWORD=$(jq -er '.ATTENDANCE_DB_PASSWORD' <<<"$secret_json")" \
    >"$secret_dir/migration.env"
  chmod 0600 "$secret_dir/migration.env"
  docker run --rm --network host --env-file "$secret_dir/migration.env" --entrypoint /bin/sh "$image" -ec \
    'psql -v ON_ERROR_STOP=1 -v auth_password="$AUTH_DB_PASSWORD" -v branch_password="$BRANCH_DB_PASSWORD" -v attendance_password="$ATTENDANCE_DB_PASSWORD" -f /database/bootstrap/roles.sql'
  encoded_master_user="$(python3 -c 'import sys,urllib.parse; print(urllib.parse.quote(sys.argv[1], safe=""))' "$master_user")"
  encoded_master_password="$(python3 -c 'import sys,urllib.parse; print(urllib.parse.quote(sys.argv[1], safe=""))' "$master_password")"
  goose_dbstring="postgres://$encoded_master_user:$encoded_master_password@$DATABASE_ENDPOINT:5432/$DB_NAME?sslmode=require"
  printf '%s\n' \
    'GOOSE_DRIVER=postgres' \
    "GOOSE_DBSTRING=$goose_dbstring" \
    'GOOSE_MIGRATION_DIR=/migrations' \
    'GOOSE_TABLE=goose_db_version' \
    >"$secret_dir/goose.env"
  chmod 0600 "$secret_dir/goose.env"
  docker run --rm --network host --env-file "$secret_dir/goose.env" "$image" up
  rm -f "$secret_dir/migration.env" "$secret_dir/goose.env"
  echo "Central migrations applied using image $image_tag."
  exit 0
fi

case "$operation" in
  auth-api) repo="${AUTH_IMAGE_REPOSITORY:?AUTH_IMAGE_REPOSITORY is required}"; role=auth_app; password_key=AUTH_DB_PASSWORD; port=9001 ;;
  branch-api) repo="${BRANCH_IMAGE_REPOSITORY:?BRANCH_IMAGE_REPOSITORY is required}"; role=branch_app; password_key=BRANCH_DB_PASSWORD; port=9002 ;;
  attendance-api) repo="${ATTENDANCE_IMAGE_REPOSITORY:?ATTENDANCE_IMAGE_REPOSITORY is required}"; role=attendance_app; password_key=ATTENDANCE_DB_PASSWORD; port=9003 ;;
  *) usage ;;
esac
registry="${repo%%/*}"
aws ecr get-login-password --region "$AWS_REGION" | docker login --username AWS --password-stdin "$registry" >/dev/null
image="$repo:$image_tag"
docker pull "$image" >/dev/null
env_file="$secret_dir/$operation.env"
write_app_env "$operation" "$role" "$password_key" "$port" "$env_file"
if docker container inspect "$operation" >/dev/null 2>&1; then docker stop --time 15 "$operation" >/dev/null; docker rm "$operation" >/dev/null; fi
docker run -d --restart unless-stopped --name "$operation" --env-file "$env_file" -p "$port:$port" "$image" >/dev/null
rm -f "$env_file"
echo "Started $operation from immutable image $image_tag. Verify its /health endpoint before closing the change."
