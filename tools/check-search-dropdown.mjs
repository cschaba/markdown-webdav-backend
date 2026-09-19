// Checks the recent-searches dropdown in a real browser: a screenshot cannot
// show what only opens on focus, and an HTML test cannot tell whether a click
// on an entry survives the focus change. Drives headless Chromium over the
// DevTools protocol with nothing but Node's built-in fetch and WebSocket.
//
//   ./markdown-webdav-backend -no-auth -listen 127.0.0.1:18086 -vault test=./testdata/vault,nogit &
//   chromium --headless=new --remote-debugging-port=9333 --user-data-dir=$(mktemp -d) about:blank &
//   node tools/check-search-dropdown.mjs 9333 http://127.0.0.1:18086 /tmp/out
//
// Prints what it found and writes desktop.png and phone-dark-scrolled.png to
// the output directory. Expected: closed, then open after a click on the
// magnifier, ten entries newest first, more content than fits, and a click on
// an entry leading to that search.
import { writeFileSync } from "node:fs";
const [port, base, out] = [process.argv[2], process.argv[3], process.argv[4]];
const sleep = ms => new Promise(r => setTimeout(r, ms));
let target;
// A browser starting cold on a CI runner can take its time; and one that is
// up but has no page yet must be waited for too, not asked 50 times at once.
for (const deadline = Date.now() + 60000; !target && Date.now() < deadline; ) {
  try { target = (await (await fetch(`http://127.0.0.1:${port}/json`)).json()).find(t => t.type === "page"); } catch { /* not listening yet */ }
  if (!target) await sleep(200);
}
if (!target) { console.error(`no browser page on port ${port} after 60 s`); process.exit(2); }
const ws = new WebSocket(target.webSocketDebuggerUrl);
await new Promise(r => ws.addEventListener("open", r));
let id = 0; const pending = new Map();
ws.addEventListener("message", e => { const m = JSON.parse(e.data); if (m.id && pending.has(m.id)) { pending.get(m.id)(m.result || m.error); pending.delete(m.id); } });
const send = (method, params = {}) => new Promise(r => { pending.set(++id, r); ws.send(JSON.stringify({ id, method, params })); });
const evaluate = async expr => (await send("Runtime.evaluate", { expression: expr, returnByValue: true })).result.value;
const go = async url => { await send("Page.navigate", { url }); await sleep(700); };
const shot = async name => writeFileSync(`${out}/${name}.png`, Buffer.from((await send("Page.captureScreenshot")).data, "base64"));
await send("Page.enable");

await send("Emulation.setDeviceMetricsOverride", { width: 900, height: 560, deviceScaleFactor: 1, mobile: false });
for (const q of ["zeppelin", "pixel", "tag:test", '"rigid frame"', "dirigible", "airship", "checkerboard", "1937", "mermaid", "footnote", "path:search/", "excalidraw plugin export"]) {
  await go(`${base}/test/-/search?q=${encodeURIComponent(q)}`);
}
await go(`${base}/test/01%20Formatting`);
console.log("closed before focus:", await evaluate(`getComputedStyle(document.querySelector(".search-history")).display`));
// open it the way a person would: a real click on the magnifier
const icon = await evaluate(`(() => { const r = document.querySelector(".search label").getBoundingClientRect(); return { x: r.x + r.width / 2, y: r.y + r.height / 2 }; })()`);
for (const type of ["mouseMoved", "mousePressed", "mouseReleased"]) await send("Input.dispatchMouseEvent", { type, x: icon.x, y: icon.y, button: "left", clickCount: 1 });
await sleep(200);
console.log("field focused:     ", await evaluate(`document.activeElement === document.querySelector("#search-q")`));
console.log("open on icon click:", await evaluate(`getComputedStyle(document.querySelector(".search-history")).display`));
console.log("entries, top first:", await evaluate(`[...document.querySelectorAll(".search-history a")].map(a => a.textContent).join(" | ")`));
console.log("scrollable:        ", await evaluate(`(() => { const l = document.querySelector(".search-history"); return l.scrollHeight + "px of content in " + l.clientHeight + "px"; })()`));
await shot("desktop");

// a real click on the third entry: does the list survive mouse-down, does it navigate?
const box = await evaluate(`(() => { const r = document.querySelectorAll(".search-history a")[2].getBoundingClientRect(); return { x: r.x + r.width / 2, y: r.y + r.height / 2 }; })()`);
const third = await evaluate(`document.querySelectorAll(".search-history a")[2].textContent`);
for (const type of ["mouseMoved", "mousePressed", "mouseReleased"]) await send("Input.dispatchMouseEvent", { type, x: box.x, y: box.y, button: "left", clickCount: 1 });
await sleep(900);
console.log(`clicked "${third}" ->`, await evaluate(`location.pathname + location.search`), "| box shows:", await evaluate(`document.querySelector("input[name=q]").value`));
console.log("now on top:        ", await evaluate(`document.querySelector(".search-history a").textContent`));

// phone width, dark
await send("Emulation.setDeviceMetricsOverride", { width: 390, height: 640, deviceScaleFactor: 2, mobile: true });
await send("Emulation.setEmulatedMedia", { features: [{ name: "prefers-color-scheme", value: "dark" }] });
await go(`${base}/test/01%20Formatting`);
await evaluate(`document.querySelector("input[name=q]").focus()`);
await sleep(200);
await evaluate(`document.querySelector(".search-history").scrollTop = 1000`);
await sleep(100);
console.log("phone, fits screen:", await evaluate(`(() => { const r = document.querySelector(".search-history").getBoundingClientRect(); return r.left >= 0 && r.right <= innerWidth; })()`));
await shot("phone-dark-scrolled");
ws.close();
