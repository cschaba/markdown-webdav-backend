// The slide show: one slide at a time, scaled to the window, changed by
// keyboard, mouse and touch. Without this script the slides are simply a page
// to scroll through; the stylesheet only takes over once "js" is set below.
//
// Also used by the PDF export's slide format, for the one thing both need:
// making a slide with too much on it fit.
(function () {
  "use strict";

  // A slide is laid out at a fixed size and scaled as a whole, so it looks
  // the same in every window and on paper. What does not fit at that size is
  // made smaller until it does, rather than cut off.
  function fit(slide) {
    var content = slide.querySelector(".slide-content");
    if (!content) return;
    content.style.zoom = "";
    var zoom = 1;
    // zoom reflows the text, so the height does not shrink in proportion: a few rounds
    for (var i = 0; i < 6 && content.scrollHeight > content.clientHeight + 1 && zoom > 0.3; i++) {
      zoom = Math.max(zoom * content.clientHeight / content.scrollHeight * 0.98, 0.3);
      content.style.zoom = zoom;
    }
  }
  function fitAll() { document.querySelectorAll(".slide").forEach(fit); }

  // Diagrams are drawn before anything is scaled. Mermaid sizes the boxes of a
  // diagram by measuring its labels on the page; measured on a slide that is
  // scaled down, the boxes come out too small for the labels as soon as the
  // scale changes - on paper, or when the window is resized - and the text is
  // cut off. "measuring" switches every scale off (see the stylesheet) and
  // hides the slides for that moment.
  var root = document.documentElement;
  root.classList.add("measuring");
  function measured() {
    root.classList.remove("measuring");
    fitAll(); // images and diagrams have their size now
  }
  addEventListener("load", function () {
    if (window.mermaid && document.querySelector(".mermaid")) mermaid.run().then(measured, measured);
    else measured();
  });
  addEventListener("beforeprint", fitAll);

  var deck = document.getElementById("deck");
  if (!deck) {
    // The export page: fitting is all it needs, and a preview that fits the window.
    var sheets = document.getElementById("sheets");
    var preview = function () {
      var w = parseFloat(getComputedStyle(sheets).getPropertyValue("--slide-w"));
      sheets.style.setProperty("--preview-zoom", Math.min(0.75, (innerWidth - 48) / w));
    };
    if (sheets) { addEventListener("resize", preview); preview(); }
    return;
  }

  var slides = Array.prototype.slice.call(deck.querySelectorAll(".slide"));
  var counter = document.getElementById("deck-counter");
  var progress = document.getElementById("deck-progress");
  var current = -1;
  document.documentElement.classList.add("js");

  function scale() {
    var w = parseFloat(getComputedStyle(deck).getPropertyValue("--slide-w")), h = parseFloat(getComputedStyle(deck).getPropertyValue("--slide-h"));
    var bar = document.querySelector(".deck-bar").offsetHeight;
    deck.style.setProperty("--scale", Math.min(innerWidth / w, (innerHeight - bar) / h));
  }
  addEventListener("resize", scale);
  scale();

  function show(n, fromHash) {
    n = Math.min(Math.max(n, 0), slides.length - 1);
    if (n === current) return;
    current = n;
    slides.forEach(function (s, i) {
      var on = i === n;
      s.classList.toggle("current", on);
      s.inert = !on;                       // hidden slides take no focus and are not read
      s.setAttribute("aria-hidden", String(!on));
    });
    counter.textContent = "Slide " + (n + 1) + " of " + slides.length;
    progress.style.width = (slides.length > 1 ? n / (slides.length - 1) * 100 : 100) + "%";
    document.getElementById("deck-prev").disabled = n === 0;
    document.getElementById("deck-next").disabled = n === slides.length - 1;
    // The slide number lives in the address: reload, Back and a copied link keep the place.
    if (!fromHash) history.replaceState(null, "", "#" + (n + 1));
  }
  function fromHash() { show((parseInt(location.hash.slice(1), 10) || 1) - 1, true); }
  addEventListener("hashchange", fromHash);
  fromHash();

  function next() { show(current + 1); }
  function prev() { show(current - 1); }
  function exit() { location.href = document.getElementById("deck-exit").href; }
  function fullScreen() {
    if (document.fullscreenElement) document.exitFullscreen();
    else if (document.documentElement.requestFullscreen) document.documentElement.requestFullscreen();
  }

  // ---- keys ---------------------------------------------------------------
  // letter: a single-character shortcut, which the reader may have switched off
  // (see keys.js); the named keys always work.
  var BINDINGS = [
    { keys: ["ArrowRight", "ArrowDown", "PageDown", " ", "Enter"], show: "→ ↓ PgDn Space Enter", does: "Next slide", run: next },
    { keys: ["ArrowLeft", "ArrowUp", "PageUp", "Backspace"], show: "← ↑ PgUp Backspace", does: "Previous slide (also Shift+Space)", run: prev },
    { keys: ["l", "j"], show: "l j", letter: true, does: "Next slide", run: next },
    { keys: ["h", "k"], show: "h k", letter: true, does: "Previous slide", run: prev },
    { keys: ["Home"], show: "Home", does: "First slide", run: function () { show(0); } },
    { keys: ["End"], show: "End", does: "Last slide", run: function () { show(slides.length - 1); } },
    { keys: ["gg"], show: "gg", letter: true, does: "First slide", run: function () { show(0); } },
    { keys: ["G"], show: "G", letter: true, does: "Last slide", run: function () { show(slides.length - 1); } },
    { keys: ["f"], show: "f", letter: true, does: "Full screen, and back", run: fullScreen },
    { keys: ["q"], show: "q", letter: true, does: "Leave the slide show, back to the note", run: exit },
    { keys: ["Escape"], show: "Esc", does: "Leave full screen; otherwise leave the slide show", run: function () { if (!document.fullscreenElement) exit(); } },
    { keys: ["?"], show: "?", letter: true, does: "Show these keys", run: function () { help(); } },
  ];
  var lettersOff = false;
  try { lettersOff = localStorage.getItem("keys-off") === "1"; } catch (e) { /* on, then */ }

  var dialog = document.getElementById("deck-help");
  function help() {
    var body = dialog.querySelector(".keys-table");
    if (!body.childElementCount) {
      var table = document.createElement("table");
      BINDINGS.forEach(function (b) {
        var row = table.insertRow(), keys = row.insertCell();
        b.show.split(" ").forEach(function (k, i) {
          if (i) keys.append(" ");
          keys.append(Object.assign(document.createElement("kbd"), { textContent: k }));
        });
        row.insertCell().textContent = b.does + (b.letter && lettersOff ? " (switched off)" : "");
      });
      body.append(table);
    }
    if (!dialog.open) dialog.showModal();
  }

  var pendingG = 0;
  addEventListener("keydown", function (e) {
    if (dialog.open || e.ctrlKey || e.altKey || e.metaKey) return;
    if (/^(INPUT|TEXTAREA|SELECT)$/.test(e.target.tagName)) return;
    // Space and Enter on a focused button or link are that control's own
    if ((e.key === " " || e.key === "Enter") && e.target.closest("button, a, summary")) return;
    var key = e.key;
    if (key === " " && e.shiftKey) { e.preventDefault(); return prev(); }
    if (key === "g") { // gg
      if (lettersOff) return;
      if (Date.now() - pendingG < 1200) { pendingG = 0; key = "gg"; } else { pendingG = Date.now(); return; }
    }
    for (var i = 0; i < BINDINGS.length; i++) {
      var b = BINDINGS[i];
      if (b.keys.indexOf(key) < 0 || (b.letter && lettersOff)) continue;
      e.preventDefault();
      return b.run();
    }
  });

  // ---- mouse and touch ------------------------------------------------------
  document.getElementById("deck-next").addEventListener("click", next);
  document.getElementById("deck-prev").addEventListener("click", prev);
  var full = document.getElementById("deck-full");
  if (document.documentElement.requestFullscreen) { full.hidden = false; full.addEventListener("click", fullScreen); }
  var helpButton = document.getElementById("deck-help-button");
  helpButton.hidden = false;
  helpButton.addEventListener("click", help);

  // A click on the slide: right half on, left half back. Not on something that
  // is itself clickable, and not at the end of selecting text.
  deck.addEventListener("click", function (e) {
    if (e.target.closest("a, button, summary, input, select, textarea, video, audio")) return;
    if (String(getSelection())) return;
    e.clientX > innerWidth / 2 ? next() : prev();
  });
  var touchX = null, touchY = null;
  deck.addEventListener("touchstart", function (e) { touchX = e.touches[0].clientX; touchY = e.touches[0].clientY; }, { passive: true });
  deck.addEventListener("touchend", function (e) {
    if (touchX === null) return;
    var dx = e.changedTouches[0].clientX - touchX, dy = e.changedTouches[0].clientY - touchY;
    touchX = null;
    if (Math.abs(dx) > 50 && Math.abs(dx) > 1.5 * Math.abs(dy)) dx < 0 ? next() : prev();
  }, { passive: true });
})();
