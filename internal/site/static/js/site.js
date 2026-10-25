// azerothcore-websignup: the page's behaviour. No frameworks, no inline
// script (the Content-Security-Policy forbids it), nothing stored in the
// browser.
(() => {
  'use strict';

  const doc = document;
  const asset = doc.documentElement.dataset.v || '';
  const icon = (name) => `/static/img/icons.svg?v=${asset}#${name}`;
  const announcer = doc.getElementById('announce');
  const announce = (text) => {
    if (!announcer) return;
    announcer.textContent = '';
    setTimeout(() => { announcer.textContent = text; }, 50);
  };

  // ---------------------------------------------------- video backdrop
  // The server lists the theme's videos as scenes, each in the sizes there
  // are of it. Pick a scene, then the smallest size that still covers the
  // backdrop sharply: the portrait one on a phone held upright, 1080p on
  // most desktops, 4K only on screens that need it. Nothing loads for
  // people who've asked for less motion, or whose browser is saving data.
  const backdrop = doc.querySelector('.sky-video[data-video]');
  const still = window.matchMedia('(prefers-reduced-motion: reduce)');
  if (backdrop && !still.matches) {
    let cfg = null;
    try { cfg = JSON.parse(backdrop.dataset.video); } catch (_) { /* no video, then */ }
    const scenes = (cfg && cfg.scenes) || [];
    const conn = navigator.connection || {};
    const thrifty = !!conn.saveData || /(^|-)2g$/.test(conn.effectiveType || '');

    const sizeFor = (scene) => {
      const r = backdrop.getBoundingClientRect();
      const dpr = Math.min(window.devicePixelRatio || 1, 2);
      const want = [r.width * dpr, r.height * dpr];
      const upright = r.height > r.width;
      let sizes = scene.sizes;
      if (cfg.quality) {
        const capped = sizes.filter((v) => !v.w || Math.min(v.w, v.h) <= cfg.quality);
        sizes = capped.length ? capped : sizes.slice(0, 1);
      }
      const sized = sizes.filter((v) => v.w);
      if (!sized.length) return sizes[0];
      const turned = sized.filter((v) => (v.h > v.w) === upright);
      const pool = turned.length ? turned : sized;
      if (thrifty) return pool[0];
      // Up to 15% stretched still looks sharp in motion.
      return pool.find((v) => Math.max(want[0] / v.w, want[1] / v.h) <= 1.15) || pool[pool.length - 1];
    };

    // make builds a scene's video and starts it loading, without playing:
    // it stays invisible until start fades it in.
    const make = (scene) => {
      const v = doc.createElement('video');
      v.muted = true;
      v.defaultMuted = true;
      v.setAttribute('muted', '');
      v.playsInline = true;
      v.setAttribute('playsinline', '');
      v.loop = true;
      v.preload = 'auto';
      v.tabIndex = -1;
      v.disablePictureInPicture = true;
      if (scene.flip) v.classList.add('flip');
      if (scene.poster) v.poster = scene.poster;
      for (const s of sizeFor(scene).src) {
        const source = doc.createElement('source');
        source.src = s.url;
        source.type = s.type;
        v.append(source);
      }
      backdrop.append(v);
      return v;
    };

    // start plays a video from make and fades it in over the last scene,
    // once it's moving and has enough to play through without stalling.
    // The last scene keeps playing until then, so there's never a gap.
    const start = (v, first) => {
      const old = backdrop.querySelector('video.on');
      // The first scene shows its picture at once.
      if (first && v.poster) v.classList.add('on');
      let faded = false;
      const fade = () => {
        if (faded) return;
        faded = true;
        v.classList.add('on');
        if (old && old !== v) {
          old.classList.remove('on');
          setTimeout(() => old.remove(), 2500);
        }
      };
      v.addEventListener('playing', () => {
        if (v.readyState >= 4) fade();
        else {
          v.addEventListener('canplaythrough', fade, { once: true });
          setTimeout(fade, 8000); // a slow line: go anyway
        }
      }, { once: true });
      v.play().catch(() => { /* blocked (battery saver, say): the picture stays */ });
      return v;
    };
    const play = (scene, first) => start(make(scene), first);

    if (scenes.length) {
      let i = scenes.findIndex((s) => s.name === cfg.scene);
      if (i < 0) i = Math.floor(Math.random() * scenes.length);
      let current = play(scenes[i], true);
      let shown = true;
      // Only spend the processor on it while it can be seen.
      const sync = () => {
        const v = backdrop.querySelector('video.on') || current;
        if (shown && doc.visibilityState === 'visible') v.play().catch(() => {});
        else v.pause();
      };
      if ('IntersectionObserver' in window) {
        new IntersectionObserver((entries) => {
          shown = entries[entries.length - 1].isIntersecting;
          sync();
        }).observe(backdrop);
      }
      doc.addEventListener('visibilitychange', sync);
      // Moving on: the next scene starts loading ahead of time, by
      // video_preload percent of video_change (unless the browser is saving
      // data, or it's set to 0), so it's ready to fade in.
      let next = null;
      const live = () => shown && doc.visibilityState === 'visible';
      if (cfg.change > 0 && scenes.length > 1) {
        const every = cfg.change * 1000;
        const lead = every * Math.min(Math.max(cfg.preload || 0, 0), 100) / 100;
        const cycle = () => {
          if (lead > 0) {
            setTimeout(() => {
              if (!next && !thrifty && live()) next = make(scenes[(i + 1) % scenes.length]);
            }, every - lead);
          }
          setTimeout(() => {
            if (live()) {
              i = (i + 1) % scenes.length;
              current = next ? start(next, false) : play(scenes[i], false);
              next = null;
            }
            cycle();
          }, every);
        };
        cycle();
      }
      // A phone turned on its side wants the other shape of video.
      let upright = innerHeight > innerWidth;
      window.addEventListener('resize', () => {
        const now = innerHeight > innerWidth;
        if (now === upright) return;
        upright = now;
        if (next) { next.remove(); next = null; } // loaded for the other shape
        current = play(scenes[i], false);
      });
    }
  }

  // ----------------------------------------------------------- tagline
  // One line, however long the tagline: shrink it until it fits its
  // column, and only let it wrap if it would be too small to read.
  const tagline = doc.querySelector('.tagline');
  if (tagline) {
    const fit = () => {
      tagline.classList.remove('wraps');
      tagline.style.removeProperty('--fit');
      const full = parseFloat(getComputedStyle(tagline).fontSize);
      const room = tagline.clientWidth;
      const need = tagline.scrollWidth;
      if (need <= room) return;
      const size = Math.floor(full * room / need * 10) / 10;
      if (size < 12) tagline.classList.add('wraps');
      else tagline.style.setProperty('--fit', `${size}px`);
    };
    fit();
    if (doc.fonts) doc.fonts.ready.then(fit);
    let pending = 0;
    window.addEventListener('resize', () => {
      cancelAnimationFrame(pending);
      pending = requestAnimationFrame(fit);
    });
  }

  // ----------------------------------------------------------- emblem
  // The emblem stays in the corner; once the page has moved, it gets a
  // backing so it stands out from what's under it.
  const brand = doc.querySelector('.brand');
  if (brand) {
    const lift = () => brand.classList.toggle('lifted', window.scrollY > 24);
    window.addEventListener('scroll', lift, { passive: true });
    lift();
  }

  // ------------------------------------------------------------ menu
  const toggle = doc.querySelector('.menu-toggle');
  const nav = doc.getElementById('site-nav');
  if (toggle && nav) {
    const setOpen = (open) => {
      toggle.setAttribute('aria-expanded', String(open));
      nav.classList.toggle('open', open);
      toggle.querySelector('use').setAttribute('href', icon(open ? 'close' : 'menu'));
    };
    toggle.addEventListener('click', () => setOpen(toggle.getAttribute('aria-expanded') !== 'true'));
    nav.addEventListener('click', (e) => { if (e.target.closest('a')) setOpen(false); });
    doc.addEventListener('keydown', (e) => {
      if (e.key === 'Escape' && nav.classList.contains('open')) {
        setOpen(false);
        toggle.focus();
      }
    });
    doc.addEventListener('click', (e) => {
      if (nav.classList.contains('open') && !nav.contains(e.target) && !toggle.contains(e.target)) setOpen(false);
    });
  }

  // ------------------------------------------------------------ tabs
  const tablist = doc.querySelector('[role="tablist"]');
  if (tablist) {
    const tabs = Array.from(tablist.querySelectorAll('[role="tab"]'));
    const select = (tab, focus) => {
      for (const t of tabs) {
        const on = t === tab;
        t.setAttribute('aria-selected', String(on));
        t.tabIndex = on ? 0 : -1;
        doc.getElementById(t.getAttribute('aria-controls')).hidden = !on;
      }
      if (focus) tab.focus();
    };
    tablist.addEventListener('click', (e) => {
      const tab = e.target.closest('[role="tab"]');
      if (tab) select(tab, false);
    });
    tablist.addEventListener('keydown', (e) => {
      const i = tabs.indexOf(doc.activeElement);
      if (i < 0) return;
      const to = { ArrowRight: i + 1, ArrowLeft: i - 1, Home: 0, End: tabs.length - 1 }[e.key];
      if (to === undefined) return;
      e.preventDefault();
      select(tabs[(to + tabs.length) % tabs.length], true);
    });
    // #change-password opens that tab, so it can be linked to.
    const fromHash = () => {
      const tab = tabs.find((t) => t.dataset.hash && `#${t.dataset.hash}` === location.hash);
      if (tab) {
        select(tab, false);
        doc.getElementById('account').scrollIntoView();
      }
    };
    window.addEventListener('hashchange', fromHash);
    fromHash();
  }

  // ------------------------------------------------- show the password
  for (const btn of doc.querySelectorAll('.reveal')) {
    btn.addEventListener('click', () => {
      const input = doc.getElementById(btn.getAttribute('aria-controls'));
      const show = input.type === 'password';
      input.type = show ? 'text' : 'password';
      btn.setAttribute('aria-pressed', String(show));
      btn.querySelector('use').setAttribute('href', icon(show ? 'eye-off' : 'eye'));
    });
  }

  // ------------------------------------------------------------ copy
  const copyText = async (text) => {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch (_) {
      return false;
    }
  };
  const selectText = (el) => {
    const range = doc.createRange();
    range.selectNodeContents(el);
    const sel = window.getSelection();
    sel.removeAllRanges();
    sel.addRange(range);
  };
  doc.addEventListener('click', async (e) => {
    const btn = e.target.closest('[data-copy]');
    if (!btn) return;
    const src = doc.getElementById(btn.dataset.copy);
    if (!src) return;
    const label = btn.querySelector('.label');
    const use = btn.querySelector('use');
    const ok = await copyText(src.textContent.trim());
    if (ok) {
      label.textContent = 'Copied';
      use.setAttribute('href', icon('check'));
      announce('Copied to the clipboard.');
    } else {
      // No clipboard access (an http:// page, say): select it instead.
      selectText(src);
      label.textContent = 'Selected';
      announce('The line is selected. Copy it with your keyboard or menu.');
    }
    clearTimeout(btn.copyTimer);
    btn.copyTimer = setTimeout(() => {
      label.textContent = 'Copy';
      use.setAttribute('href', icon('copy'));
    }, 2500);
  });

  // --------------------------------------------------- proof of work
  // Every form carries a solved challenge from the server. Solving takes a
  // moment of the browser's time, so it starts as soon as someone begins
  // filling a form in, and is usually done before they press the button.
  const work = {
    worker: null,
    job: null,
    start() {
      if (!this.job) {
        this.job = this.solve();
        this.job.catch(() => { this.job = null; });
      }
      return this.job;
    },
    async solve() {
      const res = await fetch('/api/challenge', { headers: { Accept: 'application/json' }, cache: 'no-store' });
      const data = await res.json().catch(() => ({}));
      if (!res.ok || !data.challenge) {
        throw new Error(data.message || "The anti-bot check couldn't start. Please try again.");
      }
      const nonce = await this.run(data.challenge, data.bits);
      return { challenge: data.challenge, nonce, at: Date.now() };
    },
    run(challenge, bits) {
      return new Promise((resolve, reject) => {
        if (!window.Worker) {
          reject(new Error('This browser is too old for the anti-bot check. Please try another.'));
          return;
        }
        if (!this.worker) this.worker = new Worker(`/static/js/pow-worker.js?v=${asset}`);
        const w = this.worker;
        const done = () => {
          w.removeEventListener('message', onMessage);
          w.removeEventListener('error', onError);
        };
        const onMessage = (e) => {
          done();
          if (e.data && typeof e.data.nonce === 'string') resolve(e.data.nonce);
          else reject(new Error('The anti-bot check failed. Please try again.'));
        };
        const onError = () => {
          done();
          w.terminate();
          this.worker = null;
          reject(new Error('The anti-bot check failed. Please try again.'));
        };
        w.addEventListener('message', onMessage);
        w.addEventListener('error', onError);
        w.postMessage({ challenge, bits });
      });
    },
    // take hands out a solved challenge for one submission; it can't be
    // used twice.
    async take() {
      const sol = await this.start();
      this.job = null;
      // Challenges last ten minutes; don't risk one that's nearly done.
      if (Date.now() - sol.at > 8 * 60 * 1000) return this.take();
      return sol;
    },
  };

  // ------------------------------------------------------------ forms
  const USERNAME = /^[A-Za-z0-9]+$/;
  const PRINTABLE = /^[\x21-\x7e]+$/;

  const checkPassword = (pw, user, min, field) => {
    if (pw.length < min || pw.length > 16) {
      return { field, message: `Passwords are ${min} to 16 characters (the game allows no more than 16).` };
    }
    if (!PRINTABLE.test(pw)) {
      return { field, message: 'Passwords can use letters, digits and symbols, but no spaces or accents.' };
    }
    if (user && pw.toLowerCase() === user.toLowerCase()) {
      return { field, message: "Your password can't be your username." };
    }
    return null;
  };

  // The same rules the server applies, so most mistakes show up at once.
  const rules = {
    register(v, min) {
      if (v.username.length < 3 || v.username.length > 16) {
        return { field: 'username', message: 'Usernames are 3 to 16 letters or digits.' };
      }
      if (!USERNAME.test(v.username)) {
        return { field: 'username', message: 'Usernames can only use letters and digits.' };
      }
      const email = v.email.trim();
      if (!email) return { field: 'email', message: 'Please enter your email address.' };
      if (!PRINTABLE.test(email)) {
        return { field: 'email', message: 'Please use a plain email address (no spaces or accents).' };
      }
      if (email.length > 254 || !/^[^@]+@[^@.][^@]*\.[^@]*[^@.]$/.test(email)) {
        return { field: 'email', message: "That doesn't look like an email address." };
      }
      return checkPassword(v.password, v.username, min, 'password')
        || (v.password !== v.password2 ? { field: 'password2', message: "The two passwords don't match." } : null);
    },
    password(v, min) {
      if (!v.username.trim()) return { field: 'username', message: 'Please enter your username.' };
      if (!v.password) return { field: 'password', message: 'Please enter your current password.' };
      return checkPassword(v.newPassword, v.username.trim(), min, 'newPassword')
        || (v.newPassword !== v.newPassword2 ? { field: 'newPassword2', message: "The two new passwords don't match." } : null);
    },
  };

  let busy = false;

  for (const form of doc.querySelectorAll('form[data-kind]')) {
    const kind = form.dataset.kind;
    const min = Math.max(4, parseInt(form.dataset.min, 10) || 8);
    const button = form.querySelector('button[type="submit"]');
    const label = button.querySelector('.label');
    const idle = label.textContent;
    const alert = form.querySelector('.alert');
    const inputs = Array.from(form.querySelectorAll('input[name]'));
    const byName = Object.fromEntries(inputs.map((i) => [i.name, i]));

    const errorFor = (input) => doc.getElementById(`${input.id}-error`);
    const clearField = (input) => {
      input.removeAttribute('aria-invalid');
      const el = errorFor(input);
      if (el) { el.hidden = true; el.textContent = ''; }
    };
    const showField = (name, message) => {
      const input = byName[name];
      const el = input && errorFor(input);
      if (!el) { showAlert(message); return; }
      input.setAttribute('aria-invalid', 'true');
      el.textContent = message;
      el.hidden = false;
      input.focus();
    };
    const showAlert = (message, ok) => {
      alert.textContent = message;
      alert.classList.toggle('ok', !!ok);
      alert.hidden = false;
    };
    const clearAll = () => {
      inputs.forEach(clearField);
      alert.hidden = true;
      alert.textContent = '';
    };
    const pending = (text) => {
      form.setAttribute('aria-busy', text ? 'true' : 'false');
      button.disabled = !!text;
      label.textContent = text || idle;
    };

    form.addEventListener('focusin', () => { work.start(); }, { once: true });
    form.addEventListener('input', (e) => { if (e.target.name) clearField(e.target); });

    form.addEventListener('submit', async (e) => {
      e.preventDefault();
      if (busy) return;
      clearAll();

      const values = {};
      for (const input of inputs) values[input.name] = input.value;
      const problem = rules[kind](values, min);
      if (problem) {
        showField(problem.field, problem.message);
        return;
      }
      if (kind === 'register') values.email = values.email.trim();
      if (kind === 'password') values.username = values.username.trim();

      busy = true;
      let ok = false;
      try {
        pending('Checking you’re not a bot…');
        const sol = await work.take();
        pending(kind === 'register' ? 'Creating your account…' : 'Changing your password…');
        const res = await fetch(form.getAttribute('action'), {
          method: 'POST',
          headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
          body: JSON.stringify({ ...values, challenge: sol.challenge, nonce: sol.nonce }),
        });
        const data = await res.json().catch(() => null);
        if (!data) throw new Error("The server didn't answer properly. Please try again in a minute.");
        if (data.ok) {
          ok = true;
          succeeded(form, kind, data.message, values);
          return;
        }
        if (data.field) showField(data.field, data.message);
        else showAlert(data.message);
      } catch (err) {
        showAlert(err instanceof TypeError
          ? "Couldn't reach the server. Check your connection and try again."
          : err.message);
      } finally {
        busy = false;
        pending('');
        // A challenge is good for one try; have the next one ready.
        if (!ok) work.start();
      }
    });
  }

  function succeeded(form, kind, message, values) {
    if (kind === 'password') {
      form.reset();
      const alert = form.querySelector('.alert');
      alert.textContent = message;
      alert.classList.add('ok');
      alert.hidden = false;
      announce(message);
      return;
    }
    const pane = form.parentElement;
    const done = pane.querySelector('.done');
    done.querySelector('[data-done-message]').textContent = message;
    done.querySelector('[data-done-user]').textContent = values.username.toUpperCase();
    form.reset();
    form.hidden = true;
    done.hidden = false;
    done.querySelector('.done-title').focus();
  }

  for (const again of doc.querySelectorAll('[data-again]')) {
    again.addEventListener('click', () => {
      const pane = again.closest('.pane');
      pane.querySelector('.done').hidden = true;
      const form = pane.querySelector('form');
      form.hidden = false;
      form.querySelector('input').focus();
    });
  }

  // ------------------------------------------------- live realm status
  // Every minute, while the page is in view, swap in fresh copies of the
  // realm status and player lists, rendered by the server.
  const regions = ['realm-plaque', 'online-list', 'top-list'];
  let last = Date.now();
  const refresh = async () => {
    if (doc.visibilityState !== 'visible') return;
    last = Date.now();
    try {
      const res = await fetch('/api/live', { cache: 'no-store' });
      if (!res.ok) return;
      const fresh = new DOMParser().parseFromString(await res.text(), 'text/html');
      for (const id of regions) {
        const now = doc.getElementById(id);
        const next = fresh.getElementById(id);
        if (!now || !next) continue;
        // Fresh copies skip the page's opening fade-in: the status box
        // shouldn't vanish and come back every minute.
        const node = doc.importNode(next, true);
        node.classList.add('settled');
        now.replaceWith(node);
      }
    } catch (_) {
      // Try again next time.
    }
  };
  if (regions.some((id) => doc.getElementById(id))) {
    setInterval(refresh, 60000);
    doc.addEventListener('visibilitychange', () => {
      if (doc.visibilityState === 'visible' && Date.now() - last > 60000) refresh();
    });
  }
})();
