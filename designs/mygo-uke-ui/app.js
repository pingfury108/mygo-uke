(() => {
  const template = document.getElementById("desk-template");

  function cloneDesk(theme) {
    const node = template.content.cloneNode(true);
    const win = node.querySelector(".app-window");
    win.classList.add("theme-" + theme);
    return node;
  }

  document.querySelectorAll("[data-clone-theme]").forEach((host) => {
    host.appendChild(cloneDesk(host.dataset.cloneTheme));
  });

  // ── Ukulele chord diagrams ──────────────────────────────────

  function drawDiagram(el) {
    const frets = (el.dataset.frets || "")
      .split(",")
      .map((n) => Number(n.trim()))
      .filter((n) => Number.isFinite(n));
    if (frets.length !== 4) return;

    const w = el.clientWidth || 52;
    const h = el.clientHeight || 62;
    el.innerHTML = "";

    const left = w * 0.1;
    const right = w * 0.9;
    const top = h * 0.18;
    const bottom = h * 0.9;
    const spanX = (right - left) / 3;
    const spanY = (bottom - top) / 4;
    const base = Math.max(1, Number(el.dataset.base || 1));

    for (let i = 0; i < 4; i += 1) {
      const line = document.createElement("i");
      line.className = "string";
      line.style.left = `${left + spanX * i}px`;
      el.appendChild(line);
    }

    for (let k = 0; k <= 4; k += 1) {
      const line = document.createElement("i");
      line.className = k === 0 ? "nut" : "fret";
      line.style.top = `${top + spanY * k}px`;
      el.appendChild(line);
    }

    const dotSize = Math.max(4, Math.min(w, h) * 0.075);
    frets.forEach((fret, i) => {
      const x = left + spanX * i;
      if (fret < 0) {
        const mute = document.createElement("b");
        mute.className = "mute";
        mute.textContent = "×";
        mute.style.left = `${x}px`;
        mute.style.top = `${top - h * 0.075}px`;
        el.appendChild(mute);
        return;
      }
      if (fret === 0) {
        const open = document.createElement("i");
        open.className = "open";
        open.style.left = `${x}px`;
        open.style.top = `${top - h * 0.075}px`;
        el.appendChild(open);
        return;
      }
      if (fret < base || fret > base + 3) return;
      const dot = document.createElement("i");
      dot.className = "dot";
      dot.style.left = `${x}px`;
      dot.style.top = `${top + spanY * (fret - base + 0.5)}px`;
      dot.style.width = `${dotSize}px`;
      dot.style.height = `${dotSize}px`;
      el.appendChild(dot);
    });
  }

  function drawAllDiagrams() {
    document.querySelectorAll(".uke-diagram").forEach(drawDiagram);
  }

  // ── Decorative waveform ─────────────────────────────────────

  function fillWave(el) {
    if (el.childElementCount) return;
    const count = 96;
    const played = 0.2121;
    for (let i = 0; i < count; i += 1) {
      const v =
        0.28 +
        0.44 * Math.abs(Math.sin(i * 0.61) + 0.62 * Math.sin(i * 0.17 + 1.1)) +
        0.14 * Math.abs(Math.sin(i * 1.7 + 0.4));
      const bar = document.createElement("i");
      bar.className = "wave-bar" + (i / count < played ? " played" : "");
      bar.style.height = `${Math.max(8, Math.min(100, v * 100))}%`;
      el.appendChild(bar);
    }
  }

  // ── Fit fixed-size artboards into their stage ───────────────

  function fitToStage(stage) {
    const inner = stage.querySelector(".fit-inner");
    if (!inner) return;
    const scale = stage.clientWidth / inner.offsetWidth;
    inner.style.transform = `scale(${scale})`;
    stage.style.setProperty("--fit-scale", String(scale));
    if (stage.dataset.fit === "1080x1920" || stage.dataset.fit === "1920x1080") {
      drawAllDiagrams();
    }
  }

  function fitAll() {
    document.querySelectorAll("[data-fit]").forEach(fitToStage);
  }

  // ── Time and playhead ───────────────────────────────────────

  const totalSeconds = 3 * 60 + 18;
  const playerState = new WeakMap();

  function paintProgress(win, fraction) {
    const clamped = Math.max(0, Math.min(1, fraction));
    const playhead = win.querySelector(".playhead");
    if (playhead) playhead.style.left = `${clamped * 100}%`;

    const time = win.querySelector(".time");
    if (time && time.firstChild && time.firstChild.nodeType === Node.TEXT_NODE) {
      const seconds = Math.round(totalSeconds * clamped);
      const mm = String(Math.floor(seconds / 60)).padStart(2, "0");
      const ss = String(seconds % 60).padStart(2, "0");
      time.firstChild.nodeValue = `${mm}:${ss}`;
    }

    win.querySelectorAll(".wave-bar").forEach((bar, i, bars) => {
      bar.classList.toggle("played", i / bars.length < clamped);
    });
  }

  function stateFor(win) {
    let state = playerState.get(win);
    if (!state) {
      state = { fraction: 0.2121, raf: 0, playing: false };
      playerState.set(win, state);
    }
    return state;
  }

  const playIcon =
    '<svg viewBox="0 0 12 12" fill="currentColor" aria-hidden="true"><path d="M3 1.6v8.8L10.4 6 3 1.6Z"></path></svg>';
  const pauseIcon =
    '<svg viewBox="0 0 12 12" fill="currentColor" aria-hidden="true"><path d="M3 1.6h2.1v8.8H3V1.6Zm3.9 0H9v8.8H6.9V1.6Z"></path></svg>';

  function stopPlayback(win) {
    const state = stateFor(win);
    state.playing = false;
    cancelAnimationFrame(state.raf);
    const button = win.querySelector(".play-btn");
    if (button) {
      button.innerHTML = playIcon;
      button.setAttribute("aria-label", "播放");
    }
  }

  function startPlayback(win) {
    const state = stateFor(win);
    state.playing = true;
    const button = win.querySelector(".play-btn");
    if (button) {
      button.innerHTML = pauseIcon;
      button.setAttribute("aria-label", "暂停");
    }
    const tick = () => {
      if (!state.playing) return;
      state.fraction += 0.00055;
      if (state.fraction >= 1) {
        state.fraction = 1;
        paintProgress(win, state.fraction);
        stopPlayback(win);
        return;
      }
      paintProgress(win, state.fraction);
      state.raf = requestAnimationFrame(tick);
    };
    state.raf = requestAnimationFrame(tick);
  }

  // ── Interactions ────────────────────────────────────────────

  function wireWindow(win) {
    win.querySelector(".play-btn")?.addEventListener("click", () => {
      const state = stateFor(win);
      if (state.playing) stopPlayback(win);
      else startPlayback(win);
    });

    win.querySelector(".timeline")?.addEventListener("click", (event) => {
      const rect = event.currentTarget.getBoundingClientRect();
      const fraction = (event.clientX - rect.left) / rect.width;
      const state = stateFor(win);
      state.fraction = Math.max(0, Math.min(1, fraction));
      paintProgress(win, state.fraction);
    });

    win.querySelector("[data-safe-toggle]")?.addEventListener("click", () => {
      win.classList.toggle("show-safe");
    });

    win.querySelectorAll("[data-project]").forEach((project) => {
      project.addEventListener("click", () => {
        win.querySelectorAll("[data-project]").forEach((p) => p.classList.remove("is-selected"));
        project.classList.add("is-selected");
      });
    });

    win.querySelectorAll(".pattern").forEach((pattern) => {
      pattern.addEventListener("click", () => {
        win.querySelectorAll(".pattern").forEach((p) => p.classList.remove("is-selected"));
        pattern.classList.add("is-selected");
      });
    });

    win.querySelectorAll(".tabs button").forEach((tab) => {
      tab.addEventListener("click", () => {
        win.querySelectorAll(".tabs button").forEach((t) => t.classList.remove("on"));
        tab.classList.add("on");
      });
    });

    win.querySelectorAll(".line-card").forEach((card) => {
      card.addEventListener("click", () => {
        win.querySelectorAll(".line-card").forEach((c) => c.classList.remove("is-selected"));
        card.classList.add("is-selected");
      });
    });

    win.querySelectorAll("[data-segment] button").forEach((button) => {
      button.addEventListener("click", () => {
        win.querySelectorAll("[data-segment] button").forEach((b) => b.classList.remove("on"));
        button.classList.add("on");
        win.dataset.orientation = button.textContent.trim();
      });
    });

    paintProgress(win, stateFor(win).fraction);
  }

  document.querySelectorAll(".app-window").forEach(wireWindow);

  // State boards are static; wire their own small controls.
  document.querySelectorAll(".picker-grid").forEach((grid) => {
    grid.querySelectorAll(".picker-chord").forEach((button) => {
      button.addEventListener("click", () => {
        grid.querySelectorAll(".picker-chord").forEach((b) => b.classList.remove("is-selected"));
        button.classList.add("is-selected");
        const label = (button.childNodes[0]?.nodeValue || "").trim();
        const info = grid.parentElement.querySelector(".picker-info b");
        if (info && label) info.textContent = `${label} · 已选`;
      });
    });
  });

  document.querySelectorAll(".export-grid").forEach((grid) => {
    grid.querySelectorAll(".export-option").forEach((option) => {
      option.addEventListener("click", () => {
        grid.querySelectorAll(".export-option").forEach((o) => o.classList.remove("is-selected"));
        option.classList.add("is-selected");
      });
    });
  });

  document.querySelectorAll("[data-default-safe]").forEach((frame) => {
    frame.classList.add("show-safe");
  });

  // ── Boot ────────────────────────────────────────────────────

  document.querySelectorAll("[data-wave]").forEach(fillWave);
  drawAllDiagrams();
  fitAll();

  const observer = new ResizeObserver(() => {
    fitAll();
    drawAllDiagrams();
  });
  document.querySelectorAll("[data-fit]").forEach((el) => observer.observe(el));

  window.addEventListener("load", () => {
    fitAll();
    drawAllDiagrams();
    const hash = location.hash.slice(1);
    if (hash) {
      const jump = () => document.getElementById(hash)?.scrollIntoView({ block: "start" });
      jump();
      requestAnimationFrame(jump);
      setTimeout(jump, 250);
    }
  });
})();
