# Your own text

The page's longer text comes in four parts. To change one, put a file with
its name in this folder; the site notices within a few seconds, with no
restart.

| File           | Where it goes                                 |
|----------------|-----------------------------------------------|
| `connect.html` | Under the three connection steps (tips)       |
| `about.html`   | "What's running on ..."                       |
| `bots.html`    | "Playing with bots"                           |
| `rules.html`   | A rules section. Empty unless you add it      |

Start from the built-in versions in `internal/site/content/`. They're plain
HTML: headings, paragraphs and lists. A few marks are filled in for you:
`{{REALM}}`, `{{REALMLIST}}`, `{{DOWNLOAD_URL}}` and `{{GAME_VERSION}}`.

An empty file hides its section altogether. Delete the file to go back to
the built-in text.

Two styles you can use: `<dl class="pairs">` for a list of commands or
problems and what to do about them, and `<dl class="features">` for a list
of features. Look at the built-in files for how they're written.
