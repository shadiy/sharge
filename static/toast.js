document.body.addEventListener("showMessage", function(evt){
  addToast(evt.detail.value);
})

function addToast(text) {
  const div = document.createElement("div");
  div.className = "bg-sky-900 rounded-lg border p-2 h-24";
  div.role = "alert";

  const title = document.createElement("h3");
  title.className = "mb-2";
  title.textContent = "Message";
  div.appendChild(title);

  const message = document.createElement("small");
  message.textContent = text;
  div.appendChild(message);

  document.getElementById("toast-container").appendChild(div);
  setTimeout(() => document.getElementById("toast-container").removeChild(div), 5000);
}