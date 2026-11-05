// main.js: 화면 시작점 (탭 전환, 서버 상태 표시, 각 화면 준비)
import { initJobs } from "./jobs.js";
import { initSettings } from "./settings.js";

// 탭 전환: 누른 버튼과 같은 id의 section만 보이게
function initTabs() {
  document.querySelectorAll("nav button").forEach((btn) => {
    btn.addEventListener("click", () => {
      document
        .querySelectorAll("nav button")
        .forEach((b) => b.classList.remove("active"));
      document
        .querySelectorAll("section")
        .forEach((s) => s.classList.remove("active"));
      btn.classList.add("active");
      document.getElementById(btn.dataset.tab).classList.add("active");
    });
  });
}

// showStatus: /api/status 를 불러서 화면 아래에 연결 상태 표시
function showStatus() {
  const box = document.getElementById("status");

  fetch("/api/status")
    .then((res) => res.json())
    .then((s) => {
      if (s.configErr) {
        box.className = "error";
        box.textContent = "설정 파일 오류: " + s.configErr;
      } else {
        box.className = "ok";
        box.textContent = "서버 연결됨 · 작업 폴더: " + s.appDir;
      }
    })
    .catch(() => {
      box.className = "error";
      box.textContent =
        "서버에 연결할 수 없습니다. 프로그램이 실행 중인지 확인하세요.";
    });
}

initTabs();
showStatus();
initJobs();
initSettings();
