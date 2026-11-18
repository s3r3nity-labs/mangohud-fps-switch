<script lang="ts">
  // Add or edit one profile: name, FPS limit and optional extra MangoHud settings.
  import * as Dialog from "$lib/components/ui/dialog";
  import { Button } from "$lib/components/ui/button";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";
  import Plus from "@lucide/svelte/icons/plus";
  import X from "@lucide/svelte/icons/x";
  import Trash2 from "@lucide/svelte/icons/trash-2";
  import { ProfileService } from "../../bindings/github.com/s3r3nity-labs/mangohud-fps-switch/gui";
  import type { Profile, Setting } from "../../bindings/github.com/s3r3nity-labs/mangohud-fps-switch/gui/models";

  let {
    open = $bindable(false),
    profile,
    isNew,
    otherNames,
    canDelete,
    onsave,
    ondelete,
  }: {
    open: boolean;
    profile: Profile;
    isNew: boolean;
    otherNames: string[];
    canDelete: boolean;
    onsave: (p: Profile) => Promise<boolean>;
    ondelete: () => void;
  } = $props();

  // The form works on a copy, so Cancel leaves the profile as it was.
  let name = $state("");
  let fps = $state("");
  let settings = $state<Setting[]>([]);
  let fpsError = $state("");
  let saving = $state(false);

  $effect(() => {
    if (open) {
      name = profile.name;
      fps = profile.fpsLimit;
      settings = (profile.settings ?? []).map((s) => ({ ...s }));
      fpsError = "";
    }
  });

  let nameError = $derived.by(() => {
    const n = name.trim();
    if (n === "") return "Give the profile a name.";
    if (otherNames.some((o) => o.toLowerCase() === n.toLowerCase())) return "Another profile has this name.";
    return "";
  });
  const keyPattern = /^[a-z0-9_]+$/;
  let settingsError = $derived.by(() => {
    for (const s of settings) {
      const k = s.key.trim();
      if (k === "" && s.value.trim() === "") continue;
      if (!keyPattern.test(k)) return `"${k}" isn't a MangoHud setting name (lowercase, digits, _).`;
      if (k === "fps_limit") return "Set the FPS limit above, not as a setting.";
      if (/[#\n]/.test(s.value)) return `The value of ${k} can't contain #.`;
    }
    const keys = settings.map((s) => s.key.trim()).filter((k) => k !== "");
    if (new Set(keys).size !== keys.length) return "A setting is listed twice.";
    return "";
  });

  async function checkFPS() {
    fpsError = await ProfileService.CheckFPS(fps);
  }

  async function save(e: Event) {
    e.preventDefault();
    await checkFPS();
    if (nameError || fpsError || settingsError) return;
    saving = true;
    const ok = await onsave({
      name: name.trim(),
      fpsLimit: fps.trim(),
      label: "",
      settings: settings.filter((s) => s.key.trim() !== ""),
    });
    saving = false;
    if (ok) open = false;
  }
</script>

<Dialog.Root bind:open>
  <Dialog.Content class="sm:max-w-lg">
    <form onsubmit={save} class="grid gap-5">
      <Dialog.Header>
        <Dialog.Title>{isNew ? "New profile" : `Edit “${profile.name}”`}</Dialog.Title>
        <Dialog.Description>
          Used right away by running games when you switch to it.
        </Dialog.Description>
      </Dialog.Header>

      <div class="grid gap-2">
        <Label for="name">Name</Label>
        <Input id="name" bind:value={name} placeholder="e.g. Battery" aria-invalid={nameError !== ""} />
        {#if nameError && name !== ""}<p class="text-destructive text-sm">{nameError}</p>{/if}
      </div>

      <div class="grid gap-2">
        <Label for="fps">FPS limit</Label>
        <Input id="fps" bind:value={fps} onblur={checkFPS} placeholder="60" inputmode="decimal"
          aria-invalid={fpsError !== ""} />
        {#if fpsError}
          <p class="text-destructive text-sm">{fpsError}</p>
        {:else}
          <p class="text-muted-foreground text-sm">
            0 is unlimited. A list like 60,90,0 lets MangoHud's own key (Shift+F1) switch between them in-game.
          </p>
        {/if}
      </div>

      <div class="grid gap-2">
        <Label>More MangoHud settings <span class="text-muted-foreground font-normal">(optional)</span></Label>
        {#each settings as s, i (i)}
          <div class="flex gap-2">
            <Input bind:value={s.key} placeholder="fps_limit_method" class="font-mono" aria-label="Setting" />
            <Input bind:value={s.value} placeholder="early" class="font-mono" aria-label="Value" />
            <Button variant="ghost" size="icon" type="button" aria-label="Remove setting"
              onclick={() => settings.splice(i, 1)}><X /></Button>
          </div>
        {/each}
        {#if settingsError}<p class="text-destructive text-sm">{settingsError}</p>{/if}
        <div class="flex items-center gap-3">
          <Button variant="outline" size="sm" type="button" onclick={() => settings.push({ key: "", value: "" })}>
            <Plus /> Add setting
          </Button>
          <span class="text-muted-foreground text-sm">true/false become MangoHud's 1/0.</span>
        </div>
      </div>

      <Dialog.Footer class="gap-2 sm:justify-between">
        {#if !isNew && canDelete}
          <Button variant="ghost" type="button" class="text-destructive" onclick={() => { open = false; ondelete(); }}>
            <Trash2 /> Delete
          </Button>
        {:else}
          <span></span>
        {/if}
        <div class="flex gap-2">
          <Button variant="outline" type="button" onclick={() => (open = false)}>Cancel</Button>
          <Button type="submit" disabled={saving || nameError !== "" || settingsError !== ""}>
            {isNew ? "Add" : "Save"}
          </Button>
        </div>
      </Dialog.Footer>
    </form>
  </Dialog.Content>
</Dialog.Root>
