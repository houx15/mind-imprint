(() => {
  const state = {
    mode: "overview",
    awakeningStep: 0,
    interestStep: 0,
    principle: "",
    navigator: "",
    seed: "",
    currentCategory: "",
    currentObject: "",
    currentLens: "",
    detail: "",
    branchCount: 2,
    questionCount: 4,
  };

  const navigatorProfiles = {
    nova: { name: "热血同好 NOVA", short: "NOVA", copy: "会把你的亮点放大，陪你把灵感推进行动。" },
    kiro: { name: "腹黑军师 KIRO", short: "KIRO", copy: "会持续追问证据，提醒你把判断说得更清楚。" },
    sage: { name: "资深向导 SAGE", short: "SAGE", copy: "会拆开问题并提供线索，但路仍然由你自己走。" },
  };

  const seedLabels = { music: "音乐", game: "游戏", nature: "自然", create: "创作", custom: "自定义兴趣" };
  const categoryLabels = { music: "音乐与声音", story: "故事与人物", craft: "结构与机制", world: "世界与关系", custom: "自定义兴趣" };
  const lensLabels = { story: "感受与故事", mechanism: "结构与机制", impact: "关系与影响", custom: "自定义探索" };
  const lensPrompts = {
    story: ["提示方向 · 从具体体验开始", "请说一个人物、情绪或画面：什么让你愿意继续追？"],
    mechanism: ["提示方向 · 从细节机制开始", "请说一个画面、声音、规则或过程：什么让你想把它拆开？"],
    impact: ["提示方向 · 从关系与场景开始", "请说一个使用场景、评价分歧或影响：什么让你想知道它为什么被需要？"],
    custom: ["提示方向 · 先写下你的问题", "请说一个具体瞬间或经历，让 AI 知道从哪里陪你开始查。"],
  };

  const $ = (selector, parent = document) => parent.querySelector(selector);
  const $$ = (selector, parent = document) => [...parent.querySelectorAll(selector)];
  const toast = $(".toast");
  let toastTimer;

  function showToast(message) {
    toast.textContent = message;
    toast.classList.add("is-visible");
    window.clearTimeout(toastTimer);
    toastTimer = window.setTimeout(() => toast.classList.remove("is-visible"), 2800);
  }

  function setMode(mode) {
    state.mode = mode;
    $$(".mode-button").forEach((button) => {
      const active = button.dataset.mode === mode;
      button.classList.toggle("is-active", active);
      button.setAttribute("aria-selected", String(active));
    });
    $$(".view").forEach((view) => {
      const active = view.dataset.view === mode;
      view.hidden = !active;
      view.classList.toggle("is-visible", active);
    });
    if (window.location.hash !== `#${mode}`) history.replaceState(null, "", `#${mode}`);
    window.scrollTo({ top: 0, behavior: "smooth" });
  }

  function updateStepper(kind, step) {
    const wrapper = $(`[data-stepper="${kind}"]`);
    $$(".step", wrapper).forEach((item, index) => {
      item.classList.toggle("is-active", index === step);
      item.classList.toggle("is-done", index < step);
    });
  }

  function showAwakeningStep(step) {
    state.awakeningStep = step;
    $$("[data-aw-step-panel]").forEach((panel) => { panel.hidden = Number(panel.dataset.awStepPanel) !== step; });
    updateStepper("awakening", step);
    refreshAwakeningRail();
    window.scrollTo({ top: 0, behavior: "smooth" });
  }

  function showInterestStep(step) {
    state.interestStep = step;
    $$("[data-interest-step-panel]").forEach((panel) => { panel.hidden = Number(panel.dataset.interestStepPanel) !== step; });
    updateStepper("interest", step);
    refreshInterestRail();
    window.scrollTo({ top: 0, behavior: "smooth" });
  }

  function refreshAwakeningRail() {
    const profile = navigatorProfiles[state.navigator];
    $("[data-assistant-signal]").textContent = profile ? profile.name : "等待你的选择";
    $("[data-assistant-copy]").textContent = profile ? profile.copy : "助手风格会在完成连接后确定。现在先保留空白。";
    $("[data-seed-count]").textContent = state.seed ? "1 / 1" : "0 / 1";
    $("[data-tree-copy]").textContent = state.seed ? `已记录“${seedLabels[state.seed]}”作为冷启动线索；正式兴趣测试仍在我的树中单独发生。` : "觉醒协议里的兴趣只作为冷启动信号，不是正式测试结果。";
  }

  function refreshInterestRail() {
    const category = categoryLabels[state.currentCategory];
    const lens = lensLabels[state.currentLens];
    const object = state.currentObject || category || "等待兴趣对象";
    $("[data-preview-object]").textContent = object;
    $("[data-preview-lens]").textContent = lens || "等待追问方向";
    $("[data-test-state]").textContent = state.interestStep === 0 ? "尚未开始" : state.interestStep === 4 ? "已写入树" : "探索中";
    $$('[data-branch-count]').forEach((node) => { node.textContent = state.branchCount; });
    $$('[data-question-count]').forEach((node) => { node.textContent = state.questionCount; });
    $("[data-tree-assistant]").textContent = navigatorProfiles[state.navigator]?.short || "待连接";
    $(".tree-rail-card").classList.toggle("has-new", state.interestStep === 4);
  }

  function chooseSingle(selector, attribute, value) {
    $$(selector).forEach((item) => item.setAttribute("aria-pressed", String(item.dataset[attribute] === value)));
  }

  function selectedOrNotify(value, message) {
    if (!value) { showToast(message); return false; }
    return true;
  }

  function normalizeObject(value) {
    return value.trim().replace(/\s+/g, " ").slice(0, 60);
  }

  $$("[data-mode]").forEach((button) => button.addEventListener("click", () => setMode(button.dataset.mode)));
  $$('[data-mode-link]').forEach((button) => button.addEventListener("click", (event) => { event.preventDefault(); setMode(button.dataset.modeLink); }));
  $$('[data-open-mode]').forEach((button) => button.addEventListener("click", () => setMode(button.dataset.openMode)));

  $$('[data-aw-next]').forEach((button) => button.addEventListener("click", () => {
    const next = Number(button.dataset.awNext);
    if (next === 2 && !selectedOrNotify(state.principle, "请先选择一种协作原则")) return;
    if (next === 3 && !selectedOrNotify(state.navigator, "请选择一个愿意继续对话的印记助手")) return;
    if (next === 4 && !selectedOrNotify(state.seed, "请先留下一个兴趣冷启动信号")) return;
    if (next === 4) {
      $("[data-awakening-summary='principle']").textContent = state.principle === "partner" ? "AI 参与，但判断归我" : "任务交给 AI（可随时改）";
      $("[data-awakening-summary='navigator']").textContent = navigatorProfiles[state.navigator].name;
      $("[data-awakening-summary='seed']").textContent = seedLabels[state.seed] + (state.seedDetail ? ` · ${state.seedDetail}` : "");
    }
    showAwakeningStep(next);
  }));
  $$('[data-aw-back]').forEach((button) => button.addEventListener("click", () => showAwakeningStep(Number(button.dataset.awBack))));

  $$('[data-principle]').forEach((button) => button.addEventListener("click", () => {
    state.principle = button.dataset.principle;
    chooseSingle("[data-principle]", "principle", state.principle);
    const response = $(`[data-principle-response]`);
    response.hidden = false;
    response.textContent = state.principle === "partner" ? "记录在案。这条路更慢，但留下的印记是你自己的。" : "记录在案。AI 可以帮你完成任务，但判断权仍然可以随时回到你手里。";
  }));

  $$('[data-navigator]').forEach((button) => button.addEventListener("click", () => {
    state.navigator = button.dataset.navigator;
    chooseSingle("[data-navigator]", "navigator", state.navigator);
    refreshAwakeningRail();
  }));

  $$('[data-seed]').forEach((button) => button.addEventListener("click", () => {
    state.seed = button.dataset.seed;
    chooseSingle("[data-seed]", "seed", state.seed);
    refreshAwakeningRail();
  }));
  $("#awakening-seed-detail").addEventListener("input", (event) => { state.seedDetail = event.target.value.trim(); });

  $$('[data-interest-next]').forEach((button) => button.addEventListener("click", () => {
    const next = Number(button.dataset.interestNext);
    if (next === 2) {
      state.currentObject = normalizeObject($("#interest-object").value);
      if (!selectedOrNotify(state.currentCategory || state.currentObject, "请先选择一个兴趣方向，或写下具体兴趣对象")) return;
      if (!state.currentObject) state.currentObject = categoryLabels[state.currentCategory];
      $("[data-current-object]").textContent = state.currentObject;
    }
    if (next === 3 && !selectedOrNotify(state.currentLens, "请选择一个愿意继续靠近的追问方向")) return;
    if (next === 4) {
      state.detail = $("#interest-detail").value.trim();
      if (!selectedOrNotify(state.detail, "请留下一个具体瞬间、细节或体验")) return;
      const category = categoryLabels[state.currentCategory] || "兴趣探索";
      $("[data-result-object]").textContent = state.currentObject || category;
      $("[data-result-detail]").textContent = state.detail;
      $("[data-current-category-copy]").textContent = category;
      $("[data-current-lens-copy]").textContent = lensLabels[state.currentLens] || "开放探索";
      $("[data-interest-prompt]").textContent = lensPrompts[state.currentLens]?.[1] || lensPrompts.custom[1];
      $("[data-result-next]").textContent = state.currentLens === "mechanism" ? "找一段真实画面或规则，把你的猜想拆成可以验证的步骤。" : state.currentLens === "impact" ? "找一个真实使用场景或不同评价，比较它如何影响不同的人。" : "去找一段真实材料，看看这个问题还能被怎样回答。";
      state.branchCount += 1;
      state.questionCount += 2;
    }
    showInterestStep(next);
  }));
  $$('[data-interest-back]').forEach((button) => button.addEventListener("click", () => showInterestStep(Number(button.dataset.interestBack))));

  $$('[data-current-category]').forEach((button) => button.addEventListener("click", () => {
    state.currentCategory = button.dataset.currentCategory;
    chooseSingle("[data-current-category]", "currentCategory", state.currentCategory);
    refreshInterestRail();
  }));
  $$('[data-current-lens]').forEach((button) => button.addEventListener("click", () => {
    state.currentLens = button.dataset.currentLens;
    chooseSingle("[data-current-lens]", "currentLens", state.currentLens);
    const [title, prompt] = lensPrompts[state.currentLens] || lensPrompts.custom;
    $("[data-interest-prompt-title]").textContent = title;
    $("[data-interest-prompt]").textContent = prompt;
    $("[data-current-lens-copy]").textContent = lensLabels[state.currentLens];
    refreshInterestRail();
  }));

  $$('[data-interest-insert]').forEach((button) => button.addEventListener("click", () => {
    $("#interest-object").value = button.dataset.interestInsert;
    $("#interest-object").focus();
  }));
  $$('[data-detail-insert]').forEach((button) => button.addEventListener("click", () => {
    const field = $("#interest-detail");
    field.value = field.value ? `${field.value}\n${button.dataset.detailInsert}` : button.dataset.detailInsert;
    field.focus();
  }));
  $$('[data-restart-interest]').forEach((button) => button.addEventListener("click", () => {
    state.interestStep = 1; state.currentCategory = ""; state.currentObject = ""; state.currentLens = ""; state.detail = "";
    $("#interest-object").value = ""; $("#interest-detail").value = "";
    $$('[data-current-category]').forEach((item) => item.setAttribute("aria-pressed", "false"));
    $$('[data-current-lens]').forEach((item) => item.setAttribute("aria-pressed", "false"));
    showInterestStep(1);
  }));

  window.addEventListener("hashchange", () => {
    const mode = ["overview", "awakening", "interest"].includes(location.hash.slice(1)) ? location.hash.slice(1) : "overview";
    setMode(mode);
  });

  const initialMode = ["overview", "awakening", "interest"].includes(location.hash.slice(1)) ? location.hash.slice(1) : "overview";
  setMode(initialMode);
  refreshAwakeningRail();
  refreshInterestRail();
})();
