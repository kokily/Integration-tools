// dom.js: 화면 요소를 만드는 공통 함수

// el(태그, 글자): 화면에 넣을 요소를 만듦. 글자는 textContent로 넣어서 HTML로 해석되지 않게 함
export function el(tag, text, className) {
  const e = document.createElement(tag);

  if (text !== undefined) e.textContent = text;
  if (className) e.className = className;

  return e;
}

// iconButton: 작은 버튼 (↑ ↓ ✕)
export function iconButton(text, title, onClick) {
  const b = el("button", text, "icon");

  b.type = "button";
  b.title = title;
  b.addEventListener("click", onClick);

  return b;
}

// addButton: 점선 테두리 "+ 추가" 버튼
export function addButton(text, onClick) {
  const b = el("button", text, "add");

  b.type = "button";
  b.addEventListener("click", onClick);

  return b;
}
