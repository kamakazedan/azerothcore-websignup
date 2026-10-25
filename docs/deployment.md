# Deploying

[← Back to the README](../README.md)

The site runs as one container next to AzerothCore's own, on the same
Docker network. Pick one way to run it:

- [Portainer](#portainer), pulling the image GitHub builds (recommended)
- [`docker compose`](#docker-compose), building on the server

Whichever you choose, it needs [a database user](#the-database-user) and,
for the public, [a reverse proxy](#behind-nginx-proxy-manager) in front.

## The database user

The site logs in to MySQL as its own user, `websignup`, with only the
rights it needs. Create it once.

**The easy way:** `./setup.sh` in the repository. It writes a `.env` with a
fresh password and secret key, creates the user, and starts the site with
`docker compose`. It asks for the MySQL root password; press Enter to use
the one the database container was started with.

**By hand:**

1. `cp .env.example .env`, then set `DB_PASSWORD` (letters and digits are
   safest) and `SECRET_KEY` (`openssl rand -hex 32`).
2. Create the user with that password. This fills it in on the way through,
   so it isn't saved in a file git tracks:
   ```bash
   pw=$(grep '^DB_PASSWORD=' .env | cut -d= -f2)
   sed "s/{{PASSWORD}}/'$pw'/g" sql/create-user.sql | docker exec -i ac-database mysql -uroot -p
   ```

## Portainer

GitHub builds the image every time a change is merged and publishes it as
`ghcr.io/kamakazedan/azerothcore-websignup:latest`
([.github/workflows/image.yml](../.github/workflows/image.yml)).

1. On the server, clone this repository:
   `git clone https://github.com/kamakazedan/azerothcore-websignup.git ~/azerothcore-websignup`.
   Your own files go in its `theme` and `content` folders. Git ignores
   anything you add there, so `git pull` never clashes with them.
2. Create [the database user](#the-database-user) if you haven't.
3. **Stacks → Add stack**, call it `websignup`, choose **Web editor** and
   paste in [`portainer-stack.yml`](../portainer-stack.yml).
4. Under **Environment variables**, add at least:
   - `DB_PASSWORD`: the `websignup` user's password
   - `SECRET_KEY`: 64 hex characters (`openssl rand -hex 32`)
   - `WEBSIGNUP_FILES`: the folder from step 1, like
     `/home/you/azerothcore-websignup`

   then whatever the page should say. The quick way is **Advanced mode**:
   paste in lines written like [`.env.example`](../.env.example), comments
   and all. Every setting is in [Settings](configuration.md).
5. Click **Deploy the stack**.

### Updating

Give GitHub a few minutes to build after a merge (the **Actions** tab shows
it), then open the stack and click **Update the stack** with **Re-pull
image** turned on.

Only the site's code needs this. Files in the theme and content folders
show up within seconds.

<details>
<summary>Updating automatically</summary>

Deploy the stack from **Repository** instead of the web editor (repository
`https://github.com/kamakazedan/azerothcore-websignup`, compose path
`portainer-stack.yml`) and turn on **GitOps updates** (polling), **Force
redeployment** and **Re-pull image**. Portainer then pulls the newest
image every few minutes. Forcing it means it also catches an image that
finished building after the change itself was noticed.
</details>

<details>
<summary>Moving a <code>docker compose</code> install into Portainer</summary>

1. Print the settings it uses, to paste in at step 4:
   `cat ~/azerothcore-websignup/.env`
2. Remove the old container: in Portainer, **Containers**, tick
   `ac-websignup`, **Remove**. Or `cd ~/azerothcore-websignup && docker
   compose down`. The new one takes the same name and port, so the old one
   has to go first, and your reverse proxy needs no change.
3. Follow the steps above, skipping step 2, with `WEBSIGNUP_FILES` set to
   `~/azerothcore-websignup` so the stack keeps your `theme` and `content`
   folders.
</details>

## docker compose

```bash
git clone https://github.com/kamakazedan/azerothcore-websignup.git
cd azerothcore-websignup
./setup.sh
```

Then check the settings in `.env` and run `docker compose up -d` to pick up
any changes. If you made the database user by hand, also check that
`AC_NETWORK` matches AzerothCore's network (`docker network ls | grep
ac-network`) and run `docker compose up -d --build`.

To update:

```bash
git pull
docker compose up -d --build
```

## Behind Nginx Proxy Manager

The site listens on port 8090 of the host. Add a proxy host for your
domain:

- Forward hostname or IP: the Docker host's address. Forward port: `8090`.
  Scheme: `http`.
- Turn on **Block common exploits**. On the SSL tab, request a certificate
  and turn on **Force SSL**.

Then tighten three settings so only the proxy can reach the site and tell
it who visitors are: `WEB_BIND`, `TRUSTED_PROXIES` and `HSTS`.
[Security](security.md#recommended-settings) explains each.
