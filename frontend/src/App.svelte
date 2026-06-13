<script lang="ts">
  import { Open, Reload, Export, Preview, CoverInfo, GenerateCover, ApplyCover, ClearCover, GetMetadata, SetMetadata, CleanTitleTags } from "../wailsjs/go/main/App";
  import { main } from "../wailsjs/go/models";

  let result: main.LoadResult | null = null;
  let enabled = new Set<string>();
  let loading = false;
  let errorMsg = "";
  let statusMsg = "";
  let filterRule: string | null = null;
  let view: "findings" | "preview" | "cover" | "meta" = "findings";
  let previewHtml = "";

  let coverInfo: main.CoverInfoView | null = null;
  let genCoverUri = "";

  let meta: main.MetaEdit | null = null;

  async function showMeta() {
    view = "meta";
    if (!meta) {
      try {
        meta = await GetMetadata();
      } catch (e) {
        errorMsg = String(e);
      }
    }
  }

  async function saveMeta() {
    if (!meta) return;
    try {
      await SetMetadata(meta);
    } catch (e) {
      errorMsg = String(e);
    }
  }

  async function cleanTitle() {
    if (!meta) return;
    try {
      meta.title = await CleanTitleTags(meta.title);
      meta = meta;
      await saveMeta();
    } catch (e) {
      errorMsg = String(e);
    }
  }

  const DISPLAY_CAP = 500;

  async function showCover() {
    view = "cover";
    try {
      coverInfo = await CoverInfo();
    } catch (e) {
      errorMsg = String(e);
    }
  }

  async function generateCover(reshuffle: boolean) {
    loading = true;
    errorMsg = "";
    statusMsg = "";
    try {
      genCoverUri = await GenerateCover(reshuffle);
      coverInfo = await CoverInfo();
    } catch (e) {
      errorMsg = String(e);
    } finally {
      loading = false;
    }
  }

  async function toggleApply() {
    errorMsg = "";
    statusMsg = "";
    try {
      if (coverInfo?.applied) await ClearCover();
      else await ApplyCover();
      coverInfo = await CoverInfo();
    } catch (e) {
      errorMsg = String(e);
    }
  }

  async function refreshPreview() {
    try {
      previewHtml = await Preview();
    } catch (e) {
      errorMsg = String(e);
    }
  }

  async function showPreview() {
    view = "preview";
    await refreshPreview();
  }

  async function openFile() {
    loading = true;
    errorMsg = "";
    statusMsg = "";
    try {
      const r = await Open();
      if (r) {
        result = r;
        enabled = new Set(r.rules.filter((x) => x.enabled).map((x) => x.id));
        filterRule = null;
        genCoverUri = "";
        coverInfo = null;
        meta = null;

        // подгрузить данные активной вкладки под новую книгу
        if (view === "meta") await showMeta();
        else if (view === "preview") await refreshPreview();
        else if (view === "cover") await showCover();
        // view === "findings" использует result напрямую — доп. загрузка не нужна
      }
    } catch (e) {
      errorMsg = String(e);
    } finally {
      loading = false;
    }
  }

  async function reload() {
    if (!result) return;
    loading = true;
    errorMsg = "";
    statusMsg = "";
    try {
      const r = await Reload(Array.from(enabled));
      if (r) result = r;
      if (view === "preview") await refreshPreview();
    } catch (e) {
      errorMsg = String(e);
    } finally {
      loading = false;
    }
  }

  async function toggleRule(id: string) {
    if (enabled.has(id)) enabled.delete(id);
    else enabled.add(id);
    enabled = new Set(enabled);
    await reload();
  }

  async function exportEpub() {
    loading = true;
    errorMsg = "";
    statusMsg = "";
    try {
      const out = await Export();
      if (out) statusMsg = "Сохранено: " + out;
    } catch (e) {
      errorMsg = String(e);
    } finally {
      loading = false;
    }
  }

  function selectRule(id: string) {
    filterRule = filterRule === id ? null : id;
  }

  function ctxParts(ctx: string) {
    const a = ctx.indexOf("⟦");
    const b = ctx.indexOf("⟧");
    if (a < 0 || b < 0) return { pre: ctx, mid: "", post: "" };
    return { pre: ctx.slice(0, a), mid: ctx.slice(a + 1, b), post: ctx.slice(b + 1) };
  }

  $: shown = result
    ? filterRule
      ? result.findings.filter((f) => f.ruleId === filterRule)
      : result.findings
    : [];
  $: shownCapped = shown.slice(0, DISPLAY_CAP);
</script>

<header>
  <div class="toolbar">
    <div class="brand">litagent</div>
    <button on:click={openFile} disabled={loading}>Открыть FB2…</button>
    <button on:click={exportEpub} disabled={loading || !result}>Собрать EPUB…</button>
    {#if result}
      <span class="tabs">
        <button class:tab-active={view === "findings"} on:click={() => (view = "findings")}>Правки</button>
        <button class:tab-active={view === "preview"} on:click={showPreview}>Превью</button>
        <button class:tab-active={view === "cover"} on:click={showCover}>Обложка</button>
        <button class:tab-active={view === "meta"} on:click={showMeta}>Метаданные</button>
      </span>
    {/if}
    <div class="spacer"></div>
    {#if loading}<span class="status">обработка…</span>{/if}
    {#if statusMsg}<span class="status ok">{statusMsg}</span>{/if}
    {#if errorMsg}<span class="status err">{errorMsg}</span>{/if}
  </div>
  {#if result}<div class="file" title={result.name}>{result.name}</div>{/if}
</header>

{#if !result}
  <div class="empty">
    <p>
      Откройте файл <b>.fb2</b>, чтобы проверить и исправить типографику,<br />
      а затем собрать чистый EPUB для Kindle.
    </p>
  </div>
{:else}
  <div class="meta">
    <div><span class="k">Книга</span> {result.meta.title || "—"}</div>
    <div><span class="k">Автор</span> {result.meta.author || "—"}</div>
    <div><span class="k">Язык</span> {result.meta.lang || "—"}</div>
    <div><span class="k">Глав</span> {result.meta.sections}</div>
    <div><span class="k">Обложка</span> {result.meta.hasCover ? "да" : "нет"}</div>
    <div><span class="k">Правок</span> {result.total}</div>
  </div>

  <main>
    <aside>
      <div class="aside-head">
        Правила
        {#if filterRule}<button class="link" on:click={() => (filterRule = null)}>сбросить фильтр</button>{/if}
      </div>
      <div class="legend">
        <b>A</b>/<b>B</b> применены — снимите галку, чтобы откатить.<br />
        <b>C</b> — подсказки; отметьте галку, чтобы применить.
      </div>
      <ul class="rules">
        {#each result.rules as r}
          <li class:zero={r.count === 0} class:active={filterRule === r.id}>
            <input
              type="checkbox"
              checked={enabled.has(r.id)}
              on:change={() => toggleRule(r.id)}
              title="Включить/откатить правило"
            />
            <span class="lvl lvl-{r.level}">{r.level}</span>
            <button class="rname" on:click={() => selectRule(r.id)}>{r.name}</button>
            <span class="cnt">{r.count}</span>
          </li>
        {/each}
      </ul>
    </aside>

    {#if view === "preview"}
      <section class="preview">{@html previewHtml}</section>
    {:else if view === "meta"}
      <section class="meta-tab">
        {#if meta}
          <div class="meta-hint">
            Правки применяются при «Собрать EPUB». Серию оставьте пустой, чтобы её не было в списке Kindle.
          </div>
          <div class="form">
            <label class="wide">
              <span>Название</span>
              <div class="row">
                <input bind:value={meta.title} on:change={saveMeta} />
                <button class="small" on:click={cleanTitle} title="Убрать [теги] из названия">убрать теги</button>
              </div>
            </label>
            <label class="wide">
              <span>Авторы (по одному в строке)</span>
              <textarea rows="2" bind:value={meta.authors} on:change={saveMeta}></textarea>
            </label>
            <label class="wide">
              <span>Переводчики (по одному в строке)</span>
              <textarea rows="2" bind:value={meta.translators} on:change={saveMeta}></textarea>
            </label>
            <label>
              <span>Язык</span>
              <input bind:value={meta.lang} on:change={saveMeta} placeholder="ru" />
            </label>
            <label>
              <span>Издатель</span>
              <input bind:value={meta.publisher} on:change={saveMeta} />
            </label>
            <label>
              <span>Серия</span>
              <input bind:value={meta.seriesName} on:change={saveMeta} placeholder="(пусто — без серии)" />
            </label>
            <label>
              <span>№ в серии</span>
              <input bind:value={meta.seriesNumber} on:change={saveMeta} />
            </label>
            <label>
              <span>Дата / год</span>
              <input bind:value={meta.date} on:change={saveMeta} />
            </label>
            <label>
              <span>ISBN</span>
              <input bind:value={meta.isbn} on:change={saveMeta} />
            </label>
            <label class="wide">
              <span>Ключевые слова (через запятую)</span>
              <input bind:value={meta.keywords} on:change={saveMeta} />
            </label>
            <label class="wide">
              <span>Аннотация</span>
              <textarea rows="5" bind:value={meta.annotation} on:change={saveMeta}></textarea>
            </label>
          </div>
        {:else}
          <div class="meta-hint">Загрузка метаданных…</div>
        {/if}
      </section>
    {:else if view === "cover"}
      <section class="cover-tab">
        <div class="cover-bar">
          <button on:click={() => generateCover(false)} disabled={loading}>Сгенерировать</button>
          <button on:click={() => generateCover(true)} disabled={loading || !genCoverUri}>Другой шаблон</button>
          {#if genCoverUri}
            <label class="apply-toggle" title="Снимите, чтобы оставить исходную обложку">
              <input type="checkbox" checked={coverInfo?.applied} on:change={toggleApply} />
              Использовать в EPUB
            </label>
          {/if}
          {#if coverInfo}
            <span class="cover-hint">
              Шаблонов: {coverInfo.templateCount}
              {#if coverInfo.applied}· <span class="ok">✓ будет в книге</span>{/if}
            </span>
          {/if}
        </div>
        <div class="cover-note">
          «Сгенерировать» сразу помечает обложку к сборке (галка «Использовать в EPUB» включена).
          Дальше — «Собрать EPUB…».
        </div>
        {#if coverInfo && coverInfo.templateCount === 0}
          <div class="cover-warn">
            В папке нет картинок. Положите изображения (.jpg/.png) в папку:<br />
            <code>{coverInfo.wallpaperDir}</code>
          </div>
        {/if}
        <div class="cover-cols">
          <div class="cover-col">
            <div class="cover-cap">Текущая обложка</div>
            {#if coverInfo?.originalDataUri}
              <img class="cover-img" src={coverInfo.originalDataUri} alt="текущая обложка" />
            {:else}
              <div class="cover-none">нет</div>
            {/if}
          </div>
          <div class="cover-col">
            <div class="cover-cap">Сгенерированная</div>
            {#if genCoverUri}
              <img class="cover-img" src={genCoverUri} alt="сгенерированная обложка" />
            {:else}
              <div class="cover-none">нажмите «Сгенерировать»</div>
            {/if}
          </div>
        </div>
      </section>
    {:else}
    <section class="findings">
      {#if shown.length === 0}
        <div class="none">Нет правок{filterRule ? " по этому правилу" : ""}.</div>
      {:else}
        <div class="findings-head">
          Показано {shownCapped.length} из {shown.length}
          {#if result.truncated && !filterRule}(всего {result.total}, список усечён){/if}
        </div>
        <ul class="flist">
          {#each shownCapped as f}
            <li>
              <span class="badge">{f.ruleId}</span>
              {#if f.section}<span class="sec">#{f.section}</span>{/if}
              {#if ctxParts(f.context).mid}
                <code class="ctx">{ctxParts(f.context).pre}<mark>{ctxParts(f.context).mid}</mark>{ctxParts(f.context).post}</code>
              {/if}
              <span class="diff"><del>{f.before}</del><span class="arrow">→</span><ins>{f.after}</ins></span>
            </li>
          {/each}
        </ul>
      {/if}
    </section>
    {/if}
  </main>
{/if}

<style>
  :global(html, body) {
    margin: 0;
    height: 100%;
    background: #1b2735;
    color: #e8edf2;
    font-family: "Nunito", -apple-system, Segoe UI, Roboto, sans-serif;
    font-size: 14px;
  }
  :global(#app) {
    height: 100vh;
    display: flex;
    flex-direction: column;
  }

  header {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 10px 14px;
    background: #15202b;
    border-bottom: 1px solid #2c3a48;
  }
  .toolbar {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .brand {
    font-weight: 800;
    letter-spacing: 0.5px;
    color: #6cc6ff;
  }
  header .file {
    color: #9fb3c8;
    font-style: italic;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .spacer {
    flex: 1;
  }
  button {
    background: #2b3a4a;
    color: #e8edf2;
    border: 1px solid #3a4d60;
    border-radius: 6px;
    padding: 6px 12px;
    cursor: pointer;
  }
  button:hover:not(:disabled) {
    background: #34465a;
  }
  button:disabled {
    opacity: 0.5;
    cursor: default;
  }
  .status {
    color: #9fb3c8;
  }
  .status.ok {
    color: #7fd18c;
  }
  .status.err {
    color: #ff8b8b;
  }

  .empty {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    color: #9fb3c8;
    text-align: center;
    line-height: 1.7;
  }

  .meta {
    display: flex;
    gap: 22px;
    flex-wrap: wrap;
    padding: 10px 14px;
    background: #1e2c3a;
    border-bottom: 1px solid #2c3a48;
  }
  .meta .k {
    color: #7c93a8;
    margin-right: 6px;
    font-size: 12px;
    text-transform: uppercase;
  }

  main {
    flex: 1;
    display: flex;
    min-height: 0;
  }
  aside {
    width: 340px;
    border-right: 1px solid #2c3a48;
    display: flex;
    flex-direction: column;
    min-height: 0;
  }
  .aside-head {
    padding: 10px 14px;
    color: #7c93a8;
    text-transform: uppercase;
    font-size: 12px;
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .link {
    background: none;
    border: none;
    color: #6cc6ff;
    cursor: pointer;
    padding: 0;
    text-transform: none;
  }
  .legend {
    padding: 0 14px 8px;
    color: #7c93a8;
    font-size: 12px;
  }
  ul.rules {
    list-style: none;
    margin: 0;
    padding: 0;
    overflow: auto;
  }
  ul.rules li {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 14px;
    border-bottom: 1px solid #243341;
  }
  ul.rules li.active {
    background: #243646;
  }
  ul.rules li.zero {
    opacity: 0.45;
  }
  .lvl {
    font-weight: 700;
    width: 18px;
    text-align: center;
    border-radius: 4px;
    font-size: 12px;
  }
  .lvl-A {
    background: #1f4d2e;
    color: #8be0a0;
  }
  .lvl-B {
    background: #5a4a1e;
    color: #e6c879;
  }
  .lvl-C {
    background: #3a4350;
    color: #aab8c6;
  }
  .rname {
    flex: 1;
    text-align: left;
    background: none;
    border: none;
    color: #e8edf2;
    cursor: pointer;
    padding: 0;
  }
  .rname:hover {
    color: #6cc6ff;
  }
  .cnt {
    color: #9fb3c8;
    font-variant-numeric: tabular-nums;
  }

  section.findings {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
  }
  .findings-head,
  .none {
    padding: 10px 14px;
    color: #7c93a8;
  }
  ul.flist {
    list-style: none;
    margin: 0;
    padding: 0;
    overflow: auto;
  }
  ul.flist li {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 6px 14px;
    border-bottom: 1px solid #243341;
    flex-wrap: wrap;
  }
  .badge {
    background: #2b3a4a;
    color: #9fd0ff;
    border-radius: 4px;
    padding: 1px 6px;
    font-size: 12px;
  }
  .sec {
    color: #7c93a8;
    font-size: 12px;
  }
  code.ctx {
    background: #11191f;
    border-radius: 4px;
    padding: 2px 6px;
    white-space: pre-wrap;
  }
  mark {
    background: #4a3a12;
    color: #ffd479;
    border-radius: 2px;
  }
  .diff {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }
  del {
    color: #ff9a9a;
    text-decoration: line-through;
    white-space: pre;
  }
  ins {
    color: #8be0a0;
    text-decoration: none;
    white-space: pre;
  }
  .arrow {
    color: #7c93a8;
  }

  .tabs {
    display: flex;
    gap: 4px;
  }
  .tab-active {
    background: #34465a;
    border-color: #6cc6ff;
    color: #cfe8ff;
  }

  section.preview {
    flex: 1;
    overflow: auto;
    padding: 18px 9%;
    background: #11191f;
    line-height: 1.65;
    color: #dfe7ee;
  }
  section.preview :global(p) {
    margin: 0 0 0.15em;
    text-indent: 1.4em;
    text-align: justify;
  }
  section.preview :global(h1),
  section.preview :global(h2),
  section.preview :global(h3),
  section.preview :global(h4) {
    text-align: center;
    text-indent: 0;
    color: #9fd0ff;
  }
  section.preview :global(.v) {
    text-indent: 0;
    text-align: left;
  }
  section.preview :global(.el) {
    height: 0.9em;
  }
  section.preview :global(blockquote) {
    margin: 1em 2em;
    color: #c4d2df;
    font-style: italic;
  }
  section.preview :global(.author) {
    text-align: right;
    text-indent: 0;
    font-style: italic;
  }
  section.preview :global(.sub) {
    text-align: center;
    text-indent: 0;
    font-weight: bold;
  }

  section.meta-tab {
    flex: 1;
    overflow: auto;
    min-height: 0;
    background: #11191f;
    padding: 16px;
  }
  .meta-hint {
    color: #9fb3c8;
    margin-bottom: 14px;
    line-height: 1.5;
  }
  .form {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 12px 18px;
    max-width: 900px;
  }
  .form label {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .form label.wide {
    grid-column: 1 / -1;
  }
  .form label span {
    color: #7c93a8;
    font-size: 12px;
    text-transform: uppercase;
  }
  .form input,
  .form textarea {
    background: #1b2735;
    color: #e8edf2;
    border: 1px solid #3a4d60;
    border-radius: 6px;
    padding: 7px 10px;
    font: inherit;
    resize: vertical;
  }
  .form input:focus,
  .form textarea:focus {
    outline: none;
    border-color: #6cc6ff;
  }
  .form .row {
    display: flex;
    gap: 8px;
  }
  .form .row input {
    flex: 1;
  }
  button.small {
    padding: 4px 10px;
    font-size: 12px;
    white-space: nowrap;
  }

  section.cover-tab {
    flex: 1;
    overflow: auto;
    display: flex;
    flex-direction: column;
    min-height: 0;
    background: #11191f;
  }
  .cover-bar {
    display: flex;
    align-items: center;
    gap: 10px;
    flex-wrap: wrap;
    padding: 12px 16px;
    border-bottom: 1px solid #243341;
  }
  .cover-hint {
    color: #9fb3c8;
  }
  .cover-hint .ok {
    color: #7fd18c;
  }
  .cover-note {
    padding: 0 16px 4px;
    color: #7c93a8;
    font-size: 12px;
  }
  .apply-toggle {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    color: #cfe8ff;
    cursor: pointer;
    user-select: none;
  }
  .cover-warn {
    margin: 14px 16px;
    padding: 12px 14px;
    background: #2a2113;
    border: 1px solid #5a4a1e;
    border-radius: 6px;
    color: #e6c879;
    line-height: 1.6;
  }
  .cover-warn code {
    color: #ffd479;
  }
  .cover-cols {
    display: flex;
    gap: 28px;
    padding: 20px 16px;
    justify-content: center;
    flex-wrap: wrap;
  }
  .cover-col {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
  }
  .cover-cap {
    color: #7c93a8;
    text-transform: uppercase;
    font-size: 12px;
  }
  .cover-img {
    width: 280px;
    max-width: 40vw;
    border-radius: 6px;
    box-shadow: 0 6px 24px rgba(0, 0, 0, 0.5);
    background: #1b2735;
  }
  .cover-none {
    width: 280px;
    max-width: 40vw;
    aspect-ratio: 5 / 8;
    display: flex;
    align-items: center;
    justify-content: center;
    border: 1px dashed #3a4d60;
    border-radius: 6px;
    color: #7c93a8;
    text-align: center;
    padding: 0 10px;
  }
</style>
