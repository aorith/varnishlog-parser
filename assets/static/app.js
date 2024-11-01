function openTab(evt, name) {
  var i, tabcontent, tablinks;
  tabcontent = document.getElementsByClassName("tabcontent");
  for (i = 0; i < tabcontent.length; i++) {
    tabcontent[i].style.display = "none";
  }
  tablinks = document.getElementsByClassName("tablinks");
  for (i = 0; i < tablinks.length; i++) {
    tablinks[i].className = tablinks[i].className.replace(" active", "");
  }
  document.getElementById(name).style.display = "block";
  evt.currentTarget.className += " active";
}

function toggleSidebar() {
  const sidebar = document.getElementById("sidebar");
  sidebar.classList.toggle("hidden");
}

// Code block triple-click select all for <pre><code>Code</code></pre>
let clickCount = 0;
let clickTimer = null;

document.addEventListener("click", function (event) {
  clickCount++;

  // Reset click count after a delay
  if (clickTimer) clearTimeout(clickTimer);
  clickTimer = setTimeout(() => {
    clickCount = 0;
  }, 400);

  if (clickCount === 3) {
    clickCount = 0;

    let elem = event.target.closest("pre");
    if (elem) {
      let range = document.createRange();
      range.selectNodeContents(elem);
      let selection = window.getSelection();
      selection.removeAllRanges();
      selection.addRange(range);
    }
  }
});

window.addEventListener("DOMContentLoaded", () => {
  // Default active tab
  document.getElementById("defaultTabBtn").click();

  // If the overview tab exists a log has been parsed, select it
  let overview = document.getElementById("btnOverview");
  if (overview) {
    overview.click();
  }

  // If the error tab exists a log happened, select it
  let taberror = document.getElementById("btnError");
  if (taberror) {
    taberror.click();
  }
});
