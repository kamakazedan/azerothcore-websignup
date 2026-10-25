# azerothcore-websignup

A sign-up site for an [AzerothCore](https://www.azerothcore.org/) Wrath of the
Lich King (3.3.5a) server that runs in Docker. One small container, no PHP,
nothing to install on the host.

**[Deploying](docs/deployment.md)** ·
**[Settings](docs/configuration.md)** ·
**[Theme](theme/README.md)** ·
**[Page text](content/README.md)** ·
**[Security](docs/security.md)**

<p>
  <img src="docs/desktop.jpg" alt="The site on a desktop" width="560">
  <img src="docs/phone.jpg" alt="The site on a phone" width="283">
</p>

## What players get

- **Create an account**, ready to log in with straight away, or **change
  their password**.
- **How to connect**: the client download, the `set realmlist` line with a
  copy button, and fixes for the usual problems.
- **Realm status** the way the game's realm list shows it: up or down,
  uptime, population, and how many players and bots are in the world.
- **Who's online** and the **top players**, in their class colours.
- **About the server**, a **guide to playing with bots** and your
  **rules**, in text you change without touching the code.

The top of the page is yours to dress: full-screen background videos that
change between scenes, a picture, or floating particles, from files you
drop into a folder. It works on phones as well as desktops.

## Quick start (Portainer)

GitHub builds the image on every merge and publishes it as
`ghcr.io/kamakazedan/azerothcore-websignup:latest`, so there's nothing to
build on your server.

1. On the server, clone this repository. Your own files go in its `theme`
   and `content` folders, which git leaves alone:
   ```bash
   git clone https://github.com/kamakazedan/azerothcore-websignup.git ~/azerothcore-websignup
   ```
2. Create the site's database user once: run `./setup.sh`, or see
   [Deploying](docs/deployment.md#the-database-user).
3. In Portainer, **Stacks → Add stack**, name it `websignup`, choose **Web
   editor** and paste in [`portainer-stack.yml`](portainer-stack.yml).
4. Under **Environment variables** set at least `DB_PASSWORD`,
   `SECRET_KEY` (`openssl rand -hex 32`) and `WEBSIGNUP_FILES` (the folder
   from step 1), then what the page should say: `SITE_NAME`, `REALMLIST`,
   `PUBLIC_URL` and so on. **Advanced mode** takes lines pasted from
   [`.env.example`](.env.example).
5. **Deploy the stack**, and point your reverse proxy at port 8090.

**To update:** after a change is merged, wait a few minutes for the build
(the **Actions** tab), then open the stack and click **Update the stack**
with **Re-pull image** on. Theme and text files need no update: they show
up within seconds.

Not using Portainer? [Deploying](docs/deployment.md) covers `docker
compose`, Nginx Proxy Manager and moving between the two.

<details>
<summary><b>Try it without a server</b></summary>

Demo mode shows the site with made-up players and touches no database:

```bash
docker build -t websignup .
docker run --rm -p 8090:8080 -e DEMO_MODE=1 -e SITE_NAME=Frosthold websignup
```
</details>

<details>
<summary><b>Working on the code</b></summary>

Go 1.24 or newer. `go test ./...` runs the tests, and `DEMO_MODE=1 go run .`
serves the site on port 8080. Everything the page uses is built into the
binary, so restart after changing a template or stylesheet.

- `main.go`: settings
- `internal/site`: the page, templates, styles, script, forms and theme folder
- `internal/game`: the database, validation, demo data
- `internal/srp6`: AzerothCore's password verifiers
- `internal/pow`: the proof-of-work challenges
- `internal/ratelimit`: the limits
</details>

## Credits

The crystal emblem was drawn for this project. Nothing here comes from
Blizzard. World of Warcraft and Wrath of the Lich King are trademarks of
Blizzard Entertainment, Inc., and this project isn't made or endorsed by
them.

Fonts are under the SIL Open Font License: Cinzel by Natanael Gama, and
Alegreya Sans by Juan Pablo del Peral and Huerta Tipográfica.
See `internal/site/static/fonts/`.
