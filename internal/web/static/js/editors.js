// editors.js: 설정 화면의 편집 칸 (목록, [이름 → 값] 표, 입력 칸 하나, 창고 매핑)
import { el, iconButton, addButton } from "./dom.js";

// listEditor: 이름 목록 편집 (순서 바꾸기, 삭제, 추가)
export function listEditor(arr, placeholder) {
  const box = el("div", undefined, "list");

  function render(focusIndex) {
    box.replaceChildren();

    arr.forEach((value, i) => {
      const row = el("div", undefined, "list-row");
      const input = el("input");

      input.value = value;
      input.placeholder = placeholder || "";
      input.addEventListener("input", () => {
        arr[i] = input.value;
      });

      row.append(
        input,
        iconButton("↑", "위로", () => {
          if (i > 0) {
            arr.splice(i - 1, 0, arr.splice(i, 1)[0]);
            render();
          }
        }),
        iconButton("↓", "아래로", () => {
          if (i < arr.length - 1) {
            arr.splice(i + 1, 0, arr.splice(i, 1)[0]);
            render();
          }
        }),
        iconButton("✕", "삭제", () => {
          arr.splice(i, 1);
          render();
        }),
      );
      box.append(row);
    });

    box.append(
      addButton("+ 추가", () => {
        arr.push("");
        render(arr.length - 1);
      }),
    );

    if (focusIndex !== undefined)
      box.querySelectorAll("input")[focusIndex].focus();
  }
  render();
  return box;
}

// pairEditor: [이름 → 값] 표 편집. unit을 주면 값 뒤에 단위(예: %)를 붙여 보여 줌
export function pairEditor(list, keyHint, valueHint, unit) {
  const box = el("div", undefined, "list");

  function render(focusIndex) {
    box.replaceChildren();

    list.forEach((p, i) => {
      const row = el("div", undefined, "list-row");
      const k = el("input");
      const v = el("input");

      k.value = p[0];
      k.placeholder = keyHint;
      v.value = p[1];
      v.placeholder = valueHint;

      k.addEventListener("input", () => {
        p[0] = k.value;
      });
      v.addEventListener("input", () => {
        p[1] = v.value;
      });

      row.append(k, el("span", "→", "arrow"), v);

      if (unit) row.append(el("span", unit, "unit"));

      row.append(
        iconButton("✕", "삭제", () => {
          list.splice(i, 1);
          render();
        }),
      );
      box.append(row);
    });

    box.append(
      addButton("+ 추가", () => {
        list.push(["", ""]);
        render(list.length - 1);
      }),
    );

    if (focusIndex !== undefined)
      box.querySelectorAll("input")[focusIndex * 2].focus();
  }
  render();
  return box;
}

// textEditor: 입력 칸 하나 (obj[key]를 바꿈)
export function textEditor(obj, key, hint) {
  const row = el("div", undefined, "list-row");
  const input = el("input");

  input.value = obj[key];
  input.placeholder = hint || "";
  input.addEventListener("input", () => {
    obj[key] = input.value;
  });

  row.append(input);
  return row;
}

// warehouseEditor: 시트별 원본 창고 목록
export function warehouseEditor(groups) {
  const box = el("div");

  function render() {
    box.replaceChildren();

    groups.forEach((g, i) => {
      const block = el("div", undefined, "sheet-block");
      const head = el("div", undefined, "list-row");
      const name = el("input");

      name.value = g.sheet;
      name.placeholder = "시트 이름 (예: 운영지원부)";

      name.addEventListener("input", () => {
        g.sheet = name.value;
      });

      head.append(
        name,
        iconButton("✕", "시트 삭제", () => {
          if (confirm("'" + g.sheet + "' 시트 매핑을 지울까요?")) {
            groups.splice(i, 1);
            render();
          }
        }),
      );

      block.append(head, listEditor(g.sources, "원본 창고 이름"));
      box.append(block);
    });

    box.append(
      addButton("+ 시트 추가", () => {
        groups.push({ sheet: "", sources: [] });
        render();
      }),
    );
  }
  render();
  return box;
}

// field: 제목 + 설명 + 편집 칸
export function field(title, help, editor) {
  const f = el("div", undefined, "field");

  f.append(el("div", title, "title"));

  if (help) f.append(el("div", help, "help"));

  f.append(editor);

  return f;
}

// group: 기능별 묶음 (발주서, 결산서, ...)
export function group(title, fields) {
  const g = el("div", undefined, "settings-group");

  g.append(el("h3", title));
  fields.forEach((f) => g.append(f));

  return g;
}
