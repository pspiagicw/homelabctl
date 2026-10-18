// Relative timestamps, node counts, the "synced" clock and the event history.
// Times tick every second; counts and the clock update after each htmx swap.
(function () {
	var STORE_KEY = "homelabctl.events";
	var MAX_EVENTS = 200;
	var events = [];

	function ago(ms) {
		var s = Math.max(0, Math.round(ms / 1000));
		if (s < 60) return s + "s ago";
		var m = Math.floor(s / 60);
		if (m < 60) return m + "m ago";
		var h = Math.floor(m / 60);
		if (h < 24) return h + "h " + (m % 60) + "m ago";
		return Math.floor(h / 24) + "d " + (h % 24) + "h ago";
	}

	// "2:33pm", with the date in front when it isn't today.
	function wallclock(d) {
		var h = d.getHours(), m = d.getMinutes();
		var clock = (h % 12 || 12) + ":" + (m < 10 ? "0" : "") + m + (h < 12 ? "am" : "pm");
		if (d.toDateString() === new Date().toDateString()) return clock;
		return d.toLocaleDateString([], { month: "short", day: "numeric" }) + " " + clock;
	}

	// Every <time datetime> shows "2:33pm (2h 5m ago)"; the title keeps the exact time.
	function tick() {
		var now = Date.now();
		document.querySelectorAll("time[datetime]").forEach(function (el) {
			var t = Date.parse(el.getAttribute("datetime"));
			if (!isNaN(t)) el.textContent = wallclock(new Date(t)) + " (" + ago(now - t) + ")";
		});
	}

	function set(id, value) {
		var el = document.getElementById(id);
		if (el) el.textContent = value;
	}

	function counts() {
		var on = 0, off = 0, busy = 0;
		document.querySelectorAll(".node[data-state]").forEach(function (el) {
			var s = el.getAttribute("data-state");
			if (s === "on") on++;
			else if (s === "booting" || s === "shutting") busy++;
			else off++;
		});
		set("count-on", on);
		set("count-off", off);
		set("count-busy", busy);
	}

	// Event history, kept in localStorage so it survives reloads on this device.

	function load() {
		try {
			var raw = localStorage.getItem(STORE_KEY);
			events = raw ? JSON.parse(raw) : [];
		} catch (e) {
			events = [];
		}
	}

	function save() {
		try {
			localStorage.setItem(STORE_KEY, JSON.stringify(events));
		} catch (e) {
			// Storage unavailable (private mode etc.): history lasts for this page only.
		}
	}

	function clock(ts) {
		var d = new Date(ts);
		return d.toLocaleDateString([], { month: "short", day: "numeric" }) + " " +
			d.toLocaleTimeString([], { hour12: false });
	}

	function renderEvents() {
		var log = document.getElementById("event-log");
		if (!log) return;
		log.replaceChildren();

		events.forEach(function (ev) {
			var li = document.createElement("li");
			li.className = "event " + (ev.ok ? "event-ok" : "event-error");

			var head = document.createElement("div");
			head.className = "event-head";

			var tag = document.createElement("span");
			tag.className = "event-tag";
			tag.textContent = (ev.ok ? "ok" : "err") + (ev.code ? " " + ev.code : "");

			var when = document.createElement("time");
			when.setAttribute("datetime", new Date(ev.ts).toISOString());
			when.title = clock(ev.ts);
			when.textContent = clock(ev.ts);

			head.append(tag, when);

			var msg = document.createElement("p");
			msg.className = "event-msg";
			msg.textContent = ev.message;

			li.append(head, msg);
			log.append(li);
		});

		set("event-count", events.length);
		var empty = document.getElementById("event-empty");
		if (empty) empty.hidden = events.length > 0;
		tick();
	}

	function record(ok, code, message) {
		events.unshift({ ts: Date.now(), ok: ok, code: code, message: message });
		events = events.slice(0, MAX_EVENTS);
		save();
		renderEvents();
	}

	// Action responses carry a hidden .snack; polls never do.
	function collect(xhr) {
		document.querySelectorAll(".snack:not([data-logged])").forEach(function (el) {
			el.setAttribute("data-logged", "");
			record(el.getAttribute("data-failed") !== "true", xhr ? xhr.status : 0, el.getAttribute("data-message"));
		});
	}

	function isAction(evt) {
		var cfg = evt.detail && evt.detail.requestConfig;
		return cfg && cfg.verb === "post";
	}

	document.addEventListener("DOMContentLoaded", function () {
		load();
		renderEvents();
		counts();
		set("updated", new Date().toLocaleTimeString([], { hour12: false }));

		var clear = document.getElementById("event-clear");
		if (clear) clear.addEventListener("click", function () {
			events = [];
			save();
			renderEvents();
		});
	});

	document.addEventListener("htmx:afterSettle", function (evt) {
		collect(evt.detail && evt.detail.xhr);
		counts();
		set("updated", new Date().toLocaleTimeString([], { hour12: false }));
		tick();
	});

	// The daemon didn't answer at all; only logged for actions, not the 5s poll.
	document.addEventListener("htmx:sendError", function (evt) {
		if (isAction(evt)) record(false, 0, "request failed: daemon unreachable (" + evt.detail.requestConfig.path + ")");
	});

	setInterval(tick, 1000);
})();
