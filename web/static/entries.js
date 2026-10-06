// Entries page: month-filtered list with chips, and an add/edit modal.
// Edits fill the modal from the already-loaded list — no GET-one call.

const state = {
	month: monthFromURL(),
	filter: "all",
	entries: [],
	editingID: null,
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

function setupFilterChips() {
	const chips = document.querySelectorAll("#kind-filter .chip");
	chips.forEach((chip) => {
		chip.addEventListener("click", () => {
			state.filter = chip.dataset.filter;
			chips.forEach((c) => c.setAttribute("aria-pressed", String(c === chip)));
			loadEntries();
		});
	});
}

function queryForFilter() {
	const params = new URLSearchParams({ month: state.month });
	switch (state.filter) {
		case "saving":
			params.set("kind", "saving");
			break;
		case "spend":
			params.set("kind", "spend");
			break;
		case "wasted":
			params.set("wasted", "1");
			break;
	}
	return params;
}

async function loadEntries() {
	try {
		const list = await apiFetch(`/api/entries?${queryForFilter().toString()}`);
		state.entries = list;
		renderEntries(list);
	} catch (err) {
		showError(err.message);
	}
}

function renderEntries(list) {
	const container = document.getElementById("entries-list");
	container.textContent = "";

	if (list.length === 0) {
		const empty = document.createElement("p");
		empty.className = "empty-state";
		empty.textContent = "No entries yet.";
		container.append(empty);
		return;
	}

	for (const entry of list) {
		container.append(buildEntryRow(entry));
	}
}

function buildEntryRow(entry) {
	const row = document.createElement("div");
	row.className = "entry-row";

	const date = document.createElement("div");
	date.className = "entry-date";
	date.textContent = entry.date;

	const main = document.createElement("div");
	main.className = "entry-main";
	const category = document.createElement("span");
	category.className = "entry-category";
	category.textContent = entry.category;
	main.append(category);
	if (entry.note) {
		const note = document.createElement("div");
		note.className = "entry-note";
		note.textContent = entry.note;
		main.append(note);
	}

	const amountWrap = document.createElement("div");
	const amount = document.createElement("span");
	amount.className = "entry-amount" + (entry.kind === "saving" ? " saving" : "");
	amount.textContent = formatMoney(entry.amount);
	amountWrap.append(amount);
	if (entry.wasted) {
		const badge = document.createElement("span");
		badge.className = "badge-wasted";
		badge.textContent = "WASTED";
		amountWrap.append(badge);
	}

	const actions = document.createElement("div");
	actions.className = "entry-actions";

	const editBtn = document.createElement("button");
	editBtn.type = "button";
	editBtn.textContent = "Edit";
	editBtn.addEventListener("click", () => openModal("edit", entry));

	const deleteBtn = document.createElement("button");
	deleteBtn.type = "button";
	deleteBtn.className = "danger";
	deleteBtn.textContent = "Delete";
	deleteBtn.addEventListener("click", () => deleteEntry(entry));

	actions.append(editBtn, deleteBtn);

	if (entry.kind === "spend") {
		const wastedBtn = document.createElement("button");
		wastedBtn.type = "button";
		wastedBtn.textContent = entry.wasted ? "Unmark" : "Mark wasted";
		wastedBtn.addEventListener("click", () => toggleWasted(entry));
		actions.append(wastedBtn);
	}

	row.append(date, main, amountWrap, actions);
	return row;
}

async function deleteEntry(entry) {
	if (!confirm(`Delete this ${entry.kind === "saving" ? "saving" : "spend"} of ${formatMoney(entry.amount)}?`)) {
		return;
	}
	try {
		await apiFetch(`/api/entries/${entry.id}`, { method: "DELETE" });
		await loadEntries();
	} catch (err) {
		showError(err.message);
	}
}

async function toggleWasted(entry) {
	try {
		await apiFetch(`/api/entries/${entry.id}/wasted`, {
			method: "POST",
			body: JSON.stringify({ wasted: !entry.wasted }),
		});
		await loadEntries();
	} catch (err) {
		showError(err.message);
	}
}

// ---- modal ----

const modal = {
	backdrop: null,
	form: null,
	title: null,
	kindButtons: null,
	wastedRow: null,
	saveBtn: null,
};

function cacheModalEls() {
	modal.backdrop = document.getElementById("entry-modal");
	modal.form = document.getElementById("entry-form");
	modal.title = document.getElementById("modal-title");
	modal.kindButtons = document.querySelectorAll("#kind-toggle .chip");
	modal.wastedRow = document.getElementById("wasted-row");
	modal.saveBtn = document.getElementById("modal-save");
}

function clearFieldErrors() {
	for (const id of ["error-amount", "error-category", "error-date", "error-note"]) {
		document.getElementById(id).textContent = "";
	}
}

function setKind(kind) {
	modal.kindButtons.forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.kind === kind)));
	modal.wastedRow.hidden = kind !== "spend";
	if (kind !== "spend") {
		document.getElementById("field-wasted").checked = false;
	}
	loadCategoryOptions(kind);
}

function currentKind() {
	const pressed = Array.from(modal.kindButtons).find((b) => b.getAttribute("aria-pressed") === "true");
	return pressed ? pressed.dataset.kind : "spend";
}

async function loadCategoryOptions(kind) {
	const datalist = document.getElementById("category-options");
	datalist.textContent = "";
	try {
		const cats = await apiFetch(`/api/categories?kind=${kind}`);
		for (const cat of cats) {
			const option = document.createElement("option");
			option.value = cat;
			datalist.append(option);
		}
	} catch (err) {
		showError(err.message);
	}
}

function openModal(mode, entry) {
	clearFieldErrors();
	state.editingID = mode === "edit" ? entry.id : null;
	modal.title.textContent = mode === "edit" ? "Edit entry" : "Add entry";

	const kind = mode === "edit" ? entry.kind : "spend";
	setKind(kind);

	document.getElementById("field-amount").value = mode === "edit" ? String(entry.amount) : "";
	document.getElementById("field-category").value = mode === "edit" ? entry.category : "";
	document.getElementById("field-date").value = mode === "edit" ? entry.date : todayLocal();
	document.getElementById("field-date").max = todayLocal();
	document.getElementById("field-note").value = mode === "edit" ? entry.note : "";
	document.getElementById("field-wasted").checked = mode === "edit" ? !!entry.wasted : false;

	modal.backdrop.hidden = false;
	document.getElementById("field-amount").focus();
}

function closeModal() {
	modal.backdrop.hidden = true;
	state.editingID = null;
}

function validateForm() {
	clearFieldErrors();
	let ok = true;
	let amount = null;

	try {
		amount = parseMoney(document.getElementById("field-amount").value);
	} catch (err) {
		document.getElementById("error-amount").textContent = err.message;
		ok = false;
	}

	const category = document.getElementById("field-category").value.trim();
	if (category.length > 30) {
		document.getElementById("error-category").textContent = "Category must be at most 30 characters";
		ok = false;
	}

	const date = document.getElementById("field-date").value;
	if (!date) {
		document.getElementById("error-date").textContent = "Date is required";
		ok = false;
	} else if (date > todayLocal()) {
		document.getElementById("error-date").textContent = "Date cannot be in the future";
		ok = false;
	}

	const note = document.getElementById("field-note").value;
	if (note.length > 200) {
		document.getElementById("error-note").textContent = "Note must be at most 200 characters";
		ok = false;
	}

	if (!ok) return null;
	return {
		kind: currentKind(),
		amount,
		category,
		date,
		note,
		wasted: currentKind() === "spend" && document.getElementById("field-wasted").checked,
	};
}

async function handleSubmit(e) {
	e.preventDefault();
	const payload = validateForm();
	if (!payload) return;

	modal.saveBtn.disabled = true;
	try {
		if (state.editingID) {
			await apiFetch(`/api/entries/${state.editingID}`, {
				method: "PUT",
				body: JSON.stringify(payload),
			});
		} else {
			await apiFetch("/api/entries", {
				method: "POST",
				body: JSON.stringify(payload),
			});
		}
		closeModal();
		await loadEntries();
	} catch (err) {
		showError(err.message);
	} finally {
		modal.saveBtn.disabled = false;
	}
}

function setupModal() {
	cacheModalEls();
	modal.kindButtons.forEach((btn) => {
		btn.addEventListener("click", () => setKind(btn.dataset.kind));
	});
	document.getElementById("add-entry-btn").addEventListener("click", () => openModal("add"));
	document.getElementById("modal-cancel").addEventListener("click", closeModal);
	modal.form.addEventListener("submit", handleSubmit);
	modal.backdrop.addEventListener("click", (e) => {
		if (e.target === modal.backdrop) closeModal();
	});
}

setupMonthNav();
setupFilterChips();
setupModal();
loadEntries().then(() => {
	if (new URLSearchParams(location.search).get("add") === "1") {
		openModal("add");
	}
});
