package server

import "net/http"

func serveHistoryPage(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/" {
		http.NotFound(writer, request)
		return
	}
	setWebHeaders(writer)
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.Write([]byte(historyPage))
}

func serveHistoryCSS(writer http.ResponseWriter, _ *http.Request) {
	serveWebAsset(writer, "text/css; charset=utf-8", historyCSS)
}

func serveHistoryJavaScript(writer http.ResponseWriter, _ *http.Request) {
	serveWebAsset(writer, "text/javascript; charset=utf-8", historyJavaScript)
}

func serveWebAsset(writer http.ResponseWriter, contentType, content string) {
	setWebHeaders(writer)
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Content-Type", contentType)
	writer.Write([]byte(content))
}

func setWebHeaders(writer http.ResponseWriter) {
	writer.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self' data:; base-uri 'none'; frame-ancestors 'none'; form-action 'none'")
	writer.Header().Set("Referrer-Policy", "no-referrer")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.Header().Set("X-Frame-Options", "DENY")
}

const historyPage = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="theme-color" content="#16120f">
<title>Kuhhandel Archive</title>
<link rel="stylesheet" href="/app.css">
</head>
<body>
<div class="shell">
<aside class="archive">
<header class="brand">
<div class="brand-mark" aria-hidden="true">K</div>
<div>
<p class="eyebrow">Kuhhandel</p>
<h1>Game archive</h1>
</div>
</header>
<p class="archive-copy">Open any table and retrace each public move.</p>
<label class="search">
<span>Find a game or player</span>
<input id="search" type="search" placeholder="Search history" autocomplete="off">
</label>
<div id="game-list" class="game-list" aria-live="polite"></div>
<button id="load-more" class="load-more" type="button" hidden>Load older games</button>
<footer class="archive-footer">
<span class="live-dot" aria-hidden="true"></span>
<span>Server record</span>
<button id="refresh" class="text-button" type="button">Refresh</button>
</footer>
</aside>
<main class="replay">
<section id="welcome" class="welcome">
<div class="welcome-card">
<span class="welcome-animal" aria-hidden="true">♞</span>
<p class="eyebrow">Replay room</p>
<h2>Select a game</h2>
<p>Every frame comes from the server log. Private money and offers stay hidden.</p>
</div>
</section>
<section id="viewer" class="viewer" hidden>
<header class="viewer-header">
<div>
<p id="game-id" class="game-id"></p>
<h2 id="game-title">Game replay</h2>
</div>
<div class="header-facts">
<span id="status" class="status"></span>
<span id="step-count" class="step-count"></span>
</div>
</header>
<div class="stage">
<div class="event-line">
<div>
<p id="event-step" class="eyebrow"></p>
<h3 id="event-title"></h3>
</div>
<time id="event-time"></time>
</div>
<div class="table-wrap">
<div class="table-surface">
<div id="market" class="market"></div>
<div id="players" class="players"></div>
</div>
</div>
<div class="transport" role="group" aria-label="Replay controls">
<div class="transport-buttons">
<button id="first" type="button" aria-label="First move">|◀</button>
<button id="previous" type="button" aria-label="Previous move">◀</button>
<button id="play" class="play" type="button" aria-label="Play replay">▶</button>
<button id="next" type="button" aria-label="Next move">▶</button>
<button id="last" type="button" aria-label="Last move">▶|</button>
</div>
<label class="scrubber">
<span class="sr-only">Replay position</span>
<input id="timeline" type="range" min="0" max="0" value="0">
</label>
<label class="speed">
<span>Speed</span>
<select id="speed">
<option value="1600">0.5×</option>
<option value="900" selected>1×</option>
<option value="500">2×</option>
<option value="280">4×</option>
</select>
</label>
</div>
</div>
</section>
<section id="failure" class="failure" hidden>
<p class="eyebrow">Could not load</p>
<h2 id="failure-title">The game is unavailable.</h2>
<button id="retry" type="button">Try again</button>
</section>
</main>
</div>
<script src="/app.js"></script>
</body>
</html>`
