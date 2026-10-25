// Proof-of-work solver, run in a Web Worker so the page stays responsive.
//
// Finds the first n (0, 1, 2, ...) for which SHA-256(challenge ":" n), n
// written in decimal, starts with `bits` zero bits: the server checks the
// same thing. SHA-256 is written out here rather than taken from
// crypto.subtle, which only exists on https pages and is slow one hash at a
// time. The blocks of the message that never change are hashed once.
(function (root) {
  'use strict';

  const K = new Int32Array([
    0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1, 0x923f82a4, 0xab1c5ed5,
    0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3, 0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174,
    0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc, 0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
    0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7, 0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967,
    0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13, 0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85,
    0xa2bfe8a1, 0xa81a664b, 0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
    0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
    0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208, 0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2,
  ]);
  const IV = [0x6a09e667, 0xbb67ae85, 0x3c6ef372, 0xa54ff53a, 0x510e527f, 0x9b05688c, 0x1f83d9ab, 0x5be0cd19];
  const W = new Int32Array(64);

  // compress runs one 64-byte block (bytes[off..off+64]) into the state H.
  function compress(H, bytes, off) {
    for (let i = 0; i < 16; i++) {
      const j = off + i * 4;
      W[i] = (bytes[j] << 24) | (bytes[j + 1] << 16) | (bytes[j + 2] << 8) | bytes[j + 3];
    }
    for (let i = 16; i < 64; i++) {
      const a = W[i - 15], b = W[i - 2];
      const s0 = ((a >>> 7) | (a << 25)) ^ ((a >>> 18) | (a << 14)) ^ (a >>> 3);
      const s1 = ((b >>> 17) | (b << 15)) ^ ((b >>> 19) | (b << 13)) ^ (b >>> 10);
      W[i] = (W[i - 16] + s0 + W[i - 7] + s1) | 0;
    }
    let a = H[0], b = H[1], c = H[2], d = H[3], e = H[4], f = H[5], g = H[6], h = H[7];
    for (let i = 0; i < 64; i++) {
      const S1 = ((e >>> 6) | (e << 26)) ^ ((e >>> 11) | (e << 21)) ^ ((e >>> 25) | (e << 7));
      const t1 = (h + S1 + ((e & f) ^ (~e & g)) + K[i] + W[i]) | 0;
      const S0 = ((a >>> 2) | (a << 30)) ^ ((a >>> 13) | (a << 19)) ^ ((a >>> 22) | (a << 10));
      const t2 = (S0 + ((a & b) ^ (a & c) ^ (b & c))) | 0;
      h = g; g = f; f = e; e = (d + t1) | 0;
      d = c; c = b; b = a; a = (t1 + t2) | 0;
    }
    H[0] = (H[0] + a) | 0; H[1] = (H[1] + b) | 0; H[2] = (H[2] + c) | 0; H[3] = (H[3] + d) | 0;
    H[4] = (H[4] + e) | 0; H[5] = (H[5] + f) | 0; H[6] = (H[6] + g) | 0; H[7] = (H[7] + h) | 0;
  }

  // ascii turns a string of 7-bit characters into bytes.
  function ascii(s) {
    const out = new Uint8Array(s.length);
    for (let i = 0; i < s.length; i++) out[i] = s.charCodeAt(i) & 0x7f;
    return out;
  }

  // finish hashes tail (the bytes after the last whole block already in H)
  // with SHA-256 padding for a message of total bytes. Uses buf as scratch.
  function finish(H, buf, tailLen, total) {
    buf[tailLen] = 0x80;
    const blocks = tailLen + 9 <= 64 ? 1 : 2;
    const end = blocks * 64;
    buf.fill(0, tailLen + 1, end);
    const bitsLo = (total * 8) >>> 0;
    const bitsHi = Math.floor(total / 0x20000000);
    buf[end - 8] = bitsHi >>> 24; buf[end - 7] = bitsHi >>> 16; buf[end - 6] = bitsHi >>> 8; buf[end - 5] = bitsHi;
    buf[end - 4] = bitsLo >>> 24; buf[end - 3] = bitsLo >>> 16; buf[end - 2] = bitsLo >>> 8; buf[end - 1] = bitsLo;
    compress(H, buf, 0);
    if (blocks === 2) compress(H, buf, 64);
  }

  // sha256hex is plain SHA-256 of an ASCII string, for testing.
  function sha256hex(s) {
    const bytes = ascii(s);
    const H = new Int32Array(IV);
    const whole = bytes.length - (bytes.length % 64);
    for (let off = 0; off < whole; off += 64) compress(H, bytes, off);
    const buf = new Uint8Array(128);
    buf.set(bytes.subarray(whole));
    finish(H, buf, bytes.length - whole, bytes.length);
    let hex = '';
    for (let i = 0; i < 8; i++) hex += (H[i] >>> 0).toString(16).padStart(8, '0');
    return hex;
  }

  // solve returns the nonce, as the decimal string the server wants.
  function solve(challenge, bits) {
    const prefix = ascii(challenge + ':');
    const mid = new Int32Array(IV);
    const whole = prefix.length - (prefix.length % 64);
    for (let off = 0; off < whole; off += 64) compress(mid, prefix, off);
    const tail = prefix.subarray(whole);

    const buf = new Uint8Array(128);
    const H = new Int32Array(8);
    const digits = [48]; // "0", most significant digit first
    for (;;) {
      buf.set(tail);
      let len = tail.length;
      for (let i = 0; i < digits.length; i++) buf[len++] = digits[i];
      H.set(mid);
      finish(H, buf, len, prefix.length + digits.length);
      // bits never goes over 30, so the first word is all that matters.
      if (Math.clz32(H[0]) >= bits) return String.fromCharCode.apply(null, digits);
      // count up in decimal
      let i = digits.length - 1;
      while (i >= 0 && digits[i] === 57) digits[i--] = 48;
      if (i < 0) digits.unshift(49);
      else digits[i]++;
    }
  }

  if (typeof module === 'object' && module.exports) {
    module.exports = { solve, sha256hex };
  } else {
    root.onmessage = (e) => {
      const d = e.data || {};
      const bits = Math.max(0, Math.min(30, d.bits | 0));
      root.postMessage({ nonce: solve(String(d.challenge), bits) });
    };
  }
})(this);
