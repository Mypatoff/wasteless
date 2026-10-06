// Shared helpers loaded on every page. No framework, just plain DOM APIs.
// Money is always a whole integer (so'm) — formatMoney/parseMoney only
// ever do integer arithmetic, never parseFloat/Number on a money value.

const NBSP = " ";

// apiFetch wraps fetch(): JSON in, JSON out, and throws an Error carrying
// the server's message on any non-2xx response. There's no login here, so
// a 401 (if it ever happened) is just another error for the caller to
// show — never a redirect.
async function apiFetch(path, options = {}) {
	const opts = { ...options, headers: { ...(options.headers || {}) } };
	if (opts.body !== undefined) {
		opts.headers["Content-Type"] = "application/json";
	}

	let res;
	try {
		res = await fetch(path, opts);
	} catch {
		throw new Error("Network error — is the server running?");
	}

	const text = await res.text();
	let data = null;
	if (text) {
		try {
			data = JSON.parse(text);
		} catch {
			data = null;
		}
	}

	if (!res.ok) {
		const msg = data && typeof data.error === "string" ? data.error : `Request failed (${res.status})`;
		throw new Error(msg);
	}
	return data;
}

// formatMoney renders a whole so'm amount as "150 000 so'm", grouping
// thousands with a non-breaking space using only integer/string work.
function formatMoney(amount) {
	const n = Math.trunc(Math.abs(amount));
	const digits = String(n);
	let grouped = "";
	for (let i = 0; i < digits.length; i++) {
		if (i > 0 && (digits.length - i) % 3 === 0) {
			grouped += NBSP;
		}
		grouped += digits[i];
	}
	const sign = amount < 0 ? "-" : "";
	return sign + grouped + NBSP + "so'm";
}

// parseMoney turns user input into a whole so'm integer, or throws an
// Error with a message fit to show inline next to the field.
function parseMoney(input) {
	const raw = String(input).trim();
	if (raw.includes(".") || raw.includes(",")) {
		throw new Error("Enter whole so'm without decimals, e.g. 150000");
	}
	const stripped = raw.replace(/[  ]/g, "");
	if (!/^\d+$/.test(stripped)) {
		throw new Error("Enter an amount in whole so'm, e.g. 150000");
	}
	const value = parseInt(stripped, 10);
	if (value < 1) {
		throw new Error("Amount must be greater than 0");
	}
	if (value > 100000000000) {
		throw new Error("Amount is too large");
	}
	return value;
}

// formatPermille renders an integer permille (0-1000) as "12.5%", using
// only integer division and remainder.
function formatPermille(permille) {
	const whole = Math.trunc(permille / 10);
	const tenth = Math.abs(permille % 10);
	return `${whole}.${tenth}%`;
}

// todayLocal returns "YYYY-MM-DD" built from local date parts —
// deliberately not toISOString(), which reports UTC and can be a day off.
function todayLocal() {
	const d = new Date();
	const y = d.getFullYear();
	const m = String(d.getMonth() + 1).padStart(2, "0");
	const day = String(d.getDate()).padStart(2, "0");
	return `${y}-${m}-${day}`;
}

function currentMonth() {
	return todayLocal().slice(0, 7);
}

function splitMonth(month) {
	const [y, m] = month.split("-").map(Number);
	return { y, m };
}

function joinMonth(y, m) {
	return `${y}-${String(m).padStart(2, "0")}`;
}

function prevMonth(month) {
	let { y, m } = splitMonth(month);
	m -= 1;
	if (m < 1) {
		m = 12;
		y -= 1;
	}
	return joinMonth(y, m);
}

function nextMonth(month) {
	let { y, m } = splitMonth(month);
	m += 1;
	if (m > 12) {
		m = 1;
		y += 1;
	}
	return joinMonth(y, m);
}

const MONTH_NAMES = [
	"January", "February", "March", "April", "May", "June",
	"July", "August", "September", "October", "November", "December",
];

function monthLabel(month) {
	const { y, m } = splitMonth(month);
	return `${MONTH_NAMES[m - 1]} ${y}`;
}

const MONTH_ABBR = [
	"Jan", "Feb", "Mar", "Apr", "May", "Jun",
	"Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
];

function monthAbbr(month) {
	const { m } = splitMonth(month);
	return MONTH_ABBR[m - 1];
}

// Reads the month from ?month=, falling back to (and correcting the URL
// to) the current month if it's missing or malformed.
function monthFromURL() {
	const params = new URLSearchParams(location.search);
	const m = params.get("month");
	if (m && /^\d{4}-(0[1-9]|1[0-2])$/.test(m)) {
		return m;
	}
	return currentMonth();
}

function setURLMonth(month) {
	const url = new URL(location.href);
	url.searchParams.set("month", month);
	location.href = url.toString();
}

// Global, visible error bar — the only place API errors are surfaced.
function showError(message) {
	const bar = document.getElementById("error-bar");
	const text = document.getElementById("error-text");
	if (!bar || !text) return;
	text.textContent = message;
	bar.hidden = false;
}

function hideError() {
	const bar = document.getElementById("error-bar");
	if (bar) bar.hidden = true;
}

document.addEventListener("DOMContentLoaded", () => {
	const closeBtn = document.getElementById("error-close");
	if (closeBtn) closeBtn.addEventListener("click", hideError);
});
