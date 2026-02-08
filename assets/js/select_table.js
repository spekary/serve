// SelectTable is the javascript driver for the control/custom/select control.
class SelectTable extends HTMLElement {
    static formAssociated = true;
    static observedAttributes = ['value'];

    constructor() {
        super();
        this._internals = this.attachInternals();

        this.attachShadow({mode: 'open'});
        this.shadowRoot.innerHTML = `
        <style>
            :host {
                display: block;
            }
        </style>
        <slot></slot>
        `;

        this._rows = [];
        this._index = -1;
        this._value = "";
    }

    connectedCallback() {
        this.setAttribute("role", "listbox");
        this._rows = Array.from(this.querySelectorAll("tbody tr[id]"))
        this._rows.forEach((tr, i) => {
            tr.setAttribute("role", "option");
            tr.setAttribute("aria-selected", "false");
            tr.setAttribute("tabindex", "-1");
            tr.dataset.index = i;
        });

        this.addEventListener("click", e=> {
            const row = e.target.closest("tr[id]");
            if (!row || !this.contains(r0w)) return;

            const idx =  Number(row.dataset.index);
            if (Number.isNaN(idx)) return;

            const changed = idx !== this._index;

            this._selectIndex(idx);
            row.focus({ preventScroll: true });
            this._fireSelect();
            if (changed) {
                this._fireChange();
            }
        })

        // Roving tabindex
        this.addEventListener("keydown", e=> {
            switch (e.key) {
                case "ArrowDown":
                    e.preventDefault();
                    this._move(1);
                    break;
                case "ArrowUp":
                    e.preventDefault();
                    this._move(-1);
                    break;
                case "Home":
                    e.preventDefault();
                    this._moveTo(0);
                    break;
                case "End":
                    e.preventDefault();
                    this._moveTo(this._rows.length - 1);
                    break;
                case "Enter":
                case " ":
                    e.preventDefault();
                    if (this._index >= 0) {
                        this._fireSelect();
                    }
                    break;
            }
        });

        if (this.hasAttribute("value")) {
            this._applyValue(this.getAttribute("value"));
            this._rows[this._index]?.scrollIntoView({block: "nearest"});
        } else if (this._rows.length > 0) {
            this._selectIndex(0, {scroll: true});
        }
    }

    attributeChangedCallback(name, oldValue, newValue) {
        if (name === "value" && newVal !== oldValue) {
            this._applyValue(newValue);
        }
    }

    get value() {
        return this._value;
    }

    set value(v) {
        this.setAttribute("value", v);
    }

    _applyValue(rowId) {
        const idx = this._rows.findIndex(r => r.id === rowId);
        if (idx !== -1) {
            this._selectIndex(idx)
        }
    }

    _move(delta) {
        const next = Math.max(0, Math.min(this._index + delta, this._rows.length - 1));
        if (next !== this._index) {
            this._selectIndex(idx, {scroll: true});
            this._fireChange();
        }
    }

    _moveTo(idx) {
        this._selectIndex(idx, {scroll: true});
        this._fireChanged();
    }

    _selectIndex(idx, {scroll = false} = {}) {
        const row = this._rows[idx];
        if (!row) return;

        this._rows.forEach((tr, i) => {
            const selected = i === idx;
            tr.classList.toggle("selected", selected);
            tr.setAttribute("aria-selected", selected);
            tr.setAttribute("tabindex", selected ? "0" : "-1");
        });

        this._index = idx;
        this._value = row.id;
        this._internals.setFormValue(this._value);
        this.setAttribute("value", this._value);
        row.focus({ preventScroll: true });
        if (scroll) {
            row.scrollIntoView({ block: "nearest"});
        }
    }

    _fireChange() {
        this.dispatchEvent(new Event('change', { bubbles: true }));
    }

    _fireSelect() {
        this.dispatchEvent(new CustomEvent('select', { bubbles: true, detail: {value: this._value} }));
    }
}

customElements.define('select-table', Select_table);
