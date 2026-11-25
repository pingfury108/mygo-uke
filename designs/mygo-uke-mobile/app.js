(function () {
  "use strict";

  const DURATION = 198;
  const state = new WeakMap();
  const editPhone = document.querySelector('[data-phone="edit"]');
  const ipadScreen = document.querySelector(".ipad-screen");
  let toastTimer = 0;

  const $ = (root, selector) => root.querySelector(selector);
  const $$ = (root, selector) => Array.from(root.querySelectorAll(selector));

  function formatTime(seconds) {
    const value = Math.max(0, Math.min(DURATION, seconds));
    const minutes = Math.floor(value / 60);
    const rest = value - minutes * 60;
    return String(minutes).padStart(2, "0") + ":" + rest.toFixed(1).padStart(4, "0");
  }

  function percent(seconds) {
    return Math.max(0, Math.min(100, (seconds / DURATION) * 100));
  }

  function scopeOf(element) {
    return element.closest(".phone, .ipad-screen");
  }

  function ensureState(scope) {
    if (!state.has(scope)) {
      const initial = Number(scope.dataset.time || 42);
      state.set(scope, { time: initial, playing: false, timer: 0 });
    }
    return state.get(scope);
  }

  function updateScope(scope) {
    if (!scope) return;
    const s = ensureState(scope);
    const pct = percent(s.time);

    $$(scope, "[data-clock]").forEach((el) => {
      el.textContent = formatTime(s.time);
    });

    $$(scope, ".scrub-fill, .timeline-fill").forEach((el) => {
      el.style.width = pct + "%";
    });
    $$(scope, ".scrub-head, .playhead").forEach((el) => {
      el.style.left = pct + "%";
    });

    const bars = $$(scope, ".waveform i");
    bars.forEach((bar, index) => {
      bar.classList.toggle("is-played", index / Math.max(1, bars.length - 1) <= pct / 100);
    });

    scope.classList.toggle("is-playing", s.playing);
    $$(scope, "[data-play]").forEach((button) => {
      button.setAttribute("aria-label", s.playing ? "暂停" : "播放");
    });
  }

  function setScopeTime(scope, seconds) {
    if (!scope) return;
    const s = ensureState(scope);
    s.time = Math.max(0, Math.min(DURATION, seconds));
    updateScope(scope);
  }

  function togglePlay(scope) {
    if (!scope) return;
    const s = ensureState(scope);
    if (s.playing) {
      s.playing = false;
      window.clearInterval(s.timer);
      s.timer = 0;
      updateScope(scope);
      return;
    }
    if (s.time >= DURATION) s.time = 0;
    s.playing = true;
    s.timer = window.setInterval(() => {
      s.time += 0.1;
      if (s.time >= DURATION) {
        s.time = DURATION;
        s.playing = false;
        window.clearInterval(s.timer);
        s.timer = 0;
      }
      updateScope(scope);
    }, 100);
    updateScope(scope);
  }

  function showToast(message) {
    const box = document.querySelector("[data-toast-box]");
    if (!box) return;
    box.textContent = message;
    box.classList.add("is-on");
    window.clearTimeout(toastTimer);
    toastTimer = window.setTimeout(() => box.classList.remove("is-on"), 1800);
  }

  function hexToRgba(hex, alpha) {
    const value = hex.replace("#", "");
    const red = parseInt(value.slice(0, 2), 16);
    const green = parseInt(value.slice(2, 4), 16);
    const blue = parseInt(value.slice(4, 6), 16);
    return "rgba(" + red + ", " + green + ", " + blue + ", " + alpha + ")";
  }

  function showFret(scope, chord) {
    if (!scope) return;
    const card = $(scope, "[data-fret-card]");
    if (!card) return;
    $$(card, "[data-shape]").forEach((shape) => {
      shape.style.display = shape.dataset.shape === chord ? "" : "none";
    });
    const name = $(card, "[data-fret-name]");
    if (name) name.textContent = chord;
  }

  function buildWaveform() {
    $$(document, "[data-waveform]").forEach((waveform) => {
      const count = 72;
      for (let i = 0; i < count; i += 1) {
        const bar = document.createElement("i");
        const envelope = 0.28 + 0.72 * Math.abs(Math.sin(i * 0.41));
        const detail = 0.42 + 0.58 * Math.abs(Math.sin(i * 1.73 + 0.8));
        const height = 10 + envelope * detail * 84;
        bar.style.height = Math.min(92, height) + "%";
        waveform.appendChild(bar);
      }
    });
  }

  function buildTimelineMarks() {
    $$(document, "[data-timeline]").forEach((timeline) => {
      for (let i = 0; i < 12; i += 1) {
        const tick = document.createElement("span");
        tick.className = "bar-tick";
        tick.style.left = (i / 12) * 100 + "%";
        if (i === 0 || i === 3 || i === 6 || i === 9) {
          const label = document.createElement("span");
          label.textContent = String(i + 1);
          tick.appendChild(label);
        }
        timeline.appendChild(tick);
      }
    });
  }

  function syncLineSelection(target, scope) {
    const list = target.closest(".line-list, .ins-list");
    if (list) {
      $$(list, "[data-line]").forEach((row) => {
        row.classList.toggle("is-current", row === target);
        const badge = row.querySelector(".line-state");
        if (badge) {
          if (row === target) {
            badge.textContent = "播放中";
            badge.classList.add("is-playing");
          } else {
            badge.classList.remove("is-playing");
            if (!badge.querySelector(".status-dot")) {
              badge.innerHTML = '<i class="status-dot is-good"></i>';
            }
          }
        }
      });
    }

    const time = Number(target.dataset.time || 0);
    if (scope) setScopeTime(scope, time);
    if (editPhone) setScopeTime(editPhone, time);
    const text = target.querySelector(".line-text, .ins-text");
    showToast("已定位到 " + formatTime(time) + (text ? " · " + text.textContent.trim() : ""));
  }

  function startExport(scope, config) {
    if (!scope) return;
    if (scope.classList.contains("is-export-done")) {
      showToast("已经导出完成，可以在文件中找到 MP4");
      return;
    }

    const progress = $(scope, config.progress);
    const fill = $(scope, config.fill);
    const percentLabel = $(scope, config.percent);
    const button = $(scope, config.button);
    if (!progress || !fill || !percentLabel || !button) return;

    scope.classList.add("is-exporting");
    progress.classList.add("is-on");
    button.disabled = true;
    button.textContent = "导出中 0%";
    let value = 0;
    const timer = window.setInterval(() => {
      value = Math.min(100, value + 2 + Math.random() * 4);
      const rounded = Math.round(value);
      fill.style.width = value + "%";
      percentLabel.textContent = rounded + "%";
      button.textContent = rounded < 100 ? "导出中 " + rounded + "%" : "导出完成 · 保存到文件";
      if (value >= 100) {
        window.clearInterval(timer);
        scope.classList.remove("is-exporting");
        scope.classList.add("is-export-done");
        button.disabled = false;
        progress.querySelector(".export-progress-head span").textContent = "已完成";
        const note = progress.querySelector("p");
        if (note) note.textContent = "已保存到「文件」· MyGo Uke / Exports";
        showToast("导出完成 · 已保存到文件");
      }
    }, 80);
  }

  function openExport(phone) {
    if (!phone) return;
    phone.classList.remove("is-export-closed");
  }

  function closeExport(phone) {
    if (!phone) return;
    phone.classList.add("is-export-closed");
  }

  function openIpadExport() {
    if (!ipadScreen) return;
    ipadScreen.classList.add("is-export-open");
  }

  function closeIpadExport() {
    if (!ipadScreen) return;
    ipadScreen.classList.remove("is-export-open");
  }

  // Waveform and timeline scaffolding.
  buildWaveform();
  buildTimelineMarks();

  // Playback, scrub, mute and seek.
  document.addEventListener("click", (event) => {
    const play = event.target.closest("[data-play]");
    if (play) {
      event.preventDefault();
      togglePlay(scopeOf(play));
      return;
    }

    const seek = event.target.closest("[data-seek]");
    if (seek) {
      const scope = scopeOf(seek);
      const s = ensureState(scope);
      setScopeTime(scope, s.time + Number(seek.dataset.seek || 0));
      return;
    }

    const mute = event.target.closest("[data-mute]");
    if (mute) {
      const scope = scopeOf(mute);
      if (scope) {
        scope.classList.toggle("is-muted");
        showToast(scope.classList.contains("is-muted") ? "已静音预览" : "已恢复声音");
      }
      return;
    }

    const scrub = event.target.closest("[data-scrub]");
    if (scrub) {
      const scope = scopeOf(scrub);
      const track = scrub.querySelector(".scrub-track");
      const rect = track.getBoundingClientRect();
      const ratio = Math.max(0, Math.min(1, (event.clientX - rect.left) / rect.width));
      setScopeTime(scope, DURATION * ratio);
      return;
    }

    const video = event.target.closest("[data-video]");
    if (video && !event.target.closest("button, a")) {
      togglePlay(scopeOf(video));
    }
  });

  // Chord chips update the current chord and the fingering diagram.
  $$(document, ".chord-chip").forEach((chip) => {
    chip.addEventListener("click", () => {
      const scope = scopeOf(chip);
      const chord = chip.dataset.chord || chip.textContent.trim();
      const current = $(scope, ".chord-chip-current");
      if (current) {
        const label = $(current, "[data-chord-label]");
        if (label) label.textContent = chord;
        current.dataset.chord = chord;
      }
      showFret(scope, chord);
      chip.classList.add("is-picked");
      window.setTimeout(() => chip.classList.remove("is-picked"), 360);
      showToast("当前和弦改为 " + chord);
    });
  });

  // Line selection in the lyrics tab and iPad inspector.
  $$(document, "[data-line]").forEach((row) => {
    row.addEventListener("click", () => {
      syncLineSelection(row, scopeOf(row));
    });
  });

  // Character-level chord editing.
  $$(document, ".char-row").forEach((row) => {
    $$(row, ".char-cell").forEach((cell) => {
      cell.addEventListener("click", () => {
        $$(row, ".char-cell").forEach((item) => item.classList.toggle("is-current", item === cell));
        showToast("已选中「" + cell.textContent.trim() + "」· 再点和弦改它");
      });
    });
  });

  $$(document, ".chord-span").forEach((span) => {
    span.addEventListener("click", () => {
      const cycle = ["C", "G", "Am", "F"];
      const current = span.textContent.trim();
      const next = cycle[(cycle.indexOf(current) + 1) % cycle.length];
      span.textContent = next;
      $$(span.parentElement, ".chord-span").forEach((item) => item.classList.toggle("is-on", item === span));
      const scope = scopeOf(span);
      const hero = $(scope, "[data-hero-chord]");
      if (hero) hero.textContent = next;
      showFret(scope, next);
      showToast("这一字的和弦改为 " + next);
    });
  });

  // Bottom navigation scrolls to the matching screen.
  $$(document, "[data-goto]").forEach((button) => {
    button.addEventListener("click", () => {
      const target = document.getElementById(button.dataset.goto);
      if (!target) return;
      target.scrollIntoView({ behavior: "smooth", block: "center" });
      showToast("已切到「" + (button.querySelector("span")?.textContent || "页面") + "」");
    });
  });

  // Export sheets.
  $$(document, "[data-export-open]").forEach((button) => {
    button.addEventListener("click", () => openExport(scopeOf(button)));
  });
  $$(document, "[data-export-close]").forEach((button) => {
    button.addEventListener("click", () => closeExport(scopeOf(button)));
  });
  $$(document, "[data-export-backdrop]").forEach((backdrop) => {
    backdrop.addEventListener("click", () => closeExport(scopeOf(backdrop)));
  });
  const phoneExportStart = document.querySelector("[data-export-start]");
  if (phoneExportStart) {
    phoneExportStart.addEventListener("click", () => {
      startExport(scopeOf(phoneExportStart), {
        progress: "[data-export-progress]",
        fill: "[data-export-fill]",
        percent: "[data-export-percent]",
        button: "[data-export-start]",
      });
    });
  }

  $$(document, "[data-export-open-ipad]").forEach((button) => {
    button.addEventListener("click", openIpadExport);
  });
  $$(document, "[data-export-close-ipad]").forEach((button) => {
    button.addEventListener("click", closeIpadExport);
  });
  const ipadExportBackdrop = document.querySelector("[data-export-backdrop-ipad]");
  if (ipadExportBackdrop) ipadExportBackdrop.addEventListener("click", closeIpadExport);
  const ipadExportStart = document.querySelector("[data-export-start-ipad]");
  if (ipadExportStart) {
    ipadExportStart.addEventListener("click", () => {
      startExport(ipadScreen, {
        progress: "[data-export-progress-ipad]",
        fill: "[data-export-fill-ipad]",
        percent: "[data-export-percent-ipad]",
        button: "[data-export-start-ipad]",
      });
    });
  }

  // Inspector tabs and collapse.
  $$(document, "[data-ins-tab]").forEach((tab) => {
    tab.addEventListener("click", () => {
      $$(document, "[data-ins-tab]").forEach((item) => item.classList.toggle("is-on", item === tab));
      $$(document, "[data-ins-panel]").forEach((panel) => {
        panel.classList.toggle("is-on", panel.dataset.insPanel === tab.dataset.insTab);
      });
    });
  });

  const inspectorToggle = document.querySelector("[data-inspector-toggle]");
  if (inspectorToggle && ipadScreen) {
    inspectorToggle.addEventListener("click", () => {
      const focused = ipadScreen.classList.toggle("is-focus");
      inspectorToggle.setAttribute("aria-label", focused ? "展开检查器" : "收起检查器");
      showToast(focused ? "检查器已收起 · 画布扩宽" : "检查器已展开");
    });
  }

  // Style controls update the iPad preview live.
  const styleRange = (name, apply) => {
    const input = document.querySelector('[data-range="' + name + '"]');
    if (!input) return;
    input.addEventListener("input", () => {
      apply(Number(input.value));
      const label = input.parentElement.querySelector("b");
      if (label) {
        label.textContent = name === "y" ? input.value + "%" : input.value;
      }
    });
  };

  if (ipadScreen) {
    styleRange("chord", (value) => {
      $$(ipadScreen, ".chord-chip-current b").forEach((el) => {
        el.style.fontSize = 16 + (value - 32) * 0.19 + "px";
      });
    });
    styleRange("sub", (value) => {
      $$(ipadScreen, ".lyric-current").forEach((el) => {
        el.style.fontSize = 18 + value * 0.15 + "px";
      });
    });
    styleRange("y", (value) => {
      const bottom = 8 + (92 - value) * 1.2;
      $$(ipadScreen, ".overlay-bottom").forEach((el) => {
        el.style.bottom = bottom + "px";
      });
    });
  }

  $$(document, ".swatch").forEach((swatch) => {
    swatch.addEventListener("click", () => {
      const color = getComputedStyle(swatch).getPropertyValue("--sw").trim();
      if (!color || !ipadScreen) return;
      ipadScreen.style.setProperty("--amber", color);
      ipadScreen.style.setProperty("--amber-soft", hexToRgba(color, 0.12));
      ipadScreen.style.setProperty("--amber-line", hexToRgba(color, 0.28));
      $$(document, ".swatch").forEach((item) => item.classList.toggle("is-on", item === swatch));
      showToast("强调色已应用");
    });
  });

  // Annotation toggle.
  const annotationButton = document.querySelector("[data-annotations]");
  if (annotationButton) {
    annotationButton.addEventListener("click", () => {
      const on = document.body.classList.toggle("show-annotations");
      annotationButton.innerHTML =
        '<svg class="icon"><use href="#i-spark"/></svg>' + (on ? "隐藏标注" : "显示标注");
    });
  }

  // Generic demo toasts.
  document.addEventListener("click", (event) => {
    const item = event.target.closest("[data-toast]");
    if (!item) return;
    showToast(item.dataset.toast);
  });

  // Keep the initial preview in sync.
  if (editPhone) {
    ensureState(editPhone).time = 42;
    updateScope(editPhone);
  }
  if (ipadScreen) {
    ensureState(ipadScreen).time = 42;
    updateScope(ipadScreen);
  }

  // Pause playback when the page is hidden.
  document.addEventListener("visibilitychange", () => {
    if (!document.hidden) return;
    [editPhone, ipadScreen].forEach((scope) => {
      if (!scope) return;
      const s = ensureState(scope);
      if (!s.playing) return;
      s.playing = false;
      window.clearInterval(s.timer);
      s.timer = 0;
      updateScope(scope);
    });
  });
})();
