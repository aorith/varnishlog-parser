// Live client-side filtering for the "Duplicated" headers table on the
// Headers view. The table is fully rendered server-side (every header
// duplicated at least twice); rows are just hidden/shown here based on the
// minimum count typed by the user, no server round-trip needed.
function filterDuplicateHeaders(minDup) {
  const threshold = parseInt(minDup, 10) || 2;
  const rows = document.querySelectorAll("#duplicateHeadersTable tbody tr");
  let visible = 0;

  rows.forEach((row) => {
    const show = parseInt(row.dataset.count, 10) >= threshold;
    row.hidden = !show;
    if (show) {
      visible++;
    }
  });

  const empty = document.getElementById("dupHeadersEmpty");
  if (empty) {
    empty.hidden = visible > 0;
  }
}
