// settings.js: 설정 탭 (불러오기, 화면 그리기, 저장)
import { el } from "./dom.js";
import {
  listEditor,
  pairEditor,
  textEditor,
  warehouseEditor,
  field,
  group,
} from "./editors.js";

// ui: 화면에서 편집하는 설정. config.json을 편집하기 쉬운 모양(목록, [이름, 값] 짝)으로 바꾼 것.
// 다른 파일에서 import하면 항상 지금 값을 봄 (loadSettings가 바꾸면 같이 바뀜)
export let ui = null;

// pairs: {키: 값} → [[키, 값], ...] (키 가나다순)
function pairs(obj) {
  return Object.keys(obj || {})
    .sort((a, b) => a.localeCompare(b, "ko"))
    .map((k) => [k, obj[k]]);
}

// cleanList: 앞뒤 공백을 지우고 빈 칸은 뺌
function cleanList(list) {
  return list.map((v) => String(v).trim()).filter((v) => v !== "");
}

// fromConfig: 서버 설정(config.json 모양) → 화면 편집용
function fromConfig(c) {
  const o = c.welstoryOrder || {},
    s = c.settlement || {},
    e = c.expendables || {},
    v = c.vendors || {};

  return {
    sites: (o.sites || []).slice(),
    contracts: (o.contracts || []).slice(),
    vendorName: pairs(o.vendorName),
    employeeSite: s.employeeSite || "",
    removeSuffix: s.removeSuffix || "",
    placeMap: pairs(s.placeMap),
    // 낙찰률은 화면에서 % 로 보여 줌 (0.68375 → 68.375)
    rates: pairs(e.rates).map((p) => [p[0], String(+(p[1] * 100).toFixed(6))]),
    warehouses: pairs(e.warehouses).map((p) => ({
      sheet: p[0],
      sources: p[1].slice(),
    })),
    expExclude: (e.exclude || []).slice(),
    vendorOrder: (v.order || []).slice(),
    vendorExclude: (v.exclude || []).slice(),
  };
}

// toConfig: 화면 편집용 → 서버 설정(config.json 모양)
function toConfig(u) {
  function obj(list, convert) {
    const o = {};

    list.forEach((p) => {
      const k = String(p[0]).trim();
      const val = convert ? convert(p[1]) : String(p[1]).trim();

      if (k === "" && val === "") return; // 완전히 빈 줄은 무시

      o[k] = val;
    });
    return o;
  }

  const warehouses = {};

  u.warehouses.forEach((g) => {
    const name = g.sheet.trim();
    if (name === "" && cleanList(g.sources).length === 0) return;
    warehouses[name] = cleanList(g.sources);
  });

  return {
    welstoryOrder: {
      sites: cleanList(u.sites),
      contracts: cleanList(u.contracts),
      vendorName: obj(u.vendorName),
    },
    settlement: {
      employeeSite: u.employeeSite.trim(),
      removeSuffix: u.removeSuffix,
      placeMap: obj(u.placeMap),
    },
    expendables: {
      // % → 소수 (68.375 → 0.68375). 숫자가 아니면 0으로 보내서 서버가 범위 오류로 알려 줌
      rates: obj(u.rates, (v) => {
        const n = parseFloat(String(v).replace("%", ""));
        return isNaN(n) ? 0 : +(n / 100).toFixed(8);
      }),
      warehouses: warehouses,
      exclude: cleanList(u.expExclude),
    },
    vendors: {
      order: cleanList(u.vendorOrder),
      exclude: cleanList(u.vendorExclude),
    },
  };
}

// ensureSettings: 아직 설정을 안 불러왔으면 불러옴
export function ensureSettings() {
  if (ui) return Promise.resolve(ui);
  return loadSettings();
}

// loadSettings: 서버에서 설정을 읽어 화면을 다시 그림
export function loadSettings() {
  return fetch("/api/config")
    .then((res) => res.json())
    .then((c) => {
      if (c.error) throw new Error(c.error);

      ui = fromConfig(c);
      renderSettings();

      return ui;
    })
    .catch((err) => {
      document
        .getElementById("settings-body")
        .replaceChildren(
          el("p", "설정을 불러오지 못했습니다: " + err.message, "error"),
        );

      throw err;
    });
}

// saveSettings: 서버에 저장. 결과 {ok, errors}
export function saveSettings() {
  return fetch("/api/config", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(toConfig(ui)),
  }).then((res) => res.json());
}

function showSettingsMessage(result) {
  const msg = document.getElementById("settings-msg");

  if (result.ok) {
    msg.className = "ok";
    msg.textContent =
      "저장했습니다. 이전 설정은 '설정백업' 폴더에 보관했습니다.";
    return;
  }

  msg.className = "error";
  msg.replaceChildren(
    el("strong", "저장하지 못했습니다. 아래 내용을 고쳐 주세요."),
  );

  const ul = el("ul");
  (result.errors || []).forEach((e) => ul.append(el("li", e)));
  msg.append(ul);
}

function renderSettings() {
  document
    .getElementById("settings-body")
    .replaceChildren(
      group("발주서", [
        field(
          "사업장 순서",
          "결과 파일 순서입니다. 목록에 없는 사업장은 맨 뒤로 갑니다.",
          listEditor(ui.sites, "사업장 이름"),
        ),
        field(
          "계약구분 순서",
          "사업장 안에서 계약구분 순서 (면·과 순서는 항상 면 → 과)",
          listEditor(ui.contracts, "계약구분"),
        ),
        field(
          "거래처명",
          "업로드 파일 C열에 들어갈 거래처명 (계약구분별)",
          pairEditor(ui.vendorName, "계약구분", "거래처명"),
        ),
      ]),
      group("결산서", [
        field(
          "직원식당 구분 글자",
          "현장명에 이 글자가 있으면 직원계약·직원비계약 파일로 보냅니다.",
          textEditor(ui, "employeeSite", "직원식당"),
        ),
        field(
          "현장명에서 떼어 낼 글자",
          "예: '양식뷔페-비계약' → '양식뷔페'",
          textEditor(ui, "removeSuffix", "-비계약"),
        ),
        field(
          "납품장소 바꾸기",
          "현장명을 결산서 납품장소 이름으로 바꿉니다.",
          pairEditor(ui.placeMap, "현장명", "납품장소"),
        ),
      ]),
      group("소모품", [
        field(
          "낙찰률",
          "품번이 '연도 2자리 + 1'(예: 261…)로 시작하는 (비) 시트 품목에 적용합니다.",
          pairEditor(ui.rates, "연도 (예: 26)", "낙찰률", "%"),
        ),
        field(
          "창고 매핑",
          "결과 시트마다 묶을 ESP 원본 창고 이름입니다.",
          warehouseEditor(ui.warehouses),
        ),
        field(
          "결산에서 뺄 창고",
          "",
          listEditor(ui.expExclude, "원본 창고 이름"),
        ),
      ]),
      group("거래처별", [
        field(
          "거래처 시트 순서",
          "거래가 없어도 시트를 만듭니다. 목록에 없는 거래처는 가나다순으로 뒤에 붙습니다.",
          listEditor(ui.vendorOrder, "거래처명"),
        ),
        field(
          "시트를 만들지 않을 거래처",
          "",
          listEditor(ui.vendorExclude, "거래처명"),
        ),
      ]),
    );
}

// initSettings: [저장]·[되돌리기] 버튼 연결 + 처음 불러오기
export function initSettings() {
  document.getElementById("settings-save").addEventListener("click", () => {
    saveSettings().then((result) => {
      showSettingsMessage(result);
      if (result.ok) loadSettings(); // 저장된 모양(정렬, 빈 줄 정리)으로 다시 표시
    });
  });
  document.getElementById("settings-reload").addEventListener("click", () => {
    document.getElementById("settings-msg").replaceChildren();
    loadSettings();
  });
  loadSettings();
}
