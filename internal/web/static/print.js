// Gets a page ready for paper. Everything that can be is done in the print
// stylesheet; this is what CSS cannot do.
(function () {
  "use strict";
  var root = document.documentElement;

  // Sections the reader folded away belong in the printout all the same. CSS
  // cannot open a closed <details>, so they are opened for the print and
  // closed again after it.
  var reopened = [];
  addEventListener("beforeprint", function () {
    document.querySelectorAll("details:not([open])").forEach(function (d) {
      if (d.closest("header, .export-bar")) return;
      d.open = true;
      reopened.push(d);
    });
  });
  addEventListener("afterprint", function () {
    reopened.forEach(function (d) { d.open = false; });
    reopened = [];
  });

  // The export's settings take effect when they are changed; there is no Apply
  // button to find. Applying means loading the page anew, so the place in the
  // preview is kept across it.
  var bar = document.querySelector("form.export-bar");
  var button = document.getElementById("print");
  if (bar) {
    var query = new URLSearchParams(location.search);
    var key = "export-scroll:" + location.pathname + "?" + (query.get("path") || "");
    try {
      var y = sessionStorage.getItem(key);
      if (y !== null) { sessionStorage.removeItem(key); addEventListener("load", function () { scrollTo(0, parseInt(y, 10)); }); }
    } catch (e) { /* no storage: the preview starts at the top */ }

    var settings = function () { return new URLSearchParams(new FormData(bar)).toString(); };
    var shown = settings(); // what this page was made with
    var apply = function (thenPrint) {
      if (thenPrint) {
        var flag = document.createElement("input");
        flag.type = "hidden"; flag.name = "print"; flag.value = "1";
        bar.append(flag);
      }
      try { sessionStorage.setItem(key, String(scrollY)); } catch (e) { /* see above */ }
      bar.submit(); // a second submit replaces one that is still under way
    };
    bar.addEventListener("change", function () { apply(false); });

    // A setting that is changed and Print clicked before the page has come
    // back would print the page that is about to be replaced, without the
    // setting. So a print with unapplied settings applies them first and prints
    // what comes back. (This was a real bug while there was a number field:
    // one click left the field, which applied it, and printed the old page.)
    button.addEventListener("click", function () {
      if (settings() !== shown) apply(true); else print();
    });
    if (query.has("print")) {
      query.delete("print"); // so that reloading the page does not print again
      history.replaceState(null, "", location.pathname + "?" + query.toString());
      // after load, and a moment for diagrams, which draw themselves then
      addEventListener("load", function () { setTimeout(function () { print(); }, 400); });
    }
  } else if (button) {
    button.addEventListener("click", function () { print(); });
  }
})();
