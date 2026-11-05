// jobs.js: 기능 탭 4개 (파일 놓는 곳, 실행, 결과 표시, 보관된 양식 표시)
import { el } from "./dom.js";
import { showFixes } from "./fixes.js";

// initDropZones: 파일을 고르면 이름을 보여 주고, 끌어다 놓는 동안 색을 바꿈
function initDropZones() {
  document.querySelectorAll(".drop").forEach((drop) => {
    const input = drop.querySelector("input[type=file]");
    const name = drop.querySelector(".name");
    const hint = name.textContent;

    input.addEventListener("change", () => {
      const chosen = input.files.length > 0;
      const names = Array.from(input.files).map((f) => f.name);
      name.textContent = chosen ? names.join(", ") : hint;
      drop.classList.toggle("chosen", chosen);
    });
    input.addEventListener("dragenter", () => drop.classList.add("dragover"));
    input.addEventListener("dragleave", () =>
      drop.classList.remove("dragover"),
    );
    input.addEventListener("drop", () => drop.classList.remove("dragover"));
  });
}

// loadTemplates: /api/templates 를 불러서 결산서·소모품 탭에 보관된 양식을 표시
function loadTemplates() {
  fetch("/api/templates")
    .then((res) => res.json())
    .then((t) => {
      document.querySelectorAll(".template-info").forEach((p) => {
        const info = t[p.dataset.template];

        if (!info || !info.files || info.files.length === 0) {
          p.className = "template-info error";
          p.textContent =
            "보관된 양식이 없습니다. 처음 한 번은 양식을 같이 올려 주세요.";
          return;
        }
        p.className = "template-info muted";
        p.textContent =
          "보관된 양식 (" + info.modified + "): " + info.files.join(", ");
      });
    });
}

// showResult: 서버가 돌려준 실행 결과를 화면에 그림
function showResult(box, data) {
  box.replaceChildren();

  if (!data.ok) {
    const fail = el("div", "", "box fail");

    fail.append(el("strong", "실행 실패"), el("div", data.error));
    box.append(fail);
    return;
  }

  if (data.warnings && data.warnings.length > 0) {
    const warn = el("div", "", "box warn");
    warn.append(el("strong", "확인이 필요합니다"));

    const ul = el("ul");
    data.warnings.forEach((w) => ul.append(el("li", w)));
    warn.append(ul);
    box.append(warn);
  }
  box.append(el("pre", (data.lines || []).join("\n")));

  const folder = el("div", "", "folder");
  const open = el("button", "결과 폴더 열기", "secondary");

  open.type = "button";
  open.addEventListener("click", () => {
    fetch("/api/open", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ path: data.outDir }),
    })
      .then((res) => res.json())
      .then((r) => {
        if (r.error) alert(r.error);
      });
  });

  folder.append(open, el("span", data.outDir));
  box.append(folder);

  if (data.fixes && data.fixes.length > 0) {
    showFixes(box, data.fixes);
  }
}

// initJobForms: form.job 을 제출하면 /api/run/<data-job> 으로 파일을 보내고 결과를 표시
function initJobForms() {
  document.querySelectorAll("form.job").forEach((form) => {
    const box = form.parentElement.querySelector(".result");
    const submit = form.querySelector("button[type=submit]");

    form.addEventListener("submit", (e) => {
      e.preventDefault(); // 페이지가 새로 고쳐지지 않게

      submit.disabled = true;
      submit.textContent = "변환 중...";
      box.replaceChildren(
        el("p", "변환하고 있습니다. 잠시 기다려 주세요.", "muted"),
      );

      fetch("/api/run/" + form.dataset.job, {
        method: "POST",
        body: new FormData(form),
      })
        .then((res) => res.json())
        .then((data) => showResult(box, data))
        .catch((err) =>
          showResult(box, {
            ok: false,
            error: "서버와 통신하지 못했습니다: " + err,
          }),
        )
        .finally(() => {
          submit.disabled = false;
          submit.textContent = "변환 실행";
          loadTemplates(); // 양식이 새로 보관됐을 수 있으니 다시 표시
        });
    });
  });
}

// initJobs: 기능 탭 준비
export function initJobs() {
  initDropZones();
  initJobForms();
  loadTemplates();
}
