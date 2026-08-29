// Generic sub-menu for views: a row of links below a view's <h1> (see
// .view-submenu) that switch between sibling .subview panels within the
// same .view, e.g. Headers' "State" / "Duplicated". Event-delegated so any
// view can opt in just by adding the markup, no per-view JS required.
document.addEventListener("click", (e) => {
  const link = e.target.closest(".subnav-link");
  if (!link) {
    return;
  }

  e.preventDefault();

  const menu = link.closest(".view-submenu");
  const view = link.closest(".view");
  if (!menu || !view) {
    return;
  }

  menu.querySelectorAll(".subnav-link").forEach((l) => {
    l.classList.toggle("active", l === link);
  });

  const targetId = link.dataset.target;
  view.querySelectorAll(".subview").forEach((panel) => {
    panel.hidden = panel.id !== targetId;
  });
});
