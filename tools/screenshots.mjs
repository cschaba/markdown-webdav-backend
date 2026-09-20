// Takes the screenshots for the README's slideshow, from the test vault - never
// from a real one, whose notes would be published. tools/screenshots.sh runs it
// and assembles the pictures; usage there.
//
//   node tools/screenshots.mjs <debug port> <server base URL> <output dir>
//
// Writes <output dir>/NN.png and NN.txt, the caption.
import { writeFileSync } from "node:fs";

const [port, base, out] = process.argv.slice(2);
const sleep = ms => new Promise(r => setTimeout(r, ms));
let target;
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
const js = expr => send("Runtime.evaluate", { expression: expr, awaitPromise: true });
async function press(key) {
  const shifted = key !== key.toLowerCase() || key === "?";
  const event = { key, modifiers: shifted ? 8 : 0, windowsVirtualKeyCode: key.toUpperCase().charCodeAt(0) };
  await send("Input.dispatchKeyEvent", { type: "keyDown", ...event, text: key });
  await send("Input.dispatchKeyEvent", { type: "keyUp", ...event });
  await sleep(100);
}

const WIDTH = 1200, HEIGHT = 750;
await send("Page.enable");
await send("Emulation.setDeviceMetricsOverride", { width: WIDTH, height: HEIGHT, deviceScaleFactor: 1, mobile: false });

// path, caption, and what to do before the picture is taken
const SHOTS = [
  { path: "/test/00%20Index", caption: "A note: its tags, properties and links, rendered from Obsidian's Markdown" },
  { path: "/test/02%20Code%20and%20Diagrams", caption: "Syntax highlighting and Mermaid diagrams", wait: 3000,
    // the highlighted code and the diagram under it, in one view
    then: `(pre => pre && scrollTo(0, pre.getBoundingClientRect().top + scrollY - 110))(document.querySelector("article pre:not(.mermaid)"))` },
  { path: "/test/09%20Embedded%20notes", caption: "Embedded notes, shown in place" },
  { path: "/test/13%20Callouts", caption: "Callouts, in Obsidian's types and colours, foldable without JavaScript" },
  { path: "/test/05%20Excalidraw", caption: "Excalidraw drawings, through the picture the plugin exports", wait: 1000 },
  { path: "/test/-/graph", caption: "The graph of notes and links, coloured by tag", wait: 4000 },
  { path: "/test/-/search?q=zeppelin", caption: "Search, best match first; also by tag, path and open or done tasks" },
  { path: "/test/06%20Search", caption: "A note's statistics, shown and hidden with the key i", keys: "i" },
  { path: "/test/07%20Links", caption: "Vim-style keys: f puts a letter on every link, typing it follows the link", keys: "f" },
  { path: "/test/11%20Keyboard", caption: "Every key in one help, and a switch to turn them off", keys: "?" },
  { path: "/test/-/slides?path=12%20Slides.md", caption: "A note as a slide show, split the Obsidian way", wait: 1500, keys: "l" },
  { path: "/test/-/export?path=10%20Printing.md", caption: "Export to PDF in book format", wait: 1500 },
  { path: "/test/01%20Formatting", caption: "Dark mode follows the system", dark: true },
  { path: "/test/-/about", caption: "The version, the source, and the vault counted" },
];

for (const [i, shot] of SHOTS.entries()) {
  await send("Emulation.setEmulatedMedia", { features: [{ name: "prefers-color-scheme", value: shot.dark ? "dark" : "light" }] });
  await send("Page.navigate", { url: base + shot.path });
  await sleep(1200 + (shot.wait || 0));
  if (shot.then) await js(shot.then);
  for (const key of shot.keys || "") await press(key);
  await sleep(400);
  const { data } = await send("Page.captureScreenshot", { format: "png" });
  const name = String(i + 1).padStart(2, "0");
  writeFileSync(`${out}/${name}.png`, Buffer.from(data, "base64"));
  writeFileSync(`${out}/${name}.txt`, shot.caption);
  console.log(`${name}  ${shot.caption}`);
  // What a key switched on here (the statistics) must not show on the next.
  await js(`try { localStorage.clear() } catch (e) {}`);
}
process.exit(0);
