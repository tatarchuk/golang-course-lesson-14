// The live chat of the albums web UI (Lesson 14). One WebSocket to /ws/chat?room=<room>;
// the session cookie authenticates it, so no token appears in the URL. Everything the
// server sends is shown with textContent, never parsed as HTML. The sender's own message
// is shown when the server echoes it, so it appears exactly once. A lost connection is
// retried after 1, 2, 4 ... up to 30 seconds; the history frame of the new connection
// replaces the list, so nothing said in between is missed.
(function () {
  "use strict";

  var root = document.getElementById("chat");
  if (!root) { return; }
  var room = root.dataset.room;
  var log = document.getElementById("chat-log");
  var status = document.getElementById("chat-status");
  var form = document.getElementById("chat-form");
  var input = document.getElementById("chat-text");
  var ws = null;
  var retry = 0;

  function clock(at) {
    var d = new Date(at);
    return isNaN(d.getTime()) ? "" : d.toLocaleTimeString();
  }

  function line(m) {
    var li = document.createElement("li");
    li.className = "chat-" + m.type;
    var when = document.createElement("time");
    when.textContent = clock(m.at);
    li.appendChild(when);
    var body = document.createElement("span");
    if (m.type === "message") {
      var who = document.createElement("strong");
      who.textContent = m.author;
      body.appendChild(who);
      body.appendChild(document.createTextNode(": " + m.text));
    } else {
      body.textContent = m.text;
    }
    li.appendChild(body);
    return li;
  }

  function show(m) {
    if (m.type === "history") {
      log.replaceChildren();
      (m.history || []).forEach(function (h) { log.appendChild(line(h)); });
    } else {
      log.appendChild(line(m));
    }
    if (m.type === "presence") {
      status.textContent = "Connected to #" + room + ", " + m.online + " online";
    }
    log.scrollTop = log.scrollHeight;
  }

  function connect() {
    var scheme = location.protocol === "https:" ? "wss://" : "ws://";
    ws = new WebSocket(scheme + location.host + "/ws/chat?room=" + encodeURIComponent(room));
    status.textContent = "Connecting...";
    ws.onopen = function () {
      retry = 0;
      status.textContent = "Connected to #" + room;
    };
    ws.onmessage = function (e) { show(JSON.parse(e.data)); };
    ws.onclose = function (e) {
      var wait = Math.min(30, Math.pow(2, retry));
      retry++;
      status.textContent = "Disconnected (" + e.code + (e.reason ? " " + e.reason : "") +
        "), reconnecting in " + wait + " s";
      setTimeout(connect, wait * 1000);
    };
  }

  form.addEventListener("submit", function (e) {
    e.preventDefault();
    if (ws && ws.readyState === WebSocket.OPEN && input.value.trim() !== "") {
      ws.send(JSON.stringify({ text: input.value }));
      input.value = "";
    }
  });

  connect();
})();
