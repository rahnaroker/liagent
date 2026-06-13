# README

## About

This is the official Wails Svelte-TS template.

## Live Development

To run in live development mode, run `wails dev` in the project directory. This will run a Vite development
server that will provide very fast hot reload of your frontend changes. If you want to develop in a browser
and have access to your Go methods, there is also a dev server that runs on http://localhost:34115. Connect
to this in your browser, and you can call your Go code from devtools.

## Building

To build a redistributable, production mode package, use `wails build`.

## Обложки (генерация)

Рядом с `litagent.exe` лежит папка **`wallpaper`** — это шаблоны фонов для обложки.
Список меняется свободно: просто положите/удалите картинки (`.jpg`, `.png`, `.gif`, `.webp`).

Во вкладке **«Обложка»** видно текущую обложку книги и сгенерированную:
«Сгенерировать» берёт случайный фон, «Другой шаблон» — другой случайный, «Применить
к экспорту» подставит сгенерированную обложку при сборке EPUB. По центру верхней половины
фона рисуется белая плашка с названием книги и автором (серифный шрифт Georgia).

По умолчанию обложка сохраняет **размер и пропорции исходной картинки** — выбирайте шаблоны
под экран вашего Kindle (например 600×800), чтобы не было чёрных полос по краям.

Тонкая настройка (необязательно) — файл `wallpaper/cover.json`. `width`/`height` можно не
указывать (тогда берётся размер шаблона) или задать, чтобы принудительно привести все обложки
к одному размеру:

```json
{ "jpegQuality": 88,
  "plateWidth": 0.80, "plateCenterY": 0.30,
  "fontTitle": "C:\\Windows\\Fonts\\georgiab.ttf",
  "fontAuthor": "C:\\Windows\\Fonts\\georgia.ttf" }
```

Папку шаблонов можно переопределить переменной окружения `LITAGENT_WALLPAPER`.
