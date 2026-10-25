# Security

[← Back to the README](../README.md)

- [How the site protects itself](#how-the-site-protects-itself)
- [Recommended settings](#recommended-settings)
- [Review results](#review-results)
- [Playerbots' account passwords](#playerbots-account-passwords)
- [Things it doesn't try to hide](#things-it-doesnt-try-to-hide)

## How the site protects itself

- **The database user can do very little.** It can look up and add
  accounts and set a new password, read the realm's name and uptime, and
  read the columns of `characters` the lists need. Nothing else, and it
  can't delete anything. Don't give the site root.
- **Passwords never touch the database.** The site makes the same salt and
  SRP6 verifier the server does (checked against AzerothCore's own code),
  and checks the old password the same way for a change.
- **No captcha service.** Each form carries a small proof of work that the
  browser solves while the player types, plus a hidden field bots fill in,
  plus limits on attempts and accounts per address, per account and for the
  whole site. The limits are taken before the work is done, so requests
  sent all at once can't slip past them, and memory for them is capped.
- **Playerbots' own accounts are off limits.** Nobody can sign up with a
  name that starts with `BOT_ACCOUNT_PREFIX`, or change the password of
  one.
- **No cookies and nothing stored in the browser.** Forms are only accepted
  from the site's own page.
- **A strict Content Security Policy:** no inline script or style, nothing
  loaded from anywhere else, no framing.
- **A small container:** a static Go binary on a distroless image, running
  as a non-root user on a read-only filesystem with no capabilities.

## Recommended settings

The defaults work out of the box, but are loose so that they work on any
network. Once the site is behind a reverse proxy, tighten these three.

### `WEB_BIND`: only listen where the proxy can reach

By default the container is published on every address of the host
(`0.0.0.0`), so anything on your LAN can reach port 8090 directly and
skip the proxy. Set `WEB_BIND` to the one host address your proxy connects
to. If the proxy runs in Docker on the same host, you can instead put the
site on the proxy's Docker network and remove the `ports:` lines.

### `TRUSTED_PROXIES`: only believe your proxy

The site counts its per-address limits by the visitor's address (IPv6 by
/64), taken from `X-Forwarded-For`, but only from addresses in
`TRUSTED_PROXIES`. The default trusts every private network, so anything
on those networks that reaches the site directly can pick the address it's
counted as, and get round the limits.

Set it to the address your proxy's requests arrive from: a single address
such as `192.168.1.10` for a proxy on another machine, or the proxy
network's range (from `docker network inspect`) for a proxy in Docker.

Get it right: if the proxy's requests come from an address the site
doesn't trust, every visitor counts as the proxy, and three sign-ups in a
day close the form for everyone.

### `HSTS`: always https

Once the site is only ever reached over https (Force SSL on in the proxy),
set `HSTS=1` so browsers refuse to use plain http for it.

## Review results

The code and setup were reviewed in October 2026. Nothing critical or high
was found. The code-level checks all passed: SQL queries are parameterised,
the SRP6 verifiers match AzerothCore's, theme files are served only from an
allowlist, the Content Security Policy holds, the container is locked down,
and the image build doesn't run untrusted pull request code with secrets.

| Severity | Finding | What to do |
|---|---|---|
| Medium | The defaults trust every private network and listen on every address, so a LAN client or Docker's own port forwarding can spoof `X-Forwarded-For` and skip the per-address limits | Set `WEB_BIND` and `TRUSTED_PROXIES` as [above](#recommended-settings) |
| Medium | Commits merged in GitHub's web page carry the merger's email address | In GitHub's email settings, turn on **Keep my email addresses private** |
| Low | HSTS is off by default | `HSTS=1` once the site is https only |
| Low | The proof of work slows browsers, but a dedicated bot written in native code solves it quickly | Nothing: the per-address and site-wide limits do the real work |
| Low | To stop password guessing, five wrong passwords for an account pause password changes for it on the site for 15 minutes. Because the pause is per account, someone who knows a player's username could set it off on purpose and block that player's password changes for a while | Nothing needed: logging in to the game isn't affected, and the pause ends by itself |
| Low | The database user can read every account's verifier, because a password change needs it | Keep the database off the internet and the user's password secret |
| Low | GM and test accounts show in the player lists | List them in `HIDE_ACCOUNTS` |
| Low | The base images and workflow actions are pulled by tag, not pinned to a digest | Optional: pin them for repeatable builds |

## Playerbots' account passwords

With `Playerbots.RandomBotRandomPassword = 0` in `playerbots.conf`, every
random bot account's password is its name: anyone with the game client can
log in as `RNDBOT1` with the password `RNDBOT1` and take over that bot's
characters. The site refuses those accounts, but it's worth closing on the
server:

1. Set `Playerbots.RandomBotRandomPassword = 1`, so bot accounts made from
   now on get random passwords.
2. Give the existing ones a login nothing can match. Bots never log in with
   a password, so they carry on as before. With a backup taken:
   ```sql
   UPDATE acore_auth.account
      SET salt = RANDOM_BYTES(32), verifier = RANDOM_BYTES(32)
    WHERE username LIKE 'RNDBOT%';
   ```

## Things it doesn't try to hide

- The sign-up form says when an email address already has an account, so
  someone could check whether an address is registered. Hiding it would
  need emailed confirmations.
- To stop password guessing, five wrong passwords for an account in 15
  minutes pause password changes for that account on the site for 15
  minutes. The pause applies to everyone, the owner included, so someone
  who knows a username could set it off deliberately. It only affects
  changing the password on this site: logging in to the game works as
  normal.
