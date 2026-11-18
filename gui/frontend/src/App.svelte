<script lang="ts">
  // The window: the profiles (click one to use it), editing them, a one-off FPS limit and
  // the settings. Changes from keyboard shortcuts (the CLI) show up by themselves.
  import { onMount } from "svelte";
  import { ModeWatcher } from "mode-watcher";
  import { Events } from "@wailsio/runtime";
  import { Toaster } from "$lib/components/ui/sonner";
  import { toast } from "svelte-sonner";
  import { Button } from "$lib/components/ui/button";
  import { Badge } from "$lib/components/ui/badge";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import { Switch } from "$lib/components/ui/switch";
  import * as Card from "$lib/components/ui/card";
  import * as Alert from "$lib/components/ui/alert";
  import * as Tooltip from "$lib/components/ui/tooltip";
  import Gauge from "@lucide/svelte/icons/gauge";
  import Pencil from "@lucide/svelte/icons/pencil";
  import Plus from "@lucide/svelte/icons/plus";
  import ChevronUp from "@lucide/svelte/icons/chevron-up";
  import ChevronDown from "@lucide/svelte/icons/chevron-down";
  import CircleAlert from "@lucide/svelte/icons/circle-alert";
  import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
  import Check from "@lucide/svelte/icons/check";
  import { ProfileService } from "../bindings/github.com/s3r3nity-labs/mangohud-fps-switch/gui";
  import type { Autostart, Profile, State } from "../bindings/github.com/s3r3nity-labs/mangohud-fps-switch/gui/models";
  import ProfileDialog from "$lib/ProfileDialog.svelte";

  let st = $state<State | null>(null);
  let error = $state("");
  let customFPS = $state("");
  let autostart = $state<Autostart>({ available: false, enabled: false });
  let autostartBusy = $state(false);
  let mangoConfig = $state("");

  // The profile dialog: index -1 is a new profile
  let dialogOpen = $state(false);
  let editIndex = $state(-1);
  let editProfile = $state<Profile>({ name: "", fpsLimit: "", label: "", settings: [] });

  let profiles = $derived(st?.profiles ?? []);

  const message = (e: unknown) => (e instanceof Error ? e.message : String(e));

  function show(s: State) {
    st = s;
    mangoConfig = s.mangoHudConfig;
    error = "";
  }

  async function load() {
    try {
      show(await ProfileService.State());
    } catch (e) {
      error = message(e);
    }
  }

  async function use(i: number) {
    try {
      show(await ProfileService.UseProfile(i));
    } catch (e) {
      toast.error(message(e));
    }
  }

  async function useCustom(e: Event) {
    e.preventDefault();
    if (customFPS.trim() === "") return;
    try {
      show(await ProfileService.UseFPS(customFPS));
      customFPS = "";
    } catch (e) {
      toast.error(message(e));
    }
  }

  // save writes everything to config.toml; false if it didn't work (the reason is shown).
  async function save(list: Profile[], opts?: { notify?: boolean; mangoHudConfig?: string }): Promise<boolean> {
    if (!st) return false;
    try {
      show(
        await ProfileService.Save({
          profiles: list,
          notify: opts?.notify ?? st.notify,
          mangoHudConfig: opts?.mangoHudConfig ?? st.mangoHudConfig,
        }),
      );
      return true;
    } catch (e) {
      toast.error(message(e));
      return false;
    }
  }

  function edit(i: number) {
    editIndex = i;
    editProfile = i < 0 ? { name: "", fpsLimit: "", label: "", settings: [] } : profiles[i];
    dialogOpen = true;
  }

  async function saveProfile(p: Profile) {
    const list = [...profiles];
    if (editIndex < 0) list.push(p);
    else list[editIndex] = p;
    return save(list);
  }

  async function remove() {
    const removed = profiles[editIndex];
    const at = editIndex;
    const list = profiles.filter((_, i) => i !== at);
    if (await save(list)) {
      toast(`Deleted “${removed.name}”`, {
        action: {
          label: "Undo",
          onClick: () => {
            const back = [...profiles];
            back.splice(at, 0, removed);
            save(back);
          },
        },
      });
    }
  }

  async function move(i: number, by: number) {
    const list = [...profiles];
    const [p] = list.splice(i, 1);
    list.splice(i + by, 0, p);
    await save(list);
  }

  async function saveMangoConfig() {
    if (st && mangoConfig.trim() !== st.mangoHudConfig) await save(profiles, { mangoHudConfig: mangoConfig.trim() });
  }

  async function setAutostart(on: boolean) {
    autostartBusy = true;
    try {
      autostart = await ProfileService.SetAutostart(on);
    } catch (e) {
      toast.error(message(e));
      autostart = await ProfileService.AutostartState();
    } finally {
      autostartBusy = false;
    }
  }

  onMount(() => {
    load();
    ProfileService.AutostartState().then((a) => (autostart = a));
    // A shortcut, the CLI or an editor changed a config file
    return Events.On("state-changed", load);
  });
</script>

<ModeWatcher />
<Toaster richColors position="bottom-center" />

<Tooltip.Provider>
  <main class="mx-auto flex max-w-2xl flex-col gap-6 p-6">
    <header class="flex items-center justify-between gap-4">
      <div class="flex items-center gap-3">
        <Gauge class="text-primary size-7" />
        <h1 class="text-xl font-semibold">MangoHud FPS Switch</h1>
      </div>
      {#if st?.fpsLabel}
        <Badge variant="secondary" class="text-sm">Now: {st.fpsLabel}</Badge>
      {/if}
    </header>

    {#if error}
      <Alert.Root variant="destructive">
        <CircleAlert />
        <Alert.Title>Can't read the profiles</Alert.Title>
        <Alert.Description class="whitespace-pre-line">{error}</Alert.Description>
      </Alert.Root>
    {/if}

    {#if st?.mangoHudMissing}
      <Alert.Root variant="destructive">
        <CircleAlert />
        <Alert.Title>MangoHud has no config file yet</Alert.Title>
        <Alert.Description>
          <p>Expected at <code class="break-all">{st.mangoHudPath}</code>. Create it, e.g. with GOverlay or from
            MangoHud's example config, or choose another file under Settings below.</p>
        </Alert.Description>
      </Alert.Root>
    {:else if st?.problem}
      <Alert.Root variant="destructive">
        <CircleAlert />
        <Alert.Title>Can't read MangoHud's config</Alert.Title>
        <Alert.Description class="whitespace-pre-line">{st.problem}</Alert.Description>
      </Alert.Root>
    {/if}

    {#if st && (st.perGame ?? []).length > 0}
      <Alert.Root>
        <TriangleAlert class="text-warning" />
        <Alert.Title>Some games have their own MangoHud config</Alert.Title>
        <Alert.Description>
          <p>For these games, the profiles here don't apply; their own file wins:</p>
          <ul class="list-inside list-disc">
            {#each st.perGame ?? [] as f (f)}<li class="break-all"><code>{f.split("/").pop()}</code></li>{/each}
          </ul>
        </Alert.Description>
      </Alert.Root>
    {/if}
    {#if st?.envOverride}
      <Alert.Root>
        <TriangleAlert class="text-warning" />
        <Alert.Title>MANGOHUD_CONFIG sets an FPS limit</Alert.Title>
        <Alert.Description>Games started with it use that limit instead of the profiles.</Alert.Description>
      </Alert.Root>
    {/if}

    {#if st}
      <section class="flex flex-col gap-3">
        <div class="flex items-end justify-between gap-4">
          <div>
            <h2 class="font-medium">Profiles</h2>
            <p class="text-muted-foreground text-sm">
              Click one to use it. The number is what <code>mangohud-fps-switch profile</code> and your shortcuts use.
            </p>
          </div>
          <Button variant="outline" size="sm" onclick={() => edit(-1)}><Plus /> Add</Button>
        </div>

        <ul class="flex flex-col gap-2">
          {#each profiles as p, i (p.name)}
            {@const active = i === st.active}
            <li>
              <Card.Root
                class={"flex-row items-center gap-3 px-4 py-3 transition-colors " +
                  (active ? "border-primary bg-primary/5" : "hover:bg-accent/50")}
              >
                <button
                  class="flex min-w-0 flex-1 cursor-pointer items-center gap-3 text-left"
                  onclick={() => use(i)}
                  aria-pressed={active}
                >
                  <span
                    class={"flex size-8 shrink-0 items-center justify-center rounded-md font-mono text-sm " +
                      (active ? "bg-primary text-primary-foreground" : "bg-muted")}>{i}</span>
                  <span class="min-w-0 flex-1">
                    <span class="flex items-center gap-2">
                      <span class="truncate font-medium">{p.name}</span>
                      {#if active}<Check class="text-primary size-4 shrink-0" />{/if}
                    </span>
                    <span class="text-muted-foreground block truncate text-sm">
                      {p.label}{#each p.settings ?? [] as s (s.key)} · <code>{s.key}={s.value}</code>{/each}
                    </span>
                  </span>
                </button>
                <div class="flex shrink-0 items-center">
                  <Button variant="ghost" size="icon" aria-label="Move up" disabled={i === 0}
                    onclick={() => move(i, -1)}><ChevronUp /></Button>
                  <Button variant="ghost" size="icon" aria-label="Move down" disabled={i === profiles.length - 1}
                    onclick={() => move(i, 1)}><ChevronDown /></Button>
                  <Button variant="ghost" size="icon" aria-label="Edit" onclick={() => edit(i)}><Pencil /></Button>
                </div>
              </Card.Root>
            </li>
          {/each}
        </ul>
      </section>

      <section class="flex flex-col gap-2">
        <h2 class="font-medium">Just this once</h2>
        <form class="flex gap-2" onsubmit={useCustom}>
          <Input bind:value={customFPS} placeholder="FPS limit, e.g. 72" inputmode="decimal" class="max-w-48"
            aria-label="FPS limit" />
          <Button type="submit" variant="secondary" disabled={customFPS.trim() === ""}>Set</Button>
        </form>
      </section>

      <section class="flex flex-col gap-4">
        <h2 class="font-medium">Settings</h2>
        <div class="flex items-center justify-between gap-4">
          <Label for="notify" class="flex flex-col items-start gap-1">
            <span>Notification on every change</span>
            <span class="text-muted-foreground text-sm font-normal">For keyboard shortcuts; the window doesn't need one.</span>
          </Label>
          <Switch id="notify" checked={st.notify} onCheckedChange={(v) => save(profiles, { notify: v })} />
        </div>
        {#if autostart.available}
          <div class="flex items-center justify-between gap-4">
            <Label for="autostart" class="flex flex-col items-start gap-1">
              <span>Start with the desktop</span>
              <span class="text-muted-foreground text-sm font-normal">In the tray, without opening the window.</span>
            </Label>
            <Switch id="autostart" checked={autostart.enabled} disabled={autostartBusy}
              onCheckedChange={(v) => setAutostart(v)} />
          </div>
        {/if}
        <div class="grid gap-2">
          <Label for="mango">MangoHud config</Label>
          <Input id="mango" bind:value={mangoConfig} placeholder="~/.config/MangoHud/MangoHud.conf (default)"
            class="font-mono" onblur={saveMangoConfig}
            onkeydown={(e) => e.key === "Enter" && saveMangoConfig()} />
          <p class="text-muted-foreground text-sm break-all">
            Changing: <code>{st.mangoHudPath}</code> · Profiles: <code>{st.configPath}</code>
          </p>
        </div>
      </section>
    {/if}
  </main>
</Tooltip.Provider>

<ProfileDialog
  bind:open={dialogOpen}
  profile={editProfile}
  isNew={editIndex < 0}
  otherNames={profiles.filter((_, i) => i !== editIndex).map((p) => p.name)}
  canDelete={profiles.length > 1}
  onsave={saveProfile}
  ondelete={remove}
/>
