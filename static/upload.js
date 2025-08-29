function showQueue() {
  document.querySelector("#queue").hidden = false;
}

function addItem(name) {
  const tr = document.createElement("tr");
  tr.className = "item border-b";

  const nameTd = document.createElement("td");
  nameTd.textContent = name;
  nameTd.className = "py-3 px-4";
  tr.appendChild(nameTd);

  const statusTd = document.createElement("td");
  statusTd.className = "py-3 px-4";
  statusTd.innerHTML = `<span class="muted">waiting…</span>`;
  tr.appendChild(statusTd);

  const progressTd = document.createElement("td");
  progressTd.className = "py-3 px-4 bar";
  progressTd.innerHTML = `<div class="text-blue-400 bg-blue-400"></div>`;
  tr.appendChild(progressTd);

  document.querySelector("#queue").appendChild(tr);
  return tr;
}

function setProgress(el, p) {
  el.querySelector(".bar > div").style.width = p + "%";
  el.querySelector("span").textContent = Math.floor(p) + "%";
}

async function upload(files) {
  showQueue();
  for (const file of files) {
    const row = addItem(file.name);
    await new Promise((resolve, reject) => {
      const form = new FormData();
      form.append("files[]", file, file.name);
      const xhr = new XMLHttpRequest();
      xhr.open("POST", "/upload");
      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable) {
          setProgress(row, (e.loaded / e.total) * 100);
        }
      };
      xhr.onload = () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          setProgress(row, 100);
          resolve();
        } else {
          row.querySelector("span").textContent = "failed";
          reject(new Error(xhr.statusText));
        }
      };
      xhr.onerror = () => {
        row.querySelector("span").textContent = "error";
        reject(new Error("network error"));
      };
      xhr.send(form);
    }).catch(() => {});
  }
}

window.addEventListener("DOMContentLoaded", () => {
  const drop = document.querySelector("#drop");
  const pick = document.querySelector("#pick");
  
  if (pick) {
    pick.addEventListener("change", (e) => upload(e.target.files));
  }

  if (drop) {
    drop.addEventListener("click", () => pick && pick.click());
    ["dragenter", "dragover"].forEach((ev) =>
      drop.addEventListener(ev, (e) => {
        e.preventDefault();
        drop.classList.add("drag");
      }),
    );
    ["dragleave", "drop"].forEach((ev) =>
      drop.addEventListener(ev, (e) => {
        e.preventDefault();
        drop.classList.remove("drag");
      }),
    );
    drop.addEventListener("drop", (e) => {
      if (e.dataTransfer?.files?.length) {
        upload(e.dataTransfer.files);
      }
    });
  }
});
