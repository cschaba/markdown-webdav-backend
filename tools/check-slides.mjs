// Runs the slide show in a real browser: keys, clicks, the address, scaling to
// the window, shrinking an overfull slide, and that hidden slides are out of
// reach. None of it can be told from the HTML.
//
//   ./markdown-webdav-backend -no-auth -listen 127.0.0.1:18086 -vault test=./testdata/vault,nogit &
//   chromium --headless=new --remote-debugging-port=9333 --user-data-dir=$(mktemp -d) about:blank &
//   node tools/check-slides.mjs 9333 http://127.0.0.1:18086 [dir for screenshots]
//
// Exits 1 if a check fails.
import { writeFileSync } from "node:fs";
const [port, base, out] = [process.argv[2], process.argv[3], process.argv[4]];
const sleep = ms => new Promise(r => setTimeout(r, ms));
let target;
for (let i = 0; i < 50 && !target; i++) {
  try { target = (await (await fetch(`http://127.0.0.1:${port}/json`)).json()).find(t => t.type === "page"); } catch { await sleep(200); }
}
const ws = new WebSocket(target.webSocketDebuggerUrl);
await new Promise(r => ws.addEventListener("open", r));
let id = 0; const pending = new Map();
ws.addEventListener("message", e => { const m = JSON.parse(e.data); if (m.id && pending.has(m.id)) { pending.get(m.id)(m.result || m.error); pending.delete(m.id); } });
const send = (method, params = {}) => new Promise(r => { pending.set(++id, r); ws.send(JSON.stringify({ id, method, params })); });
const js = async expr => (await send("Runtime.evaluate", { expression: expr, returnByValue: true })).result.value;
const NAMED = { Escape: 27, Enter: 13, Tab: 9, ArrowDown: 40, ArrowUp: 38, ArrowLeft: 37, ArrowRight: 39, Backspace: 8, PageDown: 34, PageUp: 33, Home: 36, End: 35, " ": 32 };
async function press(key, mods = {}) {
  const named = key in NAMED;
  const shifted = !named && (key !== key.toLowerCase() || "?".includes(key));
  const event = { key, modifiers: (shifted || mods.shift ? 8 : 0) | (mods.ctrl ? 2 : 0), windowsVirtualKeyCode: named ? NAMED[key] : key.toUpperCase().charCodeAt(0) };
  if (named) Object.assign(event, { code: key === " " ? "Space" : key, nativeVirtualKeyCode: NAMED[key] }); // named keys do nothing without these
  const text = key === "Enter" ? "\r" : key === " " ? " " : named || mods.ctrl ? undefined : key;
  await send("Input.dispatchKeyEvent", { type: "keyDown", ...event, text });
  await send("Input.dispatchKeyEvent", { type: "keyUp", ...event });
  await sleep(70);
}
const clickAt = async (x, y) => { for (const type of ["mouseMoved", "mousePressed", "mouseReleased"]) await send("Input.dispatchMouseEvent", { type, x, y, button: "left", clickCount: 1 }); await sleep(100); };
const open = async path => { await send("Page.navigate", { url: base + path }); await sleep(1500); };
const current = () => js(`(() => { const c = document.querySelectorAll(".slide.current"); return c.length === 1 ? +c[0].id.replace("slide-", "") : -c.length; })()`);
const shot = async name => { if (out) writeFileSync(`${out}/${name}.png`, Buffer.from((await send("Page.captureScreenshot")).data, "base64")); };

let failures = 0;
const check = (what, ok, detail) => { console.log((ok ? "ok    " : "FAIL  ") + what + (ok ? "" : "  -> " + detail)); if (!ok) failures++; };

await send("Page.enable");
await send("Emulation.setDeviceMetricsOverride", { width: 1000, height: 640, deviceScaleFactor: 1, mobile: false });
await open("/test/-/slides?path=12%20Slides.md");

console.log("--- one slide, filling the window");
check("starts on the first slide, one slide showing", await current() === 1, await current());
const box = JSON.parse(await js(`JSON.stringify((() => { const r = document.querySelector(".slide.current").getBoundingClientRect(); return { w: Math.round(r.width), h: Math.round(r.height), left: Math.round(r.left), top: Math.round(r.top) }; })())`));
check("scaled to the window, 16:9, centred above the bar", box.w === 1000 && Math.abs(box.h - 563) <= 1 && box.left === 0 && box.top >= 0 && box.top + box.h <= 600, JSON.stringify(box));
check("nothing scrolls", await js(`document.documentElement.scrollHeight <= innerHeight && document.documentElement.scrollWidth <= innerWidth`), "page is larger than the window");
await shot("slide-1");

console.log("--- keys");
for (const [key, want] of [["ArrowRight", 2], ["ArrowDown", 3], ["PageDown", 4], [" ", 5], ["l", 6], ["j", 7], ["Enter", 8]]) {
  await press(key);
  check(`${JSON.stringify(key)} goes on`, await current() === want, await current());
}
await press("ArrowRight");
check("the last slide is the end", await current() === 8, await current());
for (const [key, want] of [["ArrowLeft", 7], ["ArrowUp", 6], ["PageUp", 5], ["Backspace", 4], ["h", 3], ["k", 2]]) {
  await press(key);
  check(`${JSON.stringify(key)} goes back`, await current() === want, await current());
}
await press(" ", { shift: true });
check("Shift+Space goes back", await current() === 1, await current());
await press("ArrowLeft");
check("the first slide is the beginning", await current() === 1, await current());
await press("End");
check("End is the last slide", await current() === 8, await current());
await press("Home");
check("Home the first", await current() === 1, await current());
await press("G");
check("G the last", await current() === 8, await current());
await press("g"); await press("g");
check("gg the first", await current() === 1, await current());
await press("l", { ctrl: true });
check("with Ctrl the keys are left alone", await current() === 1, await current());

console.log("--- the address, the counter, what is hidden");
await press("ArrowRight"); await press("ArrowRight");
check("the slide number is in the address", await js("location.hash") === "#3", await js("location.hash"));
check("the counter says so", await js(`document.getElementById("deck-counter").textContent`) === "Slide 3 of 8", await js(`document.getElementById("deck-counter").textContent`));
check("the other slides are inert and hidden from assistive technology", await js(`[...document.querySelectorAll(".slide")].every((s, i) => (i === 2) === !s.inert && s.getAttribute("aria-hidden") === String(i !== 2))`), "not all");
check("the progress bar moves", Math.abs(parseFloat(await js(`document.getElementById("deck-progress").style.width`)) - 28.57) < 0.1, await js(`document.getElementById("deck-progress").style.width`));
await open("/test/-/slides?path=12%20Slides.md#5");
check("a reload, or a link, opens that slide", await current() === 5, await current());
await js(`location.hash = "#2"`); await sleep(150);
check("changing the address changes the slide", await current() === 2, await current());
await js(`location.hash = "#99"`); await sleep(150);
check("a number past the end is the last slide", await current() === 8, await current());

console.log("--- mouse");
await open("/test/-/slides?path=12%20Slides.md#2");
await clickAt(800, 300);
check("a click on the right half goes on", await current() === 3, await current());
await clickAt(200, 300);
check("on the left half, back", await current() === 2, await current());
await js(`document.getElementById("deck-next").click()`);
await js(`document.getElementById("deck-next").click()`);
check("the on-screen buttons work", await current() === 4, await current());
check("at the first slide the back button is disabled", await js(`(location.hash = "#1", true)`) && (await sleep(150), await js(`document.getElementById("deck-prev").disabled && !document.getElementById("deck-next").disabled`)), "not disabled");

console.log("--- an overfull slide, a diagram, a picture");
await open("/test/-/slides?path=12%20Slides.md#6");
const full = JSON.parse(await js(`JSON.stringify((() => { const c = document.querySelector(".slide.current .slide-content"); return { zoom: parseFloat(c.style.zoom || "1"), fits: c.scrollHeight <= c.clientHeight + 1, items: c.querySelectorAll("li").length }; })())`));
check("what does not fit is made smaller until it does", full.zoom < 1 && full.zoom >= 0.3 && full.fits && full.items === 18, JSON.stringify(full));
await shot("slide-6-shrunk");
check("the other slides are left at full size", await js(`[...document.querySelectorAll(".slide")].filter(s => s.querySelector(".slide-content").style.zoom).length`) === 1, "more than one slide was shrunk");
await open("/test/-/slides?path=12%20Slides.md#4");
await sleep(1500);
check("a diagram on a slide that was hidden at load is drawn", await js(`!!document.querySelector("#slide-4 .mermaid svg") && document.querySelector("#slide-4 .mermaid svg").getBoundingClientRect().width > 50`), "no svg, or no width");
await shot("slide-4-diagram");

// A label is cut off when its text reaches below the box Mermaid made for it.
const cut = () => js(`[...document.querySelectorAll(".mermaid foreignObject")].filter(f => { const d = f.firstElementChild; return d && d.getBoundingClientRect().bottom > f.getBoundingClientRect().bottom + 1.5; }).length + "/" + document.querySelectorAll(".mermaid foreignObject").length`);
check("the diagram's labels fit their boxes", /^0\/[1-9]/.test(await cut()), await cut());

console.log("--- the window changes");
await send("Emulation.setDeviceMetricsOverride", { width: 500, height: 800, deviceScaleFactor: 1, mobile: false });
await sleep(300);
const narrow = JSON.parse(await js(`JSON.stringify((() => { const r = document.querySelector(".slide.current").getBoundingClientRect(); return { w: Math.round(r.width), h: Math.round(r.height) }; })())`));
check("the slide follows a narrow window, still 16:9", narrow.w === 500 && Math.abs(narrow.h - 281) <= 1, JSON.stringify(narrow));
check("in the resized window the labels still fit", /^0\/[1-9]/.test(await cut()), await cut());
await send("Emulation.setDeviceMetricsOverride", { width: 1000, height: 640, deviceScaleFactor: 1, mobile: false });

console.log("--- on paper");
await open("/test/-/export?path=12%20Slides.md&format=slides");
await sleep(1500);
check("the export preview is scaled down to the window", parseFloat(await js(`getComputedStyle(document.querySelector(".slide")).zoom`)) < 1, await js(`getComputedStyle(document.querySelector(".slide")).zoom`));
await send("Emulation.setEmulatedMedia", { media: "print" });
await sleep(300);
check("printed at full size, the labels fit: they were measured unscaled", /^0\/[1-9]/.test(await cut()), await cut());
check("the overfull slide is still shrunk to fit on paper", await js(`(() => { const c = document.querySelectorAll(".slide-content")[5]; return parseFloat(c.style.zoom) < 1 && c.scrollHeight <= c.clientHeight + 1; })()`), "does not fit");
await send("Emulation.setEmulatedMedia", { media: "" });
await open("/test/-/slides?path=12%20Slides.md#4");

console.log("--- help, and leaving");
await press("?"); await sleep(200);
const help = JSON.parse(await js(`JSON.stringify({ open: document.getElementById("deck-help").open, rows: document.querySelectorAll("#deck-help tr").length })`));
check("? lists the keys, in a dialog", help.open && help.rows >= 10, JSON.stringify(help));
const before = await current();
await press("ArrowRight");
check("while it is open the slides rest", await current() === before, await current());
// as in the help of the other pages (check-keyboard.mjs): letters scroll the
// help or do nothing, and none is left to the browser's find bar
await send("Emulation.setDeviceMetricsOverride", { width: 1000, height: 300, deviceScaleFactor: 1, mobile: false });
await js(`window.__prevented = {}; addEventListener("keydown", e => { __prevented[e.key] = e.defaultPrevented; })`);
await press("j"); await sleep(200);
const helpJ = await js(`document.getElementById("deck-help").scrollTop`);
await press("G"); await sleep(200);
const helpG = await js(`(d => d.scrollTop + d.clientHeight - d.scrollHeight)(document.getElementById("deck-help"))`);
await press("g"); await press("g"); await sleep(200);
check("j, G and gg scroll the help", helpJ > 0 && Math.abs(helpG) < 4 && await js(`document.getElementById("deck-help").scrollTop`) === 0, helpJ + " " + helpG);
await press("/"); await press("q"); await press("l"); await sleep(300);
const taken = JSON.parse(await js(`JSON.stringify(__prevented)`));
check("the other letters do nothing there, and the browser does not get them", ["/", "q", "l", "j", "G", "g"].every(k => taken[k] === true) && await current() === before && await js(`document.getElementById("deck-help").open`), JSON.stringify(taken));
await send("Emulation.setDeviceMetricsOverride", { width: 1000, height: 640, deviceScaleFactor: 1, mobile: false });
await press("Escape"); await sleep(200);
check("Esc closes it, and does not leave the show", await js(`!document.getElementById("deck-help").open && location.pathname.endsWith("/-/slides")`), await js("location.pathname"));
await press("q"); await sleep(900);
check("q leaves, back to the note", await js("location.pathname") === "/test/12%20Slides", await js("location.pathname"));
await press("g"); await press("s"); await sleep(900);
check("gs on the note opens the slide show", await js("location.pathname + location.search") === "/test/-/slides?path=12%20Slides.md", await js("location.pathname + location.search"));
await press("Escape"); await sleep(900);
check("Esc leaves as well", await js("location.pathname") === "/test/12%20Slides", await js("location.pathname"));

console.log("--- letter keys switched off");
await js(`localStorage.setItem("keys-off", "1")`);
await open("/test/-/slides?path=12%20Slides.md");
await press("l"); await press("j"); await press("G");
check("off: the letters do nothing", await current() === 1, await current());
await press("ArrowRight"); await press(" ");
check("off: arrows and Space still work", await current() === 3, await current());
await js(`localStorage.removeItem("keys-off")`);

ws.close();
console.log(failures ? `\n${failures} check(s) failed` : "\nall checks passed");
process.exit(failures ? 1 : 0);
