// serve.js
//
// This is the companion javascript module to the Serve web framework.
//


const hiddenInputPrefix = "Serve__"; // corresponds with same in go code.
const jsonObjectType = "objType"; // corresponds to same in go code.

const changedControlIds = new Set();
let ajaxError = false;
let discardEvents = false; // Used, for example, to debounce a submit button.
const controlIdsToRefresh = new Set();

// Injected functions
export const testFuncs = {
    testStep() {
    }
};
export const websocketFuncs = {
    close() {
    }
};


/**
 * Returns the generated serve form.
 */
export function getForm() {
    return document.querySelector('form[data-srv]');
}

/**
 * Initializes the form.
 *
 * Called by a generated script embedded in each page to attach required events and
 * callbacks to the form.
 */
export function initForm() {
    let frm = getForm();

    if (!frm) {
        console.error('Form not found');
        return;
    }
    trackChanges(frm)
}


// Change tracking

/**
 * Records a change in a control.
 *
 * After a change is recorded, the value of the control will be sent to the app
 * on the next ajax call. This is exported for special situations where the
 * "change" or "input" event does not capture a value before the value is needed.
 * @param {string} id
 */

export function recordChange(id) {
    if (!id) {
        return;
    }
    changedControlIds.add(id);
}

function isTrackableControl(el, frm) {
    return (
        el instanceof HTMLElement &&
        typeof el.form !== "undefined" &&
        el.form === frm &&
        el.id
    );
}

/**
 * Begins change tracking on a form.
 *
 * This should be called when a form is first rendered.
 *
 * @param {HTMLFormElement} frm
 */
function trackChanges(frm) {
    const handler = (e) => {
        const el = e.target;
        if (!isTrackableControl(el, frm)) return;

        recordChange(el.id);
    };

    for (const type of ["input", "change"]) {
        frm.addEventListener(type, handler, {capture: true});
    }
}

/**
 * @typedef {Object} ActionValues
 * @property {*}    [event]      The event's action value, if one is provided. This can be any type, including an object.
 * @property {*}    [action]     The action's action value, if one is provided. Any type.
 * @property {*}    [control]    The control's action value, if one is provided. Any type.
 */

/**
 * @typedef {Object} ActionParams
 * @property {string} [controlId]           The control id to post an action to
 * @property {number} [eventId]             The event id
 * @property {ActionValues} [actionValues]  Values to send with the event
 */

/**
 * Posts an ajax call to the ajax queue. Ajax actions call this.
 *
 * @param {ActionParams} params
 *
 * @return {Promise<void>}
 */
export async function postAjax(params) {
    if (discardEvents) {
        return;
    }
    try {
        let frm = getForm();
        if (!frm) throw new Error('Form not found');
        await enqueueAjax(
            frm.action , // should not change even if form is replaced.
            () => ajaxFormData(params)   // gathered right before send
        );
    } catch (err) {
        displayAjaxError(err.body);
        testFuncs.testStep();
    } finally {
        discardEvents = false;
    }
}

/*
 * Returns a form data object with the currently changed form elements to
 * be used to send to the app through an ajax call.
 *
 * Note that this data will be different from the data sent as a form post.
 * The app will detect this.
 *
 * @param {ActionParams} params
 */
function ajaxFormData(params) {
    let frm = getForm();
    let fd = new FormData();

    if (!frm) throw new Error("Serve form not found");

    for (const el of frm.elements) {
        let id = el.id;
        if (!id) continue;
        let changed = changedControlIds.has(id);
        let alwaysPost = el.hasAttribute("data-srv-post");
        if (changed ||
            alwaysPost ||
            ajaxError) {

            switch (el.type) {
                case "radio":
                    // Only a checked button is sent.
                    // It is up to the server to reset the states for the other radios in the group.
                    // Must be careful in case multiple radios were checked to get only the currently active one.
                    // The name is the group.
                    if (el.checked) {
                        fd.set(id, "on")
                        if (el.name) {
                            fd.set(el.name, el.value);
                        }
                    }
                    break;
                case "checkbox":
                    fd.set(id, el.checked ? "on" : "off"); // notify for both on and off
                    break;
                default:
                    if (el.files instanceof FileList) {
                        for (const file of el.files) fd.append(id, file, file.name);
                    } else if (Array.isArray(el.value)) {
                        for (const v of el.value) fd.append(id, v);
                    } else {
                        fd.set(id, el.value);
                    }
                    break;
            }
        }
    }

    for (const id of controlIdsToRefresh) {
        fd.append(hiddenInputPrefix + "refresh", id);
    }

    // Always post timezone data
    fd.set(hiddenInputPrefix + "tzo", String(-new Date().getTimezoneOffset()));
    fd.set(hiddenInputPrefix + "tz", Intl.DateTimeFormat().resolvedOptions().timeZone);

    fd.set(hiddenInputPrefix + "params", JSON.stringify(params));

    changedControlIds.clear();
    controlIdsToRefresh.clear();
    return fd;
}

/**
 * Will cause the form to update without a specific action. Useful if you know that javascript control
 * values have changed and want to post the changes back to the server to get a response.
 */
function updateForm() {
    throttle('serve.update', 500, function () {
        void postAjax({}); // call postAjax and not wait
    });
}

/**
 * Will cause the given goradd control to refresh
 * @param {string} id
 */
export function refresh(id) {
    controlIdsToRefresh.add(id);
    updateForm();
}

/***********
 * Response Processing
 ************/

/**
 * Ways to manipulate a particular control via javascript through Ajax.
 *
 * @typedef {Object} AjaxResponseControl
 * @property {Object<string, *>} [properties]
 * @property {Object<string, string>} [attributes]  Attributes to set via setAttribute.
 * @property {string} [html] Replaces the entire control with this html, or appends new html to form's innerhtml if control not specified.
 **/

/**
 * Commands the app can send the document.
 *
 * @typedef {Object} AjaxResponseCommand
 * @property {string} [script]      Javascript to execute
 * @property {string} [selector]    A selector of items to run the command on. If an id is present, it will subselect on the id. Multiple results are fine.
 * @property {string} [id]          An id of an item to run the command on.
 * @property {string} [func]        The function to call on the selected items. If no id or selector, the window is the target.
 *                                  Function names can be split by periods (.) to search for a function on a sub object of the target.
 * @property {*[]} [params]         The arguments of the function
 * @property {boolean} [final]      Whether this should be executed after all other commands
 **/

/**
 * The structure of the Ajax response.
 *
 * @typedef {Object} AjaxResponse
 * @property {string[]} [alerts]      Strings to display in an alert.
 * @property {string[]} [styleSheets] Style sheets to load on the fly.
 * @property {string[]} [js]         JavaScript modules to load on the fly.
 * @property {Object<string, AjaxResponseControl>} [controls] Controls to change and how to change them, mapped by id.
 * @property {AjaxResponseCommand[]} [commands] Commands to execute
 * @property {boolean} [winclose] Directs the browser to close the current window
 * @property {string} [loc] A new location to navigate to, or "reload" to reload the current page.
 **/

/**
 * Load a stylesheet and resolve when it’s applied.
 * @param {string} href
 * @returns {Promise<void>}
 */
function loadStyleSheet(href) {
    return new Promise((resolve, reject) => {
        const link = document.createElement("link");
        link.rel = "stylesheet";
        link.href = href;
        link.onload = () => resolve();
        link.onerror = () => reject(new Error(`Failed to load stylesheet: ${href}`));
        document.head.appendChild(link);
    });
}

/**
 * Load a JS module (ESM).
 * @param {string} specifier
 * @returns {Promise<any>}
 */
async function loadModule(specifier) {
    // specifier can be a URL or bare specifier (depending on your build/env)
    return import(specifier);
}

/**
 * Apply a control update.
 * @param {AjaxResponseControl} update
 * @param {string} id
 */
function applyControlUpdate(update, id) {
    const el = document.getElementById(id);

    // set properties
    if (update.properties && el) {
        for (const [k, v] of Object.entries(update.properties)) {
            try {
                el[k] = v;
            } catch (e) {
                console.warn(`Property '${k}' does not exist on #${id}`);
            }
        }
    }

    // set attributes
    if (update.attributes && el) {
        for (const [k, v] of Object.entries(update.attributes)) {
            el.setAttribute(k, String(v));
        }
    }

    // html replacement / injection
    if (update.html !== undefined) {
        if (el) {
            el.outerHTML = update.html;
        } else {
            // No existing node: append to end of form if present
            let frm = getForm();
            if (frm) {
                frm.insertAdjacentHTML("beforeend", update.html);
            }
        }
    }
}

const pendingFinalCommands = [];
let flushScheduled = false;

function flushFinalCommands() {
    while (pendingFinalCommands.length) {
        const cmd = pendingFinalCommands.shift();
        applyCommand(cmd);
    }
}

function scheduleFinalFlush() {
    if (flushScheduled) return;
    flushScheduled = true;

    // Defer flush until ajax queue is truly idle.
    void whenAjaxIdle().then(() => {
        flushScheduled = false;
        flushFinalCommands();
    });
}

/**
 * Executes a single Ajax response command.
 *
 * A command may either execute arbitrary JavaScript provided by the server,
 * or invoke a function on one or more target objects resolved from the document
 * or the global window object.
 *
 * Targets are resolved in the following order:
 *   - If neither `id` nor `selector` is provided, the global `window` object
 *     is used as the target.
 *   - If `id` is provided, the element with that id is used as the target.
 *   - If `selector` is provided, all matching elements are used as targets.
 *     If both `id` and `selector` are provided, the selector is applied within
 *     the element identified by `id`.
 *
 * The `func` property specifies a dot-delimited path to a function that will be
 * invoked on each target (for example: "classList.add" or "console.log").
 * Any parameters supplied in `params` are passed to the function.
 *
 * This function assumes that commands originate from a trusted source.
 *
 * @param {AjaxResponseCommand} cmd
 */
function applyCommand(cmd) {
    if (cmd.script) {
        // Possibly remove if not really needed
        new Function(cmd.script)();
        return;
    }
    if (cmd.func) {
        // Get the objects that are the target of the function
        let targets = [window];
        if (cmd.selector) {
            if (cmd.id) {
                const el = document.getElementById(cmd.id);
                if (!el) {
                    console.error("Could not find element by the id provided.")
                    return;
                }
                targets = Array.from(el.querySelectorAll(cmd.selector));
            } else {
                targets = Array.from(document.querySelectorAll(cmd.selector));
            }
        } else if (cmd.id) {
            const el = document.getElementById(cmd.id);
            if (!el) {
                console.error("Could not find element by the id provided.")
                return;
            }
            targets = [el];
        }
        const funcParts = cmd.func.split(".");
        const args = unpackArray(cmd.params) ?? [];

        for (let target of targets) {
            let ctx = null;
            let cur = target;
            for (const key of funcParts) {
                ctx = cur;
                cur = cur?.[key];
                if (cur == null) break;
            }

            if (typeof cur !== "function") {
                console.error(`Command function not found or not callable: ${cmd.func}`);
                continue;
            }

            // obj should be the function object and ctx the parent element
            cur.apply(ctx, args);
        }
    }
}


/**
 *
 * @param {AjaxResponse} resp
 * @returns {Promise<void>}
 */
async function processAjaxResponse(resp) {
    if (resp.alerts?.length) {
        for (const msg of resp.alerts) window.alert(msg);
    }

    const loaders = [];

    if (resp.styleSheets?.length) {
        for (const href of resp.styleSheets) loaders.push(loadStyleSheet(href));
    }

    if (resp.js?.length) {
        for (const spec of resp.js) loaders.push(loadModule(spec));
    }

    if (loaders.length) {
        await Promise.all(loaders);
    }

    if (resp.controls) {
        for (const [id, update] of Object.entries(resp.controls)) {
            applyControlUpdate(update, id);
        }
    }

    if (resp.commands?.length) {
        for (const cmd of resp.commands) {
            if (cmd.final) {
                pendingFinalCommands.push(cmd);
            } else {
                applyCommand(cmd);
            }
        }

        // If any finals were queued, flush them once ALL ajax is done.
        if (pendingFinalCommands.length) {
            scheduleFinalFlush();
        }
    }

    if (resp.winclose) {
        websocketFuncs.close();
        window.close();
        return;
    }
    if (resp.loc) {
        websocketFuncs.close();
        if (resp.loc === "reload") {
            window.location.reload();
        } else {
            window.location.assign(resp.loc);
        }
    }
}


/**
 * A parameter to send to a command. Could be values that cannot be normally
 * represented in json, such as a Date.
 *
 * @typedef {Object} AjaxResponseCommandParam
 * @property {string} objType     The type of a special data type.
 * @property {string} [func]      Javascript to execute.
 * @property {*[]} [params]       Function parameters.
 * @property {string} [varName]   Variable name.
 **/

/**
 * Convert from JSON return values into richer JS values.
 *
 * Supports special encoded objects:
 * - closure: creates a Function from source (trusted only!)
 * - date: decodes a date using unpackJsonDate
 * - varName: resolves a dotted path from window (e.g. "console.log")
 * - func: resolves and immediately calls a function, returning its result
 *
 * @param {any[] | null | undefined} arr
 * @returns {any[] | null}
 */
function unpackArray(arr) {
    if (!arr) return null;
    return arr.map(unpackObj);
}

/**
 * Resolve a dotted path starting from `window`.
 * @param {string} path
 * @returns {any}
 */
function resolveFromWindow(path) {
    return path.split(".").reduce((cur, key) => cur?.[key], window);
}

/**
 * @param {any} obj
 * @returns {any}
 */
function unpackObj(obj) {
    if (obj == null) return obj;

    // Arrays: recurse
    if (Array.isArray(obj)) return unpackArray(obj);

    // Non-object primitives: no change
    if (typeof obj !== "object") return obj;

    // Special object?
    const kind = obj.serveObj;
    switch (kind) {
        case "closure": {
            // obj.func is source code for the body; obj.params optional list of parameter names
            // Preserving original intent: decode params then pass as param list.
            const params = obj.params ? obj.params.map(unpackObj) : [];
            return params.length ? new Function(...params, obj.func) : new Function(obj.func);
        }

        case "date":
            return unpackJsonDate(obj);

        case "varName":
            return resolveFromWindow(obj.varName);

        case "func": {
            // Find target context from window, then call func on it
            const target = obj.context ? resolveFromWindow(obj.context) : window;
            const fn = target?.[obj.func];
            if (typeof fn !== "function") return undefined;

            const params = obj.params ? obj.params.map(unpackObj) : [];
            return fn.apply(target, params);
        }

        default:
            // Unknown discriminator: fall through to deep-unpack as plain object
            break;
    }


    // Plain object: deep-unpack entries
    const out = {};
    for (const [k, v] of Object.entries(obj)) {
        out[k] = unpackObj(v);
    }
    return out;
}

/**
 * Unpacks a date object that was packed by dateTime.DateTime.MarshalJson. If the date represented a
 * timestamp on the server side, it will be a timestamp here, but the time will be in local time.
 * In other words, if the server timezone and browser timezone are different,
 * then they will show different times, but both will correspond to the same world time.
 * If on the server side the date represented simply a date and time in local time,
 * the date will become the same date and time in local time here. If the server timezone and browser
 * timezone are different, they will both show the same time, meaning they will not be the same world time.
 * If it was a zero date on the server, it becomes a null here.
 *
 * This solves some problems inherent in the traditional JSON date format consisting of an ISO8601 string.
 *
 * @param {object} o
 * @returns {null|Date}
 */
function unpackJsonDate(o) {
    if (o.z) {
        return null;
    } else if (o.t) {
        return new Date(Date.UTC(o.y, o.mo, o.d, o.h, o.m, o.s, o.ms));
    } else {
        return new Date(o.y, o.mo, o.d, o.h, o.m, o.s, o.ms);
    }
}


/***************
 * Named Timers
 ***************/

const timers = new Map();

/**
 * @typedef {Object} NamedTimer
 * @property {number} i - insertion time (ms since epoch)
 * @property {number} d - delay requested (ms)
 * @property {boolean} p - periodic
 * @property {number} s - start time of last firing (ms)
 * @property {number} e - end time of last firing (ms)
 * @property {number|null} t - timeout/interval handle (null if inactive)
 */

/**
 * Sets the named timer.
 *
 * @param {string} id
 * @param {(timer: NamedTimer) => void} action
 * @param {number} delayMs
 * @param {boolean} [periodic=false]
 */
export function setTimer(id, action, delayMs, periodic = false) {
    clearTimer(id);

    /** @type {NamedTimer} */
    const timer = {
        i: Date.now(),
        d: delayMs,
        p: !!periodic,
        s: 0,
        e: 0,
        t: null
    };

    const runOnce = () => {
        timer.s = Date.now();
        clearTimer(id);          // match old behavior: one-shot clears itself before action
        action(timer);
        timer.e = Date.now();
    };

    const runPeriodic = () => {
        // optionally record timings for periodic too
        timer.s = Date.now();
        action(timer);
        timer.e = Date.now();
    };

    timer.t = timer.p
        ? setInterval(runPeriodic, delayMs)
        : setTimeout(runOnce, delayMs);

    timers.set(id, timer);
}

/**
 * Returns true if there is an active timer for the given id.
 * @param {string} id
 * @returns {boolean}
 */
export function hasTimer(id) {
    const t = timers.get(id);
    return !!t && t.t != null;
}

/**
 * Clears the named timer.
 * @param {string} id
 */
export function clearTimer(id) {
    const t = timers.get(id);
    if (!t || t.t == null) return;

    if (t.p) clearInterval(t.t);
    else clearTimeout(t.t);

    t.t = null; // keep history
}

/**
 * Creates a timer on a control that fires a 'timerexpiredevent'.
 * @param {string} controlID
 * @param {number} delayMs
 * @param {boolean} [periodic=false]
 */
export function startControlTimer(controlID, delayMs, periodic = false) {
    const timerId = `${controlID}_ct`;
    setTimer(timerId, () => {
        const el = document.getElementById(controlID);
        if (!el) return;
        el.dispatchEvent(new CustomEvent("timerexpiredevent", {bubbles: true}));
    }, delayMs, periodic);
}

/**
 * Stops the control's timer.
 * @param {string} controlID
 */
export function stopControlTimer(controlID) {
    clearTimer(`${controlID}_ct`);
}

/**
 * Throttle: schedules f so it runs such that a minimum of minIntervalMs milliseconds
 * will occur between this firing and the previous firing of the same action.
 *
 * id is used to track and know that it is the same action.
 *
 * If already scheduled, does nothing.
 * @param {string} id
 * @param {number} minIntervalMs
 * @param {() => void} f
 */
export function throttle(id, minIntervalMs, f) {
    if (hasTimer(id)) return; // already scheduled

    const prev = timers.get(id);
    const prevEnd = prev?.e > 0 ? prev.e : 0;

    const now = Date.now();
    const delay = Math.max(minIntervalMs - (now - prevEnd), 0);

    // Keep timing/history consistent with setTimer
    setTimer(id, () => f(), delay, false);
}

/************
 * Ajax Queue
 *
 * Serializes ajax POST requests so only one is in-flight at a time.
 *
 * This queue is intended for relatively quick requests and therefore does not
 * track upload/download progress. If you need progress reporting, implement a
 * specialized uploader/downloader.
 *****************/

/**
 * @typedef {Object} AjaxResult
 * @property {number} status - HTTP status code (typically 200 on success).
 * @property {any} json - Parsed JSON response body.
 */

let ajaxTimeout = 0;

/**
 * Sets the number of milliseconds ajax operations will wait before timing out.
 * A value of 0 disables timeouts.
 *
 * @param {number} ms
 */
export function setAjaxTimeout(ms) {
    ajaxTimeout = ms;
}

/**
 * Sends FormData to a URL via POST.
 *
 * @param {string} url
 * @param {FormData} data
 * @returns {Promise<Response>}
 */
async function fetchForm(url, data) {
    const headers = new Headers({"X-Requested-With": "XMLHttpRequest"});

    const init = {
        method: "POST",
        body: data,
        headers,
        credentials: "same-origin",
    };

    if (ajaxTimeout === 0) {
        return fetch(new Request(url, init));
    }

    const controller = new AbortController();
    const t = setTimeout(() => controller.abort(new Error("timeout")), ajaxTimeout);

    try {
        return await fetch(new Request(url, {...init, signal: controller.signal}));
    } finally {
        clearTimeout(t);
    }
}

// ---- Promise-chained serial queue ----

let tail = Promise.resolve();

/**
 * Enqueue an ajax POST request that runs serially.
 *
 * `dataCallback` is invoked only when the request is about to be sent, so it can
 * gather fresh FormData at send-time.
 *
 * Resolves with `{ status, json }` on success.
 * Rejects with an Error on HTTP/network/timeout/JSON-parse failure. On failures,
 * the error may include:
 * - `err.status` (number)
 * - `err.body` (string response body, if available)
 * - `err.cause` (original exception, if any)
 *
 * @param {string} url
 * @param {() => FormData} dataCallback
 * @returns {Promise<void>}
 **/
function enqueueAjax(url, dataCallback) {
    if (typeof dataCallback !== "function") {
        throw new TypeError("dataCallback must be a function returning FormData");
    }

    const job = tail.then(async () => {
        const data = dataCallback();
        if (!(data instanceof FormData)) {
            throw new TypeError("dataCallback must return a FormData object");
        }

        const res = await fetchForm(url, data);
        const text = await res.text();

        if (!res.ok) {
            const err = new Error(`HTTP ${res.status}: ${res.statusText}`);
            err.status = res.status;
            err.body = text;
            throw err;
        }

        let j = {};
        try {
            j = JSON.parse(text);
        } catch (e) {
            const err = new Error("Invalid JSON response");
            err.status = res.status;
            err.body = text;
            err.cause = e;
            throw err;
        }
        await processAjaxResponse(j);
    });

    // Keep the queue alive even if this job fails
    tail = job.catch(() => {}).finally(() => {
        // Let response processing flush finals when the queue is truly empty.
        // We just schedule an idle check trigger point here.
        scheduleFinalFlush();
    });

    return job;
}

/**
 * Resolves once the ajax queue becomes idle (no queued/in-flight ajax jobs).
 * If new jobs are enqueued while waiting, it waits until those complete too.
 * @returns {Promise<void>}
 */
export function whenAjaxIdle() {
    return new Promise((resolve) => {
        const check = () => {
            const snap = tail;
            snap.then(() => {
                // If no one appended after we captured snap, we're idle now.
                if (tail === snap) resolve();
                else queueMicrotask(check);
            });
        };
        queueMicrotask(check);
    });
}

/**
 * Displays the ajax error in either a popup window, or a new web page.
 * @param {string} resultText
 * @private
 */
function displayAjaxError(resultText) {
    window.alert(`An error occurred.\r
\r
${resultText}`);
}

/**************
 * Event handling support
 **************/

/**
 * Discards all later events until the current event is processed.
 *
 * Used by event handlers to debounce events that should not be processed multiple times...
 * a submit button for instance.
 */
export function blockEvents() {
    discardEvents = true;
}
