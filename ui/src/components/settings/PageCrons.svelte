<script>
    import ApiClient from "@/utils/ApiClient";
    import tooltip from "@/actions/tooltip";
    import { pageTitle } from "@/stores/app";
    import { addSuccessToast } from "@/stores/toasts";
    import PageWrapper from "@/components/base/PageWrapper.svelte";
    import RefreshButton from "@/components/base/RefreshButton.svelte";
    import SettingsSidebar from "@/components/settings/SettingsSidebar.svelte";

    $pageTitle = "Crons";

    let crons = [];
    let isLeader = false;
    let isLoading = false;
    let isRunning = {};

    loadCrons();

    async function loadCrons() {
        isLoading = true;

        try {
            // Load crons list and leader status in parallel
            const [cronsResponse, leaderResponse] = await Promise.all([
                ApiClient.crons.getFullList(),
                fetch(`${ApiClient.baseUrl}/api/crons/leader-status`, {
                    headers: {
                        'Authorization': ApiClient.authStore.token ? `Bearer ${ApiClient.authStore.token}` : '',
                    },
                }).then(res => res.json())
            ]);

            crons = cronsResponse;
            isLeader = leaderResponse.isLeader || false;
            isLoading = false;
        } catch (err) {
            if (!err.isAbort) {
                ApiClient.error(err);
                isLoading = false;
            }
        }
    }

    async function cronRun(jobId) {
        if (!isLeader) {
            ApiClient.error(new Error("Manual cron job execution is only allowed on leader instances"));
            return;
        }

        isRunning[jobId] = true;

        try {
            await ApiClient.crons.run(jobId);
            addSuccessToast(`Successfully triggered ${jobId}.`);
            isRunning[jobId] = false;
        } catch (err) {
            if (!err.isAbort) {
                ApiClient.error(err);
                isRunning[jobId] = false;
            }
        }
    }
</script>

<SettingsSidebar />

<PageWrapper>
    <header class="page-header">
        <nav class="breadcrumbs">
            <div class="breadcrumb-item">Settings</div>
            <div class="breadcrumb-item">{$pageTitle}</div>
        </nav>
    </header>

    <div class="wrapper">
        <div class="panel" autocomplete="on">
            <div class="flex m-b-sm flex-gap-10">
                <span class="txt-xl">Registered app cron jobs</span>
                <div class="flex flex-gap-xs">
                    {#if isLoading}
                        <span class="skeleton-loader skeleton-loader-sm" />
                    {:else}
                        {#if isLeader}
                        <span class="label label-success">
                            <i class="ri-crown-line"></i>
                            Leader Instance
                        </span>
                    {:else}
                        <span class="label label-warning">
                            <i class="ri-team-line"></i>
                            Follower Instance
                        </span>
                        {/if}
                        <RefreshButton class="btn-sm" tooltip={"Refresh"} on:refresh={loadCrons} />
                    {/if}
                </div>
            </div>

            {#if !isLeader && !isLoading}
                <div class="alert alert-warning m-b-sm">
                    <div class="icon">
                        <i class="ri-information-line"></i>
                    </div>
                    <div class="content">
                        <p>
                            This is a follower instance. Cron jobs are only executed on leader instances.
                            Manual cron execution is disabled.
                        </p>
                    </div>
                </div>
            {/if}

            <div class="list list-compact">
                <div class="list-content">
                    {#if isLoading}
                        <div class="list-item list-item-loader">
                            <span class="skeleton-loader skeleton-loader-lg" />
                        </div>
                        <div class="list-item list-item-loader">
                            <span class="skeleton-loader skeleton-loader-lg" />
                        </div>
                        <div class="list-item list-item-loader">
                            <span class="skeleton-loader skeleton-loader-lg" />
                        </div>
                        <div class="list-item list-item-loader">
                            <span class="skeleton-loader skeleton-loader-lg" />
                        </div>
                    {:else}
                        {#each crons as cron (cron.id)}
                            <div class="list-item">
                                <!-- <i class="ri-time-line"></i> -->
                                <div class="content">
                                    <span class="txt">{cron.id}</span>
                                </div>
                                <span class="txt-hint txt-nowrap txt-mono cron-expr m-r-xs">
                                    {cron.expression}
                                </span>
                                <div class="actions">
                                    <button
                                        type="button"
                                        class="btn btn-sm btn-circle btn-hint btn-transparent"
                                        class:btn-loading={isRunning[cron.id]}
                                        disabled={isRunning[cron.id] || !isLeader}
                                        aria-label={isLeader ? "Run" : "Run (disabled - not leader)"}
                                        use:tooltip={isLeader ? "Run" : "Run (disabled - not leader)"}
                                        on:click|preventDefault={() => cronRun(cron.id)}
                                    >
                                        <i class="ri-play-large-line"></i>
                                    </button>
                                </div>
                            </div>
                        {:else}
                            <div class="list-item list-item-placeholder">
                                <span class="txt">No app crons found.</span>
                            </div>
                        {/each}
                    {/if}
                </div>
            </div>

            <p class="txt-hint m-t-xs">
                App cron jobs can be registered only programmatically with
                <a
                    href="{import.meta.env.PB_DOCS_URL}/go-jobs-scheduling/"
                    target="_blank"
                    rel="noopener noreferrer"
                >
                    Go
                </a>
                or
                <a
                    href="{import.meta.env.PB_DOCS_URL}/js-jobs-scheduling/"
                    target="_blank"
                    rel="noopener noreferrer"
                >
                    JavaScript
                </a>.
            </p>
        </div>
    </div>
</PageWrapper>
