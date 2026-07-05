const composer = document.querySelector("[data-composer]");
if (composer) {
  const body = composer.querySelector("[data-body]");
  const counter = composer.querySelector("[data-counter]");
  const update = () => {
    const max = Number(body.getAttribute("maxlength") || 200);
    counter.textContent = String(max - [...body.value].length);
  };

  body.addEventListener("input", update);
  update();
}

for (const form of document.querySelectorAll("[data-delete-form]")) {
  form.addEventListener("submit", (event) => {
    if (!window.confirm("Delete this post?")) {
      event.preventDefault();
    }
  });
}
