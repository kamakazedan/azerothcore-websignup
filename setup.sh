#!/usr/bin/env bash
# First-time setup for the sign-up site, beside an AzerothCore server that
# runs in Docker:
#
#   ./setup.sh
#
# 1. makes .env from .env.example, with a new database password and secret
#    key (an existing .env is left alone),
# 2. checks AzerothCore's Docker network is there,
# 3. creates the database user the site logs in as (asks for the MySQL root
#    password, or reads MYSQL_ROOT_PASSWORD),
# 4. builds and starts the site.
#
# Running it again is safe. DB_CONTAINER=name ./setup.sh if your database
# container isn't called ac-database.
set -euo pipefail
cd "$(dirname "$0")"

say() { printf '\n== %s\n' "$*"; }
die() { printf '\nError: %s\n' "$*" >&2; exit 1; }
random_hex() { od -An -tx1 -N"$1" /dev/urandom | tr -d ' \n'; }
setting() { grep -E "^$1=" .env | tail -n 1 | cut -d= -f2- || true; }

command -v docker >/dev/null || die "Docker isn't installed, or isn't on the PATH."
docker compose version >/dev/null 2>&1 || die "this needs Docker Compose v2 (the 'docker compose' command)."

# 1. Settings ----------------------------------------------------------------
# Secrets only ever pass through shell builtins here, never a command's
# arguments, where other users of the machine could see them.
if [ ! -f .env ]; then
  say "Making .env, with a new database password and secret key"
  db_password=$(random_hex 24)
  secret_key=$(random_hex 32)
  (
    umask 077
    while IFS= read -r line || [ -n "$line" ]; do
      case "$line" in
        DB_PASSWORD=) line="DB_PASSWORD=$db_password" ;;
        SECRET_KEY=) line="SECRET_KEY=$secret_key" ;;
      esac
      printf '%s\n' "$line"
    done < .env.example > .env
  )
  echo "Check what the page says (SITE_NAME, REALMLIST, CLIENT_DOWNLOAD_URL...) in .env."
else
  say "Using the .env that's already here"
fi

DB_PASSWORD=$(setting DB_PASSWORD)
DB_USER=$(setting DB_USER)
AUTH_DB=$(setting AUTH_DB)
CHARACTERS_DB=$(setting CHARACTERS_DB)
AC_NETWORK=$(setting AC_NETWORK)
WEB_PORT=$(setting WEB_PORT)
DB_CONTAINER=${DB_CONTAINER:-ac-database}

[ -n "$DB_PASSWORD" ] || die "DB_PASSWORD is empty in .env."
case "$DB_PASSWORD" in
  *[!A-Za-z0-9._~+=-]*) die "keep DB_PASSWORD to letters, digits and . _ ~ + = - so it survives SQL and .env quoting." ;;
esac
[ "${DB_USER:-websignup}" = websignup ] || die "this script makes the user 'websignup'. Set DB_USER=websignup in .env, or make your own user with sql/create-user.sql."
for name in "${AUTH_DB:-acore_auth}" "${CHARACTERS_DB:-acore_characters}"; do
  [[ "$name" =~ ^[A-Za-z0-9_]+$ ]] || die "database names may only use letters, digits and _ (got '$name')."
done

# 2. Network -----------------------------------------------------------------
AC_NETWORK=${AC_NETWORK:-azerothcore-playerbots_ac-network}
if ! docker network inspect "$AC_NETWORK" >/dev/null 2>&1; then
  echo "Docker networks with ac-network in the name:"
  docker network ls --format '  {{.Name}}' | grep -i 'ac-network' || echo "  (none)"
  die "there's no Docker network called $AC_NETWORK. Set AC_NETWORK in .env to the one your AzerothCore containers are on."
fi

# 3. Database user -----------------------------------------------------------
say "Creating the database user 'websignup' in $DB_CONTAINER"
docker inspect "$DB_CONTAINER" >/dev/null 2>&1 || die "there's no container called $DB_CONTAINER. Run: DB_CONTAINER=name ./setup.sh"
if [ -z "${MYSQL_ROOT_PASSWORD+set}" ]; then
  read -r -s -p "MySQL root password (just Enter to use the one $DB_CONTAINER was started with): " MYSQL_ROOT_PASSWORD || true
  echo
fi
# The root password goes in as an environment variable and then an option
# file on a pipe, the way the MySQL image's own scripts do it, so it never
# shows on a command line. With nothing typed, the container's own
# MYSQL_ROOT_PASSWORD is used.
pass=()
if [ -n "${MYSQL_ROOT_PASSWORD:-}" ]; then
  export MYSQL_ROOT_PASSWORD
  pass=(-e MYSQL_ROOT_PASSWORD)
fi
sql=$(<sql/create-user.sql)
sql=${sql//"{{PASSWORD}}"/"'$DB_PASSWORD'"}
sql=${sql//'`acore_auth`'/"\`${AUTH_DB:-acore_auth}\`"}
sql=${sql//'`acore_characters`'/"\`${CHARACTERS_DB:-acore_characters}\`"}
printf '%s\n' "$sql" |
  docker exec -i ${pass[@]+"${pass[@]}"} "$DB_CONTAINER" bash -c '
    pw=${MYSQL_ROOT_PASSWORD//\\/\\\\}; pw=${pw//\"/\\\"}
    mysql --defaults-extra-file=<(printf "[client]\npassword=\"%s\"\n" "$pw") -uroot' ||
  die "the database user couldn't be made. Is the root password right?"
echo "Done."

# 4. Start -------------------------------------------------------------------
say "Building and starting the site"
docker compose up -d --build

port=${WEB_PORT:-8090}
command -v curl >/dev/null || { say "Started. Check it on port $port."; exit 0; }
for _ in $(seq 1 30); do
  if curl -fsS "http://127.0.0.1:$port/healthz" >/dev/null 2>&1; then
    say "The site is up on port $port"
    echo "Point your reverse proxy (Nginx Proxy Manager) at http://<this machine's address>:$port"
    exit 0
  fi
  sleep 1
done
say "The site started, but isn't answering yet. Its log:"
docker compose logs --tail 30 websignup
