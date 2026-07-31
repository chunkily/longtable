<script lang="ts">
	// Picking art for a scene or a token: the room's library to choose
	// from, with uploading as one option among them rather than the only
	// way in. An upload joins the library on the way past, so the second
	// time you need the same goblin it's already here.
	import { toast } from 'svelte-sonner';
	import { assetUrl, listRoomAssets, uploadAsset, type Asset } from '$lib/api';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';

	let {
		roomSlug,
		sessionToken,
		selectedId = $bindable(null),
		idPrefix = 'asset',
		emptyHint = 'Nothing in the library yet — upload an image to get started.'
	}: {
		roomSlug: string;
		sessionToken: string;
		/**
		 * The chosen asset, or null for "no image". Bindable so the
		 * surrounding dialog can read it on submit and reset it afterwards.
		 */
		selectedId?: string | null;
		/** Distinguishes the input ids when two pickers share a page. */
		idPrefix?: string;
		emptyHint?: string;
	} = $props();

	let library = $state<Asset[]>([]);
	let loading = $state(true);
	let uploading = $state(false);
	let attribution = $state('');
	let fileInput = $state<HTMLInputElement | null>(null);

	// Loaded once when the picker mounts rather than on every open: the
	// library changes when someone uploads, and an upload goes through
	// this component, so it can keep its own list current.
	$effect(() => {
		void refresh();
	});

	async function refresh() {
		loading = true;
		try {
			library = await listRoomAssets(roomSlug, sessionToken);
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'failed to load the asset library');
		} finally {
			loading = false;
		}
	}

	async function handleUpload(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		if (!file) return;

		uploading = true;
		try {
			const asset = await uploadAsset(roomSlug, sessionToken, file, attribution);
			// Selecting it immediately is the point of uploading, and putting
			// it at the front matches the server's newest-first order without
			// a second round trip.
			library = [asset, ...library.filter((a) => a.id !== asset.id)];
			selectedId = asset.id;
			attribution = '';
			if (asset.flattened) {
				toast.info('Animated images are stored as a still picture — kept the first frame.');
			}
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'upload failed');
		} finally {
			uploading = false;
			// Clearing the input means picking the same file again still
			// fires a change event, which matters after a failed upload.
			input.value = '';
		}
	}

	function toggle(id: string) {
		selectedId = selectedId === id ? null : id;
	}
</script>

<div class="flex flex-col gap-3">
	<div class="flex flex-col gap-2">
		<Label for="{idPrefix}-attribution">Attribution or licence (optional)</Label>
		<Input
			id="{idPrefix}-attribution"
			bind:value={attribution}
			placeholder="e.g. by Alice, CC-BY"
			autocomplete="off"
		/>
		<p class="text-xs text-muted-foreground">
			Applied to the next image you upload, and shown to everyone in the room.
		</p>
	</div>

	<div class="flex items-center gap-2">
		<Button
			type="button"
			variant="outline"
			size="sm"
			disabled={uploading}
			onclick={() => fileInput?.click()}
		>
			{uploading ? 'Uploading…' : 'Upload new image'}
		</Button>
		{#if selectedId}
			<Button type="button" variant="ghost" size="sm" onclick={() => (selectedId = null)}>
				Clear selection
			</Button>
		{/if}
	</div>
	<!-- Hidden because the native control can't be styled to match, and
	     it's driven by the button above. -->
	<input
		bind:this={fileInput}
		type="file"
		accept="image/png,image/jpeg,image/webp,image/gif"
		class="hidden"
		aria-label="Upload an image"
		onchange={handleUpload}
	/>

	{#if loading}
		<p class="text-sm text-muted-foreground">Loading library…</p>
	{:else if library.length === 0}
		<p class="text-sm text-muted-foreground">{emptyHint}</p>
	{:else}
		<ul class="grid max-h-64 grid-cols-3 gap-2 overflow-y-auto sm:grid-cols-4">
			{#each library as asset (asset.id)}
				<li>
					<button
						type="button"
						aria-pressed={selectedId === asset.id}
						title={asset.attribution ? `${asset.filename} — ${asset.attribution}` : asset.filename}
						class={[
							'flex w-full flex-col gap-1 rounded-md border p-1 text-left',
							selectedId === asset.id && 'outline-2 outline-offset-2 outline-sky-400'
						]}
						onclick={() => toggle(asset.id)}
					>
						<img
							src={assetUrl(asset.id)}
							alt={asset.filename}
							loading="lazy"
							class="h-16 w-full rounded object-cover"
						/>
						<span class="truncate text-xs">{asset.filename}</span>
						{#if asset.attribution}
							<span class="truncate text-[10px] text-muted-foreground">{asset.attribution}</span>
						{/if}
					</button>
				</li>
			{/each}
		</ul>
	{/if}
</div>
