package server

const historyJavaScript = `const elements = {
  failure: document.getElementById("failure"),
  failureTitle: document.getElementById("failure-title"),
  first: document.getElementById("first"),
  gameID: document.getElementById("game-id"),
  gameList: document.getElementById("game-list"),
  gameTitle: document.getElementById("game-title"),
  last: document.getElementById("last"),
  loadMore: document.getElementById("load-more"),
  market: document.getElementById("market"),
  next: document.getElementById("next"),
  play: document.getElementById("play"),
  players: document.getElementById("players"),
  previous: document.getElementById("previous"),
  refresh: document.getElementById("refresh"),
  retry: document.getElementById("retry"),
  search: document.getElementById("search"),
  speed: document.getElementById("speed"),
  status: document.getElementById("status"),
  stepCount: document.getElementById("step-count"),
  eventStep: document.getElementById("event-step"),
  eventTime: document.getElementById("event-time"),
  eventTitle: document.getElementById("event-title"),
  timeline: document.getElementById("timeline"),
  viewer: document.getElementById("viewer"),
  welcome: document.getElementById("welcome")
}

const animalEmoji = {
  rooster: "🐓",
  goose: "🪿",
  cat: "🐈",
  dog: "🐕",
  sheep: "🐑",
  goat: "🐐",
  donkey: "🫏",
  pig: "🐖",
  cow: "🐄",
  horse: "🐎"
}

const animalOrder = Object.keys(animalEmoji)
const dateFormat = new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" })
let games = []
let hasMore = false
let offset = 0
let selectedID = ""
let frames = []
let frameIndex = 0
let timer = 0
let loadSequence = 0

function node(tag, className, text) {
  const element = document.createElement(tag)
  if (className) element.className = className
  if (text !== undefined) element.textContent = text
  return element
}

function show(panel) {
  elements.welcome.hidden = panel !== elements.welcome
  elements.viewer.hidden = panel !== elements.viewer
  elements.failure.hidden = panel !== elements.failure
}

async function fetchJSON(url) {
  const response = await fetch(url, { headers: { Accept: "application/json" } })
  if (!response.ok) {
    let message = "Request failed"
    try {
      const body = await response.json()
      if (body.error) message = body.error
    } catch (_) {
      message = response.statusText || message
    }
    throw new Error(message)
  }
  return response.json()
}

async function loadGames(reset) {
  if (reset) {
    offset = 0
    games = []
    elements.gameList.replaceChildren(node("p", "loading-list", "Loading game history…"))
  }
  elements.loadMore.disabled = true
  try {
    const result = await fetchJSON("/api/history?offset=" + offset)
    const known = new Set(games.map(game => game.id))
    for (const game of result.games) {
      if (!known.has(game.id)) games.push(game)
    }
    offset = games.length
    hasMore = result.hasMore
    renderGameList()
  } catch (error) {
    elements.gameList.replaceChildren(node("p", "empty-list", error.message))
  } finally {
    elements.loadMore.disabled = false
  }
}

function renderGameList() {
  const query = elements.search.value.trim().toLocaleLowerCase()
  const shown = games.filter(game => {
    const names = game.players.map(player => player.name).join(" ")
    return (game.id + " " + names).toLocaleLowerCase().includes(query)
  })
  const items = shown.map(game => {
    const button = node("button", "game-card" + (game.id === selectedID ? " active" : ""))
    button.type = "button"
    button.setAttribute("aria-pressed", String(game.id === selectedID))
    button.addEventListener("click", () => selectGame(game.id))
    const top = node("span", "game-card-top")
    const names = game.players.map(player => player.name).join(", ") || "Waiting for players"
    top.append(node("span", "game-card-title", names))
    top.append(node("span", "mini-status " + game.status, game.status))
    const bottom = node("span", "game-card-bottom")
    bottom.append(node("span", "", game.eventCount + (game.eventCount === 1 ? " move" : " moves")))
    bottom.append(node("time", "", dateFormat.format(new Date(game.updatedAt))))
    button.append(top, bottom)
    return button
  })
  if (items.length === 0) {
    const message = games.length === 0 ? "No games have been recorded yet." : "No loaded games match your search."
    elements.gameList.replaceChildren(node("p", "empty-list", message))
  } else {
    elements.gameList.replaceChildren(...items)
  }
  elements.loadMore.hidden = !hasMore
}

async function selectGame(gameID) {
  stopPlayback()
  selectedID = gameID
  renderGameList()
  const sequence = ++loadSequence
  show(elements.welcome)
  try {
    const result = await fetchJSON("/api/history/" + encodeURIComponent(gameID))
    if (sequence !== loadSequence) return
    frames = result.frames
    if (frames.length === 0) throw new Error("The game has no recorded moves")
    frameIndex = 0
    elements.timeline.max = String(frames.length - 1)
    elements.timeline.value = "0"
    history.replaceState(null, "", "#" + encodeURIComponent(gameID))
    show(elements.viewer)
    renderFrame()
  } catch (error) {
    if (sequence !== loadSequence) return
    elements.failureTitle.textContent = error.message
    show(elements.failure)
  }
}

function playerName(publicView, playerID) {
  return publicView.players.find(player => player.id === playerID)?.name || "Unknown player"
}

function titleFor(frame) {
  const publicView = frame.public
  const actor = playerName(publicView, frame.actorId)
  const auction = publicView.auction
  const trade = publicView.trade
  switch (frame.type) {
    case "room.created": return actor + " opened the table"
    case "player.joined": return actor + " joined the table"
    case "game.started": return actor + " started the game"
    case "auction.started": return actor + " put up " + (auction?.animal || "an animal")
    case "auction.bid_placed": return actor + " bid " + (auction?.highestBid || 0)
    case "auction.closed": return actor + " closed the auction"
    case "auction.resolved": return actor + " settled the auction"
    case "trade.started": return actor + " challenged " + playerName(publicView, trade?.targetId)
    case "trade.accepted": return actor + " accepted the trade"
    case "trade.countered": return actor + " answered the trade"
    case "trade.reoffered": return actor + " made a second offer"
    default: return frame.type.replaceAll(".", " ")
  }
}

function renderMarket(publicView) {
  const fragment = document.createDocumentFragment()
  fragment.append(node("span", "market-phase", publicView.phase.replaceAll("_", " ")))
  let symbol = "▧"
  let title = "Choose a move"
  let detail = publicView.turnPlayerId ? playerName(publicView, publicView.turnPlayerId) + " has the turn" : "The table is ready"
  if (publicView.auction) {
    const auction = publicView.auction
    symbol = animalEmoji[auction.animal] || "?"
    title = auction.animal
    detail = auction.highestBid > 0
      ? playerName(publicView, auction.highestBidderId) + " leads at " + auction.highestBid
      : playerName(publicView, auction.auctioneerId) + " is taking bids"
  } else if (publicView.trade) {
    const trade = publicView.trade
    symbol = animalEmoji[trade.animal] || "?"
    title = trade.animal + " trade"
    detail = playerName(publicView, trade.challengerId) + " ↔ " + playerName(publicView, trade.targetId) + " · " + trade.cardCount + (trade.cardCount === 1 ? " card" : " cards")
  } else if (publicView.status === "lobby") {
    symbol = "♟"
    title = "Lobby"
    detail = publicView.players.length + (publicView.players.length === 1 ? " player seated" : " players seated")
  } else if (publicView.status === "finished") {
    symbol = "✓"
    title = "Game complete"
    detail = (publicView.winnerIds || []).map(id => playerName(publicView, id)).join(" & ") + " won"
  } else {
    detail += " · " + publicView.deckRemaining + " cards in deck"
  }
  fragment.append(node("span", "market-animal", symbol))
  fragment.append(node("h4", "", title))
  fragment.append(node("p", "market-detail", detail))
  elements.market.replaceChildren(fragment)
}

function renderPlayers(publicView) {
  const cards = publicView.players.map(player => {
    const active = player.id === publicView.turnPlayerId
    const winner = (publicView.winnerIds || []).includes(player.id)
    const card = node("article", "player-card" + (active ? " active" : "") + (winner ? " winner" : ""))
    const head = node("div", "player-head")
    head.append(node("span", "player-name", player.name), node("span", "seat", "Seat " + (player.seat + 1)))
    const stats = node("div", "player-stats")
    if (player.id === publicView.hostId) stats.append(node("span", "role host", "Host"))
    if (active) stats.append(node("span", "role", "Turn"))
    if (winner) stats.append(node("span", "role winner", "Winner"))
    stats.append(node("span", "score", player.score + " pts"))
    const animals = node("div", "animal-row")
    for (const animal of animalOrder) {
      const count = player.animals[animal] || 0
      if (count > 0) animals.append(node("span", "animal-chip", animalEmoji[animal] + " ×" + count))
    }
    if (animals.childElementCount === 0) animals.append(node("span", "no-animals", "No animals yet"))
    card.append(head, stats, animals)
    return card
  })
  elements.players.replaceChildren(...cards)
}

function renderFrame() {
  const frame = frames[frameIndex]
  const publicView = frame.public
  const names = publicView.players.map(player => player.name).join(" · ")
  elements.gameID.textContent = publicView.gameId
  elements.gameTitle.textContent = names || "Game replay"
  elements.status.textContent = publicView.status
  elements.status.className = "status " + publicView.status
  elements.stepCount.textContent = frameIndex + 1 + " / " + frames.length
  elements.eventStep.textContent = "Move " + (frameIndex + 1) + " · " + frame.type.replaceAll(".", " ")
  elements.eventTitle.textContent = titleFor(frame)
  elements.eventTime.textContent = dateFormat.format(new Date(frame.occurredAt))
  elements.eventTime.dateTime = frame.occurredAt
  elements.timeline.value = String(frameIndex)
  elements.first.disabled = frameIndex === 0
  elements.previous.disabled = frameIndex === 0
  elements.next.disabled = frameIndex === frames.length - 1
  elements.last.disabled = frameIndex === frames.length - 1
  renderMarket(publicView)
  renderPlayers(publicView)
}

function moveTo(index) {
  if (frames.length === 0) return
  frameIndex = Math.max(0, Math.min(frames.length - 1, index))
  renderFrame()
}

function stopPlayback() {
  if (timer) window.clearInterval(timer)
  timer = 0
  elements.play.textContent = "▶"
  elements.play.setAttribute("aria-label", "Play replay")
}

function startPlayback() {
  if (frames.length < 2) return
  if (frameIndex === frames.length - 1) moveTo(0)
  elements.play.textContent = "Ⅱ"
  elements.play.setAttribute("aria-label", "Pause replay")
  timer = window.setInterval(() => {
    if (frameIndex === frames.length - 1) {
      stopPlayback()
      return
    }
    moveTo(frameIndex + 1)
  }, Number(elements.speed.value))
}

function togglePlayback() {
  if (timer) stopPlayback()
  else startPlayback()
}

function hashGameID() {
  try {
    return decodeURIComponent(location.hash.slice(1))
  } catch (_) {
    return ""
  }
}

elements.search.addEventListener("input", renderGameList)
elements.loadMore.addEventListener("click", () => loadGames(false))
elements.refresh.addEventListener("click", () => loadGames(true))
elements.retry.addEventListener("click", () => selectedID && selectGame(selectedID))
elements.first.addEventListener("click", () => { stopPlayback(); moveTo(0) })
elements.previous.addEventListener("click", () => { stopPlayback(); moveTo(frameIndex - 1) })
elements.play.addEventListener("click", togglePlayback)
elements.next.addEventListener("click", () => { stopPlayback(); moveTo(frameIndex + 1) })
elements.last.addEventListener("click", () => { stopPlayback(); moveTo(frames.length - 1) })
elements.timeline.addEventListener("input", () => { stopPlayback(); moveTo(Number(elements.timeline.value)) })
elements.speed.addEventListener("change", () => {
  if (timer) {
    stopPlayback()
    startPlayback()
  }
})
window.addEventListener("hashchange", () => {
  const gameID = hashGameID()
  if (gameID && gameID !== selectedID) selectGame(gameID)
})
window.addEventListener("keydown", event => {
  if (event.target instanceof HTMLInputElement || event.target instanceof HTMLSelectElement) return
  if (event.key === "ArrowLeft") {
    event.preventDefault()
    stopPlayback()
    moveTo(frameIndex - 1)
  }
  if (event.key === "ArrowRight") {
    event.preventDefault()
    stopPlayback()
    moveTo(frameIndex + 1)
  }
  if (event.key === " ") {
    event.preventDefault()
    togglePlayback()
  }
})

async function start() {
  await loadGames(true)
  const requested = hashGameID()
  if (requested) await selectGame(requested)
  else if (games.length > 0) await selectGame(games[0].id)
}

start()`
