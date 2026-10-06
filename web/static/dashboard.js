// Dashboard page: month summary, wasted-by-category bars, saved-vs-wasted
// chart. Relies on the shared helpers in app.js.

const SVGNS = "http://www.w3.org/2000/svg";

function svgEl(name, attrs = {}) {
	const e = document.createElementNS(SVGNS, name);
	for (const key in attrs) {
		e.setAttribute(key, attrs[key]);
	}
	return e;
}

let state = {
	month: monthFromURL(),
	chartMonths: 6,
};

function setupMonthNav() {
	document.getElementById("month-label").textContent = monthLabel(state.month);
	document.getElementById("month-prev").addEventListener("click", () => {
		setURLMonth(prevMonth(state.month));
	});
	const nextBtn = document.getElementById("month-next");
	if (state.month >= currentMonth()) {
		nextBtn.disabled = true;
	} else {
		nextBtn.addEventListener("click", () => setURLMonth(nextMonth(state.month)));
	}
}

function renderStats(summary) {
	document.getElementById("stat-saved").textContent = formatMoney(summary.saved);
	document.getElementById("stat-spent").textContent = formatMoney(summary.spent);
	document.getElementById("stat-wasted").textContent = formatMoney(summary.wasted);
	document.getElementById("stat-wasted-share").textContent = formatPermille(summary.wasted_permille);
	document.getElementById("stat-total-saved").textContent = formatMoney(summary.total_saved_all_time);
}

function renderCategoryBars(byCategory) {
	const container = document.getElementById("category-bars");
	container.textContent = "";

	if (!byCategory || byCategory.length === 0) {
		const empty = document.createElement("p");
		empty.className = "empty-state";
		empty.textContent = "No spending recorded this month.";
		container.append(empty);
		return;
	}

	const maxSpent = Math.max(...byCategory.map((c) => c.spent), 1);
	for (const c of byCategory) {
		const row = document.createElement("div");
		row.className = "cat-row";

		const name = document.createElement("div");
		name.className = "cat-name";
		name.textContent = c.category;

		const track = document.createElement("div");
		track.className = "cat-track";
		const fillSpent = document.createElement("div");
		fillSpent.className = "cat-fill-spent";
		fillSpent.style.width = `${(c.spent / maxSpent) * 100}%`;
		const fillWasted = document.createElement("div");
		fillWasted.className = "cat-fill-wasted";
		fillWasted.style.width = `${(c.wasted / maxSpent) * 100}%`;
		track.append(fillSpent, fillWasted);

		const amounts = document.createElement("div");
		amounts.className = "cat-amounts";
		const wastedSpan = document.createElement("span");
		wastedSpan.className = "wasted";
		wastedSpan.textContent = formatMoney(c.wasted);
		const sep = document.createTextNode(" / ");
		const spentSpan = document.createElement("span");
		spentSpan.className = "spent";
		spentSpan.textContent = formatMoney(c.spent);
		amounts.append(wastedSpan, sep, spentSpan);

		row.append(name, track, amounts);
		container.append(row);
	}
}

// niceCeil rounds value up to the nearest "nice" number (1, 2, or 5 times
// a power of ten), using only integer multiplication and comparison.
function niceCeil(value) {
	if (value <= 0) return 1;
	let mag = 1;
	while (mag * 10 <= value) mag *= 10;
	for (const step of [1, 2, 5, 10]) {
		if (mag * step >= value) return mag * step;
	}
	return mag * 10;
}

// formatTick renders an axis value compactly: plain below 1,000, "50k" /
// "12.5k" up to 999,999, "1.2M" above — all with integer division/modulo.
function formatTick(n) {
	if (n < 1000) {
		return String(n);
	}
	if (n < 1000000) {
		const whole = Math.floor(n / 1000);
		const tenth = Math.floor((n % 1000) / 100);
		return tenth === 0 ? `${whole}k` : `${whole}.${tenth}k`;
	}
	const whole = Math.floor(n / 1000000);
	const tenth = Math.floor((n % 1000000) / 100000);
	return tenth === 0 ? `${whole}M` : `${whole}.${tenth}M`;
}

function renderChart(points) {
	const container = document.getElementById("chart-container");
	container.textContent = "";

	const hasData = points.some((p) => p.saved > 0 || p.wasted > 0);
	if (!hasData) {
		const empty = document.createElement("div");
		empty.className = "empty-state";
		const msg = document.createElement("p");
		msg.textContent = "No data yet";
		const btn = document.createElement("a");
		btn.className = "btn primary";
		btn.href = "/entries?add=1";
		btn.textContent = "Add your first entry";
		empty.append(msg, btn);
		container.append(empty);
		return;
	}

	const top = niceCeil(Math.max(...points.map((p) => Math.max(p.saved, p.wasted))));
	const ticks = [0, Math.floor(top / 2), top];
	const tickLabels = ticks.map(formatTick);
	const longest = Math.max(...tickLabels.map((t) => t.length));

	// Left padding is derived from the longest tick label so wide labels
	// (e.g. "1.2M") never get clipped against the axis.
	const leftPad = 14 + longest * 6.5;
	const rightPad = 10;
	const topPad = 22;
	const bottomPad = 26;
	const plotHeight = 140;
	const slotWidth = 64;
	const vbWidth = leftPad + rightPad + points.length * slotWidth;
	const vbHeight = topPad + plotHeight + bottomPad;
	const chartBottom = topPad + plotHeight;

	const svg = svgEl("svg", {
		class: "chart-svg",
		viewBox: `0 0 ${vbWidth} ${vbHeight}`,
		role: "img",
		"aria-label": `Saved versus wasted money for the last ${points.length} months`,
	});

	const currencyLabel = svgEl("text", { class: "chart-currency", x: leftPad, y: 12 });
	currencyLabel.textContent = "so'm";
	svg.append(currencyLabel);

	for (const tickVal of ticks) {
		const y = chartBottom - (tickVal / top) * plotHeight;
		const line = svgEl("line", {
			class: "gridline",
			x1: leftPad, x2: vbWidth - rightPad,
			y1: y, y2: y,
		});
		const label = svgEl("text", {
			x: leftPad - 6, y: y + 3,
			"text-anchor": "end",
		});
		label.textContent = tickVal === 0 ? "0" : formatTick(tickVal);
		svg.append(line, label);
	}

	const tooltip = document.createElement("div");
	tooltip.className = "chart-tooltip";
	tooltip.hidden = true;
	tooltip.setAttribute("role", "tooltip");

	function showTooltip(evt, text) {
		tooltip.textContent = text;
		tooltip.hidden = false;
		const rect = container.getBoundingClientRect();
		const srcRect = evt.target.getBoundingClientRect();
		tooltip.style.left = `${srcRect.left - rect.left + srcRect.width / 2}px`;
		tooltip.style.top = `${srcRect.top - rect.top}px`;
	}
	function hideTooltip() {
		tooltip.hidden = true;
	}

	const barWidth = 18;
	const barGap = 6;
	points.forEach((p, i) => {
		const slotX = leftPad + i * slotWidth;
		const groupCenter = slotX + slotWidth / 2;

		const savedHeight = (p.saved / top) * plotHeight;
		const wastedHeight = (p.wasted / top) * plotHeight;
		const savedX = groupCenter - barWidth - barGap / 2;
		const wastedX = groupCenter + barGap / 2;

		const savedRect = svgEl("rect", {
			class: "bar-saved",
			x: savedX, width: barWidth,
			y: chartBottom - savedHeight, height: Math.max(savedHeight, 0),
			tabindex: "0",
			"aria-label": `${monthLabel(p.month)} saved ${formatMoney(p.saved)}`,
		});
		const wastedRect = svgEl("rect", {
			class: "bar-wasted",
			x: wastedX, width: barWidth,
			y: chartBottom - wastedHeight, height: Math.max(wastedHeight, 0),
			tabindex: "0",
			"aria-label": `${monthLabel(p.month)} wasted ${formatMoney(p.wasted)}`,
		});

		const tipText = `${monthLabel(p.month)} — saved ${formatMoney(p.saved)}, wasted ${formatMoney(p.wasted)}`;
		for (const rect of [savedRect, wastedRect]) {
			rect.addEventListener("mouseenter", (e) => showTooltip(e, tipText));
			rect.addEventListener("focus", (e) => showTooltip(e, tipText));
			rect.addEventListener("touchstart", (e) => showTooltip(e, tipText), { passive: true });
			rect.addEventListener("mouseleave", hideTooltip);
			rect.addEventListener("blur", hideTooltip);
		}

		const xLabel = svgEl("text", {
			x: groupCenter, y: chartBottom + 16,
			"text-anchor": "middle",
		});
		xLabel.textContent = monthAbbr(p.month);

		svg.append(savedRect, wastedRect, xLabel);
	});

	container.append(svg, tooltip);
	container.append(buildTableFallback(points));
}

function buildTableFallback(points) {
	const table = document.createElement("table");
	table.className = "visually-hidden";
	const caption = document.createElement("caption");
	caption.textContent = "Saved vs wasted by month";
	const thead = document.createElement("thead");
	const headRow = document.createElement("tr");
	for (const h of ["Month", "Saved", "Wasted"]) {
		const th = document.createElement("th");
		th.textContent = h;
		headRow.append(th);
	}
	thead.append(headRow);
	const tbody = document.createElement("tbody");
	for (const p of points) {
		const row = document.createElement("tr");
		const cells = [monthLabel(p.month), formatMoney(p.saved), formatMoney(p.wasted)];
		for (const c of cells) {
			const td = document.createElement("td");
			td.textContent = c;
			row.append(td);
		}
		tbody.append(row);
	}
	table.append(caption, thead, tbody);
	return table;
}

async function loadChart() {
	try {
		const points = await apiFetch(`/api/chart?months=${state.chartMonths}`);
		renderChart(points);
	} catch (err) {
		showError(err.message);
	}
}

function setupChartToggle() {
	const buttons = document.querySelectorAll("#chart-period .chip");
	buttons.forEach((btn) => {
		btn.addEventListener("click", () => {
			state.chartMonths = Number(btn.dataset.months);
			buttons.forEach((b) => b.setAttribute("aria-pressed", String(b === btn)));
			loadChart();
		});
	});
}

async function loadSummary() {
	try {
		const summary = await apiFetch(`/api/summary?month=${state.month}`);
		renderStats(summary);
		renderCategoryBars(summary.by_category);
	} catch (err) {
		showError(err.message);
	}
}

setupMonthNav();
setupChartToggle();
loadSummary();
loadChart();
