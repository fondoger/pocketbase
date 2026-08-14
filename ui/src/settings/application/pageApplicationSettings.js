import { settingsSidebar } from "../settingsSidebar";
import { batchAccordion } from "./batchAccordion";
import { rateLimitAccordion, sortRules } from "./rateLimitAccordion";
import { superuserAccordion } from "./superuserAccordion";
import { trustedProxyAccordion } from "./trustedProxyAccordion";

export function pageApplicationSettings() {
    app.store.title = "Application settings";

    const data = store({
        isLoading: false,
        isSaving: false,
        formSettings: null,
        originalFormSettings: null,
        get originalFormSettingsHash() {
            return JSON.stringify(data.originalFormSettings);
        },
        get formSettingsHash() {
            return JSON.stringify(data.formSettings);
        },
        get hasChanges() {
            return data.originalFormSettingsHash != data.formSettingsHash;
        },
    });

    loadSettings();

    async function loadSettings() {
        data.isLoading = true;

        try {
            const settings = await app.pb.settings.getAll();
            init(settings);

            data.isLoading = false;
        } catch (err) {
            if (!err.isAbort) {
                app.checkApiError(err);
                // data.isLoading = false; don't reset in case of a server error
            }
        }
    }

    function hasSuperuserIPsChanged() {
        return JSON.stringify(data.formSettings?.superuserIPs)
            != JSON.stringify(data.originalFormSettings?.superuserIPs);
    }

    async function saveWithConfirm() {
        const oldSuperuserIPs = app.utils.toArray(data.originalFormSettings?.superuserIPs);
        const superuserIPs = app.utils.toArray(data.formSettings?.superuserIPs);

        if (
            !superuserIPs.length
            // no change
            || JSON.stringify(oldSuperuserIPs) == JSON.stringify(superuserIPs)
        ) {
            return save();
        }

        return app.modals.confirm(
            t.div(
                { className: "txt-center" },
                t.h6(
                    null,
                    "The ONLY allowed superuser IPs will change to: ",
                    t.br(),
                    t.strong(null, superuserIPs.join(", ")),
                ),
                t.p(null, "Please make sure that your IP is in the list or you'll be locked."),
                t.p(
                    { className: "txt-hint" },
                    "In case of lockout, you can reset the setting with the ",
                    t.a(
                        {
                            href: import.meta.env.PB_SUPERUSER_IPS_RESET_DOCS,
                            target: "_blank",
                            rel: "noopener noreferrer",
                            className: "link-primary txt-bold txt-sm",
                        },
                        t.code(
                            null,
                            "superuser ips",
                            t.i({ ariaHidden: true, className: "ri-arrow-right-up-line txt-sm" }),
                        ),
                    ),
                    " console command.",
                ),
            ),
            () => save(),
            null,
            { yesButton: "Yes, save changes" },
        );
    }

    async function save() {
        if (data.isSaving || !data.hasChanges) {
            return;
        }

        data.isSaving = true;

        data.formSettings.rateLimits.rules = sortRules(data.formSettings.rateLimits.rules);

        try {
            const redacted = app.utils.filterRedactedProps(data.formSettings);

            const updatedSettings = await app.pb.settings.update(redacted);

            // reauthenticate to ensure that the superuser has still access
            if (hasSuperuserIPsChanged()) {
                try {
                    await app.pb.collection("_superusers").authRefresh();
                } catch (_) {
                    app.pb.authStore.clear();
                }
            }

            init(updatedSettings);

            app.toasts.success("Successfully saved application settings.");
        } catch (err) {
            app.checkApiError(err);
        }

        data.isSaving = false;
    }

    function init(settings = {}) {
        // refresh local app settings
        app.store.settings = JSON.parse(JSON.stringify(settings));

        // load from the css style as fallback
        if (!settings.meta?.accentColor) {
            const cssColor = window.getComputedStyle(document.documentElement)?.getPropertyValue("--accentColor");
            if (cssColor?.startsWith("#")) {
                settings.meta = settings.meta || {};
                settings.meta.accentColor = cssColor.toLowerCase() || "";
            }
        }

        data.originalFormSettings = {
            superuserIPs: settings.superuserIPs || [],
            meta: settings.meta || {},
            batch: settings.batch || {},
            trustedProxy: settings.trustedProxy || { headers: [] },
            rateLimits: settings.rateLimits || { excludedIPs: [], rules: [] },
        };

        sortRules(data.originalFormSettings.rateLimits.rules);

        data.formSettings = JSON.parse(JSON.stringify(data.originalFormSettings));
    }

    function reset() {
        data.formSettings = JSON.parse(data.originalFormSettingsHash);
    }

    return t.div(
        {
            pbEvent: "pageApplicationSettings",
            className: "page page-application-settings",
        },
        settingsSidebar(),
        t.div(
            { className: "page-content full-height" },
            t.header(
                { className: "page-header" },
                t.nav(
                    { className: "breadcrumbs" },
                    t.div({ className: "breadcrumb-item" }, "Settings"),
                    t.div({ className: "breadcrumb-item" }, "Application"),
                ),
            ),
            t.div(
                { className: "wrapper m-b-base" },
                () => {
                    if (data.isLoading) {
                        return t.div({ className: "block txt-center" }, t.span({ className: "loader lg" }));
                    }

                    return t.form(
                        {
                            pbEvent: "applicationSettingsForm",
                            className: "grid application-settings-form",
                            inert: () => data.isSaving,
                            onsubmit: (e) => {
                                e.preventDefault();
                                saveWithConfirm();
                            },
                        },
                        t.div(
                            { className: "col-md-5" },
                            t.div(
                                { className: "field" },
                                t.label({ htmlFor: "meta.appName" }, "Application name"),
                                t.input({
                                    id: "meta.appName",
                                    name: "meta.appName",
                                    type: "text",
                                    required: true,
                                    value: () => data.formSettings.meta.appName || "",
                                    oninput: (e) => (data.formSettings.meta.appName = e.target.value),
                                }),
                            ),
                        ),
                        t.div(
                            { className: "col-md-5" },
                            t.div(
                                { className: "field" },
                                t.label({ htmlFor: "meta.appURL" }, "Application URL"),
                                t.input({
                                    id: "meta.appURL",
                                    name: "meta.appURL",
                                    // note: text for compatibility with older versions
                                    // (https://github.com/pocketbase/pocketbase/issues/7681)
                                    //
                                    // @todo consider reverting back to "url" once enforced on the backend too
                                    type: "text",
                                    required: true,
                                    value: () => data.formSettings.meta.appURL || "",
                                    oninput: (e) => (data.formSettings.meta.appURL = e.target.value),
                                }),
                            ),
                        ),
                        t.div(
                            { className: "col-md-2" },
                            // pass isSaving to ensure that it will be rerendered after save
                            () => accentColorField(data, data.isSaving),
                        ),
                        t.div(
                            { className: "col-lg-12" },
                            () => batchAccordion(data),
                            () => trustedProxyAccordion(data),
                            () => rateLimitAccordion(data),
                            () => superuserAccordion(data),
                        ),
                        t.div(
                            { className: "col-lg-12" },
                            t.div(
                                { className: "field" },
                                t.input({
                                    id: "meta.hideControls",
                                    name: "meta.hideControls",
                                    type: "checkbox",
                                    className: "switch",
                                    checked: () => data.formSettings.meta.hideControls,
                                    onchange: (e) => (data.formSettings.meta.hideControls = e.target.checked),
                                }),
                                t.label(
                                    { htmlFor: "meta.hideControls" },
                                    t.span({ className: "txt" }, "Hide/Lock collection and record controls"),
                                    t.i({
                                        className: "ri-information-line link-hint",
                                        ariaDescription: app.attrs.tooltip(
                                            "To prevent accidental changes when in production environment, collections create and update buttons will be hidden.\nRecords update will also require an extra unlock step before save.",
                                        ),
                                    }),
                                ),
                            ),
                        ),
                        t.div({ className: "col-lg-12" }, t.hr()),
                        t.div(
                            { className: "col-lg-12" },
                            t.div(
                                { className: "flex" },
                                t.div({ className: "m-r-auto" }),
                                t.button(
                                    {
                                        type: "button",
                                        className: "btn transparent secondary",
                                        disabled: () => data.isSaving,
                                        hidden: () => !data.hasChanges,
                                        onclick: reset,
                                    },
                                    t.span({ className: "txt" }, "Cancel"),
                                ),
                                t.button(
                                    {
                                        className: () => `btn expanded-lg ${data.isSaving ? "loading" : ""}`,
                                        disabled: () => !data.hasChanges || data.isSaving,
                                    },
                                    t.span({ className: "txt" }, "Save changes"),
                                ),
                            ),
                        ),
                    );
                },
            ),
            t.footer({ className: "page-footer" }, app.components.credits()),
        ),
    );
}

function accentColorField(pageData) {
    const uniqueId = "accent_" + app.utils.randomString();

    const local = store({
        isTooLight: false,
    });

    let colorChangeTimeoutId;
    let tempNoAnimationTimeoutId;

    function changeAccentColor(color) {
        // temporary disable animations to minimize flickering
        clearTimeout(tempNoAnimationTimeoutId);
        document.documentElement.style.setProperty("--animationSpeed", "0");

        if (color) {
            document.documentElement.style.setProperty("--accentColor", color.toLowerCase());
        } else {
            document.documentElement.style.removeProperty("--accentColor");
        }

        // restore animation
        tempNoAnimationTimeoutId = setTimeout(() => {
            document.documentElement.style.removeProperty("--animationSpeed");
        }, 100);
    }

    const watchers = [
        watch(() => pageData.formSettings?.meta?.accentColor, (newColor) => {
            clearTimeout(colorChangeTimeoutId);
            colorChangeTimeoutId = setTimeout(() => {
                changeAccentColor(newColor);
            }, 100);
        }),
    ];

    return t.div(
        {
            className: "field",
            ariaDescription: app.attrs.tooltip(() => local.isTooLight ? "Invalid - color is too light" : ""),
            onunmount: () => {
                clearTimeout(colorChangeTimeoutId);
                changeAccentColor(pageData.formSettings.meta.accentColor);
                watchers.forEach((w) => w?.unwatch());
            },
        },
        t.label(
            { htmlFor: uniqueId },
            t.span({ className: "txt" }, "Accent"),
            t.i({
                hidden: () => !local.isTooLight,
                className: "txt-warning ri-alert-line",
            }),
        ),
        app.components.colorPicker({
            id: uniqueId,
            name: "meta.accentColor",
            predefinedColors: () => app.store.predefinedAccentColors,
            value: () => pageData.formSettings.meta.accentColor,
            onchange: (color) => {
                // @todo consider removing the constraint once contrast-color is implemented
                local.isTooLight = false;
                if (!app.utils.isDarkEnoughForWhiteText(color)) {
                    local.isTooLight = true;
                    return;
                }

                pageData.formSettings.meta.accentColor = color;
            },
        }),
    );
}
