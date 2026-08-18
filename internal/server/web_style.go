package server

const historyCSS = `:root {
  color-scheme: dark;
  --ink: #f4ead8;
  --muted: #a69b8d;
  --paper: #16120f;
  --panel: #201a16;
  --panel-2: #29211b;
  --line: #3c3128;
  --cream: #ead8b8;
  --rust: #bb583b;
  --rust-dark: #713627;
  --green: #506c52;
  --gold: #c99a45;
  --shadow: rgba(0, 0, 0, 0.35);
  font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
}

* {
  box-sizing: border-box;
}

html,
body {
  min-height: 100%;
  margin: 0;
  background: var(--paper);
  color: var(--ink);
}

body {
  min-width: 320px;
}

button,
input,
select {
  color: inherit;
  font: inherit;
}

button,
select {
  cursor: pointer;
}

button:focus-visible,
input:focus-visible,
select:focus-visible {
  outline: 2px solid var(--gold);
  outline-offset: 2px;
}

.shell {
  display: grid;
  grid-template-columns: 340px minmax(0, 1fr);
  min-height: 100vh;
}

.archive {
  position: sticky;
  top: 0;
  display: flex;
  flex-direction: column;
  height: 100vh;
  padding: 30px 22px 18px;
  overflow: hidden;
  background: #120f0c;
  border-right: 1px solid var(--line);
}

.brand {
  display: flex;
  align-items: center;
  gap: 13px;
}

.brand-mark {
  display: grid;
  width: 48px;
  height: 48px;
  place-items: center;
  color: #1b1510;
  background: var(--cream);
  border: 2px solid #fff3dc;
  border-radius: 50%;
  box-shadow: 0 5px 0 #6e5e4d;
  font-family: Georgia, "Times New Roman", serif;
  font-size: 28px;
  font-weight: 800;
}

.brand h1,
.viewer h2,
.welcome h2,
.failure h2,
.event-line h3 {
  margin: 0;
  font-family: Georgia, "Times New Roman", serif;
  font-weight: 600;
  letter-spacing: -0.025em;
}

.brand h1 {
  font-size: 25px;
}

.eyebrow {
  margin: 0 0 4px;
  color: var(--gold);
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.16em;
  text-transform: uppercase;
}

.archive-copy {
  max-width: 245px;
  margin: 24px 0 20px;
  color: var(--muted);
  font-size: 14px;
  line-height: 1.5;
}

.search {
  display: block;
}

.search span,
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}

.search input {
  width: 100%;
  padding: 12px 14px;
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: 9px;
}

.search input::placeholder {
  color: #746b61;
}

.game-list {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 8px;
  min-height: 0;
  margin-top: 16px;
  padding-right: 4px;
  overflow-y: auto;
  scrollbar-color: var(--line) transparent;
}

.game-card {
  width: 100%;
  padding: 14px;
  text-align: left;
  background: transparent;
  border: 1px solid transparent;
  border-radius: 10px;
  transition: background 150ms ease, border-color 150ms ease, transform 150ms ease;
}

.game-card:hover {
  background: var(--panel);
  border-color: var(--line);
  transform: translateX(2px);
}

.game-card.active {
  background: #2d221b;
  border-color: var(--rust);
}

.game-card-top,
.game-card-bottom,
.header-facts,
.archive-footer {
  display: flex;
  align-items: center;
}

.game-card-top,
.game-card-bottom {
  justify-content: space-between;
  gap: 10px;
}

.game-card-title {
  overflow: hidden;
  font-size: 14px;
  font-weight: 750;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.game-card-bottom {
  margin-top: 8px;
  color: var(--muted);
  font-size: 11px;
}

.mini-status {
  flex: none;
  padding: 3px 7px;
  color: var(--cream);
  background: var(--panel-2);
  border-radius: 99px;
  font-size: 9px;
  font-weight: 800;
  letter-spacing: 0.09em;
  text-transform: uppercase;
}

.mini-status.finished {
  color: #d9efd8;
  background: #2a452d;
}

.mini-status.playing {
  color: #ffe2a7;
  background: #4c3820;
}

.load-more {
  margin-top: 12px;
  padding: 10px;
  color: var(--cream);
  background: transparent;
  border: 1px solid var(--line);
  border-radius: 8px;
}

.archive-footer {
  gap: 8px;
  margin-top: 16px;
  padding-top: 16px;
  color: var(--muted);
  border-top: 1px solid var(--line);
  font-size: 11px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.live-dot {
  width: 7px;
  height: 7px;
  background: #76a878;
  border-radius: 50%;
  box-shadow: 0 0 0 4px rgba(118, 168, 120, 0.12);
}

.text-button {
  margin-left: auto;
  padding: 0;
  color: var(--gold);
  background: none;
  border: 0;
  text-transform: uppercase;
}

.replay {
  position: relative;
  min-width: 0;
  min-height: 100vh;
  background-color: #1a1511;
  background-image: linear-gradient(rgba(255, 255, 255, 0.018) 1px, transparent 1px), linear-gradient(90deg, rgba(255, 255, 255, 0.018) 1px, transparent 1px);
  background-size: 32px 32px;
}

.welcome,
.failure {
  display: grid;
  min-height: 100vh;
  padding: 32px;
  place-items: center;
}

.welcome-card,
.failure {
  text-align: center;
}

.welcome-card {
  max-width: 440px;
  padding: 54px 48px;
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: 18px;
  box-shadow: 0 24px 70px var(--shadow);
}

.welcome-animal {
  display: block;
  margin-bottom: 22px;
  color: var(--cream);
  font-size: 58px;
}

.welcome h2,
.failure h2 {
  font-size: clamp(34px, 5vw, 54px);
}

.welcome-card > p:last-child,
.failure p {
  color: var(--muted);
  line-height: 1.6;
}

.viewer {
  min-height: 100vh;
}

.viewer-header {
  display: flex;
  min-height: 104px;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  padding: 24px clamp(22px, 4vw, 58px);
  background: rgba(22, 18, 15, 0.9);
  border-bottom: 1px solid var(--line);
}

.game-id {
  margin: 0 0 4px;
  color: var(--muted);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 11px;
}

.viewer h2 {
  font-size: 30px;
}

.header-facts {
  gap: 10px;
}

.status,
.step-count {
  padding: 8px 11px;
  border: 1px solid var(--line);
  border-radius: 99px;
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.status {
  color: #f6d591;
  background: #46331f;
}

.status.finished {
  color: #d6edda;
  background: #28422c;
}

.step-count {
  color: var(--muted);
}

.stage {
  max-width: 1260px;
  margin: 0 auto;
  padding: 32px clamp(18px, 4vw, 58px) 42px;
}

.event-line {
  display: flex;
  min-height: 64px;
  align-items: flex-end;
  justify-content: space-between;
  gap: 24px;
  margin-bottom: 22px;
}

.event-line h3 {
  font-size: clamp(25px, 3vw, 38px);
}

.event-line time {
  flex: none;
  padding-bottom: 5px;
  color: var(--muted);
  font-size: 12px;
}

.table-wrap {
  padding: 12px;
  background: #271b14;
  border: 1px solid #65452f;
  border-radius: 34px;
  box-shadow: 0 28px 70px var(--shadow), inset 0 0 0 3px rgba(234, 216, 184, 0.05);
}

.table-surface {
  position: relative;
  display: grid;
  min-height: 500px;
  padding: 34px;
  place-items: center;
  overflow: hidden;
  background-color: #314b38;
  background-image: radial-gradient(rgba(255, 255, 255, 0.045) 1px, transparent 1px);
  background-size: 11px 11px;
  border: 1px solid #59705d;
  border-radius: 24px;
}

.market {
  position: relative;
  z-index: 2;
  display: grid;
  width: min(280px, 34%);
  min-width: 220px;
  min-height: 210px;
  padding: 20px;
  place-items: center;
  text-align: center;
  background: #e8d6b7;
  border: 1px solid #fff1d7;
  border-radius: 14px;
  box-shadow: 0 12px 0 rgba(41, 27, 18, 0.38), 0 22px 42px rgba(0, 0, 0, 0.3);
  color: #241a14;
  transform: rotate(-1deg);
}

.market-phase {
  color: #6f3f28;
  font-size: 10px;
  font-weight: 900;
  letter-spacing: 0.14em;
  text-transform: uppercase;
}

.market-animal {
  display: block;
  margin: 8px 0 3px;
  font-size: 64px;
  line-height: 1;
}

.market h4 {
  margin: 0;
  font-family: Georgia, "Times New Roman", serif;
  font-size: 25px;
  text-transform: capitalize;
}

.market-detail {
  margin: 8px 0 0;
  color: #655143;
  font-size: 12px;
  line-height: 1.4;
}

.players {
  position: absolute;
  inset: 22px;
  z-index: 1;
}

.player-card {
  position: absolute;
  width: min(230px, 25%);
  min-width: 175px;
  padding: 15px;
  background: #1b1815;
  border: 1px solid #62574d;
  border-radius: 13px;
  box-shadow: 0 12px 26px rgba(0, 0, 0, 0.28);
  transition: border-color 150ms ease, box-shadow 150ms ease;
}

.player-card:nth-child(1) {
  top: 0;
  left: 50%;
  transform: translateX(-50%);
}

.player-card:nth-child(2) {
  top: 50%;
  right: 0;
  transform: translateY(-50%);
}

.player-card:nth-child(3) {
  right: 12%;
  bottom: 0;
}

.player-card:nth-child(4) {
  bottom: 0;
  left: 12%;
}

.player-card:nth-child(5) {
  top: 50%;
  left: 0;
  transform: translateY(-50%);
}

.player-card.active {
  border-color: #e4bd63;
  box-shadow: 0 0 0 3px rgba(228, 189, 99, 0.18), 0 12px 26px rgba(0, 0, 0, 0.3);
}

.player-card.winner {
  border-color: #8cc18e;
}

.player-head,
.player-stats,
.animal-row {
  display: flex;
  align-items: center;
}

.player-head {
  justify-content: space-between;
  gap: 10px;
}

.player-name {
  overflow: hidden;
  font-size: 14px;
  font-weight: 800;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.seat {
  flex: none;
  color: var(--muted);
  font-size: 10px;
}

.player-stats {
  gap: 6px;
  min-height: 19px;
  margin-top: 8px;
}

.role {
  padding: 2px 6px;
  color: #e9c16b;
  background: #49371f;
  border-radius: 99px;
  font-size: 8px;
  font-weight: 900;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.role.host {
  color: #d8cbc0;
  background: #3e352e;
}

.role.winner {
  color: #d6f2d7;
  background: #315035;
}

.score {
  margin-left: auto;
  color: var(--cream);
  font-size: 10px;
}

.animal-row {
  flex-wrap: wrap;
  gap: 5px;
  min-height: 27px;
  margin-top: 10px;
}

.animal-chip {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  padding: 3px 6px;
  color: #d8cec4;
  background: #2d2824;
  border: 1px solid #433b35;
  border-radius: 6px;
  font-size: 12px;
}

.no-animals {
  color: #a09589;
  font-size: 10px;
  font-style: italic;
}

.transport {
  display: grid;
  grid-template-columns: auto minmax(120px, 1fr) auto;
  align-items: center;
  gap: 22px;
  margin-top: 24px;
  padding: 15px 18px;
  background: rgba(32, 26, 22, 0.92);
  border: 1px solid var(--line);
  border-radius: 13px;
}

.transport-buttons {
  display: flex;
  align-items: center;
  gap: 5px;
}

.transport button {
  display: grid;
  width: 34px;
  height: 34px;
  padding: 0;
  place-items: center;
  color: var(--cream);
  background: transparent;
  border: 1px solid transparent;
  border-radius: 50%;
  font-size: 11px;
}

.transport button:hover:not(:disabled) {
  background: var(--panel-2);
  border-color: var(--line);
}

.transport button.play {
  width: 42px;
  height: 42px;
  color: #211710;
  background: var(--cream);
  font-size: 14px;
}

.transport button:disabled {
  cursor: default;
  opacity: 0.3;
}

.scrubber input {
  width: 100%;
  accent-color: var(--rust);
}

.speed {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--muted);
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.speed select {
  padding: 7px 25px 7px 9px;
  background: var(--panel-2);
  border: 1px solid var(--line);
  border-radius: 7px;
}

.failure button {
  padding: 10px 16px;
  color: #211710;
  background: var(--cream);
  border: 0;
  border-radius: 8px;
}

.empty-list,
.loading-list {
  padding: 28px 14px;
  color: var(--muted);
  text-align: center;
  font-size: 13px;
  line-height: 1.5;
}

[hidden] {
  display: none !important;
}

@media (max-width: 980px) {
  .shell {
    grid-template-columns: 280px minmax(0, 1fr);
  }

  .archive {
    padding-inline: 16px;
  }

  .table-surface {
    min-height: 620px;
  }

  .market {
    width: 210px;
    min-width: 0;
  }

  .player-card {
    width: 190px;
  }

  .player-card:nth-child(3) {
    right: 4%;
  }

  .player-card:nth-child(4) {
    left: 4%;
  }
}

@media (max-width: 760px) {
  .shell {
    display: block;
  }

  .archive {
    position: relative;
    width: 100%;
    height: auto;
    max-height: none;
    border-right: 0;
    border-bottom: 1px solid var(--line);
  }

  .archive-copy {
    margin-block: 14px;
  }

  .game-list {
    max-height: 230px;
  }

  .archive-footer {
    display: none;
  }

  .welcome,
  .failure {
    min-height: 520px;
  }

  .viewer-header,
  .event-line {
    align-items: flex-start;
    flex-direction: column;
  }

  .event-line {
    gap: 7px;
  }

  .table-surface {
    display: flex;
    min-height: 0;
    flex-direction: column;
    gap: 14px;
    padding: 18px;
  }

  .market {
    order: 0;
    width: 100%;
    max-width: 310px;
    min-height: 180px;
  }

  .players {
    position: static;
    display: grid;
    width: 100%;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 9px;
    order: 1;
  }

  .player-card,
  .player-card:nth-child(n) {
    position: static;
    width: auto;
    min-width: 0;
    transform: none;
  }

  .transport {
    grid-template-columns: 1fr;
    gap: 12px;
  }

  .transport-buttons {
    justify-content: center;
  }

  .speed {
    justify-content: center;
  }
}

@media (prefers-reduced-motion: reduce) {
  *,
  *::before,
  *::after {
    scroll-behavior: auto !important;
    transition-duration: 0.01ms !important;
  }
}`
