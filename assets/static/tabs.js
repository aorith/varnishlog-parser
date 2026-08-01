// View switching is driven entirely by location.hash: nav links point to a
// view's own id (e.g. #overview-view), and deep links point to an element
// nested inside a view (e.g. #tx-262-req-rxreq). Both are handled the same
// way: find the target element, activate its containing .view, and scroll
// to the target.
(function() {
  // nav is position:sticky, so it stays on screen after scrolling to an
  // anchor; --nav-height feeds html's scroll-padding-top so the anchor
  // itself doesn't land underneath it.
  function updateNavHeight() {
    const nav = document.querySelector("nav");
    if (nav) {
      document.documentElement.style.setProperty(
        "--nav-height",
        `${nav.offsetHeight}px`,
      );
    }
  }

  function setActive(activeSelector, newActiveEl) {
    document.querySelectorAll(activeSelector).forEach((el) => {
      el.classList.remove("active");
    });
    if (newActiveEl) {
      newActiveEl.classList.add("active");
    }
  }

  function activateView(view) {
    setActive(".view.active", view);

    const link = document.querySelector(`nav a.nav-link[href="#${view.id}"]`);
    setActive("nav a.nav-link.active", link);
  }

  function goToHash() {
    const id = location.hash.slice(1);
    if (!id) {
      return;
    }

    const target = document.getElementById(id);
    if (!target) {
      return;
    }

    const view = target.closest(".view");
    if (!view) {
      return;
    }

    activateView(view);

    // Deep links can point inside a collapsed <details> (e.g. a transaction
    // group in the VCL Log Tree that isn't the first/open one).
    for (let el = target; el; el = el.parentElement) {
      if (el.tagName === "DETAILS") {
        el.open = true;
      }
    }

    target.scrollIntoView({ behavior: "smooth", block: "start" });
  }


  // Sync the URL and nav-link hash (#) after clicking the PARSE button.
  function syncHashToActiveView() {
    const active = document.querySelector(".view.active");
    if (active) {
      history.replaceState(null, "", `#${active.id}`);
      activateView(active);
    }
  }

  window.addEventListener("hashchange", goToHash);
  window.addEventListener("resize", updateNavHeight);
  document.addEventListener("DOMContentLoaded", () => {
    updateNavHeight();
    if (location.hash) {
      goToHash();
    } else {
      syncHashToActiveView();
    }
  });
})();
