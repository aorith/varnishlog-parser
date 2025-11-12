let db;

// Initialize IndexedDB
function initDB() {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open("HistoryDB", 1);

    request.onerror = () => reject("Database failed to open");

    request.onsuccess = () => {
      db = request.result;
      resolve(db);
    };

    request.onupgradeneeded = (e) => {
      db = e.target.result;

      if (!db.objectStoreNames.contains("entries")) {
        const objectStore = db.createObjectStore("entries", {
          keyPath: "hash",
        });
        objectStore.createIndex("created", "created", { unique: false });
      }
    };
  });
}

// https://developer.mozilla.org/en-US/docs/Web/API/SubtleCrypto/digest
async function digestMessage(message) {
  const msgUint8 = new TextEncoder().encode(message); // encode as (utf-8) Uint8Array
  const hashBuffer = await window.crypto.subtle.digest("SHA-256", msgUint8); // hash the message
  if (Uint8Array.prototype.toHex) {
    // Use toHex if supported.
    return new Uint8Array(hashBuffer).toHex(); // Convert ArrayBuffer to hex string.
  }
  // If toHex() is not supported, fall back to an alternative implementation.
  const hashArray = Array.from(new Uint8Array(hashBuffer)); // convert buffer to byte array
  const hashHex = hashArray
    .map((b) => b.toString(16).padStart(2, "0"))
    .join(""); // convert bytes to hex string
  return hashHex;
}

// Add new entry
async function addEntry(content, nParsed) {
  const timestamp = new Date().toISOString().slice(0, 19);
  const hash = await digestMessage(content);

  const host = document.getElementById("firstParsedHostname").value;
  let name = `${nParsed}txs`;
  if (host) {
    name += `@${host}`;
  }

  const entry = {
    hash: hash,
    name: name,
    content: content,
    created: timestamp,
  };

  return new Promise((resolve, reject) => {
    const transaction = db.transaction(["entries"], "readwrite");
    const objectStore = transaction.objectStore("entries");

    const getRequest = objectStore.get(hash);
    getRequest.onsuccess = (event) => {
      const existing = event.target.result;

      if (existing) {
        resolve(false);
        return;
      }

      const putRequest = objectStore.put(entry);
      putRequest.onsuccess = () => {
        console.log("New entry added:", hash);
        resolve(true);
      };

      putRequest.onerror = () => {
        console.error("Error saving history entry");
        reject(putRequest.error);
      };
    };

    getRequest.onerror = () => {
      console.error("Error checking existing entry");
      reject(getRequest.error);
    };
  });
}

// Get all entries
async function getAllEntries() {
  return new Promise((resolve, reject) => {
    const transaction = db.transaction(["entries"], "readonly");
    const objectStore = transaction.objectStore("entries");
    const request = objectStore.getAll();

    request.onsuccess = () => {
      const entries = request.result;
      // Sort by created date, newest first
      entries.sort((a, b) => new Date(b.created) - new Date(a.created));
      resolve(entries);
    };

    request.onerror = () => reject("Error fetching entries");
  });
}

// Get entry by hash
async function getEntry(hash) {
  return new Promise((resolve, reject) => {
    const transaction = db.transaction(["entries"], "readonly");
    const objectStore = transaction.objectStore("entries");
    const request = objectStore.get(hash);

    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject("Error fetching entry");
  });
}

// Update entry
async function updateEntry(entry) {
  return new Promise((resolve, reject) => {
    const transaction = db.transaction(["entries"], "readwrite");
    const objectStore = transaction.objectStore("entries");
    const request = objectStore.put(entry);

    request.onsuccess = () => resolve();
    request.onerror = () => reject("Error updating entry");
  });
}

// Delete entry
async function deleteEntry(hash) {
  if (!confirm("Are you sure you want to delete this entry?")) {
    return;
  }

  return new Promise((resolve, reject) => {
    const transaction = db.transaction(["entries"], "readwrite");
    const objectStore = transaction.objectStore("entries");
    const request = objectStore.delete(hash);

    request.onsuccess = () => {
      renderHistory();
      resolve();
    };

    request.onerror = () => {
      console.log("Error deleting entry");
      reject();
    };
  });
}

// Load entry
async function loadEntry(hash) {
  const input = document.getElementById("logsInput");
  const button = document.getElementById("parse-submit-btn");
  try {
    const entry = await getEntry(hash);
    if (entry) {
      input.value = entry.content;
      setTimeout(() => button.click(), 100);
    }
  } catch (error) {
    console.log("Error loading entry " + error);
  }
}

// Rename entry
async function renameEntry(hash, newName) {
  try {
    const entry = await getEntry(hash);
    if (entry) {
      entry.name = newName;
      await updateEntry(entry);
      renderHistory();
    }
  } catch (error) {
    console.log("Error renaming entry");
  }
}

// Render history table
async function renderHistory() {
  try {
    const entries = await getAllEntries();
    const container = document.getElementById("historyContainer");

    if (entries.length === 0) {
      container.innerHTML =
        '<div class="empty-state">No entries yet. Parse some logs first.</div>';
      return;
    }

    let html =
      "<table><thead><tr><th>Name</th><th>Hash</th><th>Created At</th><th>Actions</th></tr></thead><tbody>";

    entries.forEach((entry) => {
      html += `
                        <tr>
                            <td class="name-cell" onclick="editName('${entry.hash}')" id="name-${entry.hash}">${entry.name}</td>
                            <td class="hash-cell"><abbr title="sha-256 digest generated from the logs input: ${entry.hash}">${entry.hash.slice(0, 10)}...</abbr></td>
                            <td>${entry.created}</td>
                            <td>
                                <a class="btn-small btn-load" onclick="loadEntry('${entry.hash}')">Load</a>
                                <a class="btn-small btn-delete" onclick="deleteEntry('${entry.hash}')">Delete</a>
                            </td>
                        </tr>
                    `;
    });

    html += "</tbody></table>";
    container.innerHTML = html;
  } catch (error) {
    container.innerHTML = "Error rendering history";
    console.log("Error rendering history");
  }
}

// Edit name inline
async function editName(hash) {
  try {
    const entry = await getEntry(hash);
    if (!entry) return;

    const cell = document.getElementById(`name-${hash}`);
    const currentName = entry.name;

    cell.innerHTML = `<input type="text" class="name-input" value="${currentName}" id="input-${hash}" onblur="saveName('${hash}')" onkeypress="if(event.key==='Enter') saveName('${hash}')">`;

    const input = document.getElementById(`input-${hash}`);
    input.focus();
    input.select();
  } catch (error) {
    console.log("Error editing name");
  }
}

// Save name
async function saveName(hash) {
  const input = document.getElementById(`input-${hash}`);
  if (input) {
    const newName = input.value.trim();
    if (newName) {
      await renameEntry(hash, newName);
    } else {
      renderHistory();
    }
  }
}

// Save entry
async function saveCurrentLogs(nParsed) {
  const textInput = document.getElementById("logsInput");
  let content = textInput.value;

  // Remove empty lines
  content = content
    .split("\n")
    .filter((line) => line.trim() !== "")
    .join("\n");

  if (content.trim() === "") {
    return;
  }

  await addEntry(content, nParsed);
  renderHistory();
}

// Initialize history log
document.onreadystatechange = () => {
  if (document.readyState === "complete") {
    initDB()
      .then(() => {
        renderHistory();

        const nParsed = Number(
          document.getElementById("currentNumOfTxsParsed").value,
        );
        if (!isNaN(nParsed) && nParsed > 0) {
          saveCurrentLogs(nParsed);
        }
      })
      .catch((error) => {
        console.log("Failed to initialize database: " + error);
      });
  }
};
