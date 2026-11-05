// fixes.js: 실행 결과 아래 [설정에 추가] (설정에 없어서 경고가 난 항목을 바로 추가)
import { el } from "./dom.js";
import { ui, ensureSettings, loadSettings, saveSettings } from "./settings.js";

// showFixes: 결과 아래에 설정에 없는 항목을 바로 추가하는 줄을 그림
export function showFixes(box, fixes) {
  const wrap = el("div", "", "box warn");

  wrap.append(el("strong", "설정에 추가"));
  box.append(wrap);

  ensureSettings().then(() => {
    fixes.forEach((fix) => {
      const row = el("div", undefined, "fix-row");
      let extra = null; // 추가로 고르거나 입력할
      // 칸
      if (fix.kind === "site") {
        row.append(
          el("span", "사업장 '" + fix.value + "' → 발주서 사업장 순서 맨 뒤"),
        );
      } else if (fix.kind === "contract") {
        row.append(el("span", "계약구분 '" + fix.value + "' · 거래처명"));
        extra = el("input");
        extra.placeholder = "거래처명";
        row.append(extra);
      } else if (fix.kind === "warehouse") {
        row.append(el("span", "창고 '" + fix.value + "' → 시트"));
        extra = el("select");
        ui.warehouses.forEach((g) => {
          const opt = el("option", g.sheet);
          opt.value = g.sheet;
          extra.append(opt);
        });
        row.append(extra);
      } else if (fix.kind === "rate") {
        row.append(el("span", fix.value + "년 낙찰률"));
        extra = el("input");
        extra.placeholder = "예: 68.375";
        row.append(extra, el("span", "%"));
      } else {
        return;
      }

      const btn = el("button", "추가", "secondary");

      btn.type = "button";
      btn.addEventListener("click", () =>
        applyFix(fix, extra ? extra.value : "", row),
      );
      row.append(btn);
      wrap.append(row);
    });
  });
}

// applyFix: 설정에 항목을 넣고 바로 저장
function applyFix(fix, extra, row) {
  ensureSettings()
    .then(() => {
      if (fix.kind === "site") {
        ui.sites.push(fix.value);
      } else if (fix.kind === "contract") {
        ui.contracts.push(fix.value);
        ui.vendorName.push([fix.value, extra]);
      } else if (fix.kind === "warehouse") {
        ui.warehouses.forEach((g) => {
          if (g.sheet === extra) g.sources.push(fix.value);
        });
      } else if (fix.kind === "rate") {
        ui.rates.push([fix.value, extra]);
      }
      return saveSettings();
    })
    .then((result) => {
      if (result.ok) {
        row.replaceChildren(
          el("span", "✔ 추가했습니다. 다시 실행하면 반영됩니다.", "fix-done"),
        );
      } else {
        alert("추가하지 못했습니다.\n" + (result.errors || []).join("\n"));
      }
      loadSettings(); // 저장 결과(실패했으면 원래 설정)로 설정 화면을 다시 맞춤
    });
}
