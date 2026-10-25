# Settings

[← Back to the README](../README.md)

Settings go in `.env` (with `docker compose`) or the stack's environment
variables (with Portainer). [`.env.example`](../.env.example) has them all
with comments. The defaults suit AzerothCore's own Docker setup.

The look of the page has its own settings file, `theme.conf`: see
[Theme](../theme/README.md). The longer text is in HTML files: see
[Page text](../content/README.md).

## What the page says

| Setting | Default | What it does |
|---|---|---|
| `SITE_NAME` | `AzerothCore` | The big name at the top, and the page title |
| `TAGLINE` | `Northrend awaits. Your legend begins here.` | The line under it. It shrinks to fit one line, so shorter reads bigger |
| `REALMLIST` | | The address in `set realmlist ...` |
| `CLIENT_DOWNLOAD_URL` | | A client to download. Empty hides the button |
| `CLIENT_HAS_REALMLIST` | `1` | That client already points at the realm, so its players can skip the realmlist step |
| `GAME_VERSION` | `3.3.5a (12340)` | |
| `PUBLIC_URL` | | The site's address, like `https://wow.example.com` |
| `DISCORD_URL` | | An invite to your Discord, like `https://discord.gg/abc123`. It shows in the menu, under How to connect and after sign-up. Empty hides it |
| `AI_PLAYERS` | | How many AI players to mention (like `1500`) when none are counted online, so the realm never sounds empty |

## What it offers

| Setting | Default | What it does |
|---|---|---|
| `REGISTRATION` | `1` | `0` closes sign-ups |
| `CHANGE_PASSWORD` | `1` | `0` turns off password changes |
| `SHOW_ONLINE_PLAYERS` | `1` | |
| `SHOW_TOP_PLAYERS` | `1` | |
| `ONLINE_LIST_SIZE` | `50` | |
| `TOP_LIST_SIZE` | `10` | |
| `HIDE_ACCOUNTS` | | Accounts kept out of the lists (GMs, test accounts), comma separated |
| `BOT_ACCOUNT_PREFIX` | `RNDBOT` | Playerbots' accounts: counted as AI players, never listed, and nobody can sign up with a name starting like this |

## Accounts

| Setting | Default | What it does |
|---|---|---|
| `MIN_PASSWORD_LENGTH` | `8` | Between 4 and 16; the game allows no more than 16 |
| `ACCOUNTS_PER_IP_PER_DAY` | `3` | |
| `ALLOW_SHARED_EMAIL` | `0` | `1` lets accounts share an email address |
| `EXPANSION` | `2` | 2 is Wrath |

## Security

See [Security](security.md#recommended-settings) for what to set these to.

| Setting | Default | What it does |
|---|---|---|
| `SECRET_KEY` | random at each start | Signs the anti-bot challenges |
| `POW_DIFFICULTY` | `18` | Each step doubles the browser's work |
| `TRUSTED_PROXIES` | private networks | Who may set `X-Forwarded-For` |
| `HSTS` | `0` | `1` tells browsers to always use https |

## Database and Docker

| Setting | Default | What it does |
|---|---|---|
| `DB_HOST`, `DB_PORT` | `ac-database`, `3306` | |
| `DB_USER`, `DB_PASSWORD` | `websignup` | Or `DB_PASSWORD_FILE` for a Docker secret |
| `DB_TLS` | `preferred` | `true`, `skip-verify` or `false` |
| `AUTH_DB`, `CHARACTERS_DB` | `acore_auth`, `acore_characters` | |
| `REALM_ID` | `1` | Which realm the status and uptime are for |
| `WORLD_ADDR`, `AUTH_ADDR` | `ac-worldserver:8085`, `ac-authserver:3724` | Checked to tell whether the realm is up |
| `WEB_PORT`, `WEB_BIND` | `8090`, `0.0.0.0` | Where the container is published on the host |
| `AC_NETWORK` | `azerothcore-playerbots_ac-network` | AzerothCore's Docker network |
| `WEBSIGNUP_FILES` | | Portainer only: the folder holding `theme` and `content` |
