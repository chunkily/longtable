<script lang="ts">
	// How anyone else gets to this table, in the two forms it travels in:
	// the six characters someone reads out, and the address someone pastes.
	//
	// Both are readonly inputs rather than text, because selecting an
	// <input> is one click plus Ctrl-A and selecting a run of text in a
	// dialog is a drag people miss. There is no copy button, and that is a
	// decision — `navigator.clipboard` exists only in a secure context and
	// every Player here is on `http://192.168.x.x:8080`, so a button would
	// work for the GM on localhost and fail for the table. The whole
	// argument, including the implementation that was written and deleted,
	// is in planning/backlog/share-room-code-from-room.md.
	import { onMount } from 'svelte';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Dialog from '$lib/components/ui/dialog';

	let { slug, open = $bindable(false) }: { slug: string; open?: boolean } = $props();

	// Read on mount rather than at module scope: there is no window while
	// prerendering, and this component is inside the room page's bundle.
	let origin = $state('');
	onMount(() => (origin = window.location.origin));

	const url = $derived(origin ? `${origin}/r/${slug}` : '');

	/** One click fills the selection, so Ctrl-C is the only other step. */
	function selectAll(event: Event) {
		(event.currentTarget as HTMLInputElement).select();
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content>
		<Dialog.Header>
			<Dialog.Title>Room code</Dialog.Title>
			<Dialog.Description>Send either of these to invite someone.</Dialog.Description>
		</Dialog.Header>

		<div class="flex flex-col gap-2">
			<Label for="share-code">Code</Label>
			<!-- `md:text-3xl` as well as `text-3xl`, and it is doing real
			     work: the base Input carries `md:text-sm`, which otherwise
			     wins from 768px up and renders this at 14px on every desktop.
			     The same trap is on the home page's code box, and it was
			     reintroduced here within a day. The indent cancels the
			     trailing letter-space that tracking adds after the last
			     character, which pushes centred text visibly left. -->
			<Input
				id="share-code"
				value={slug}
				readonly
				onfocus={selectAll}
				onclick={selectAll}
				class="h-auto py-3 text-center indent-[0.35em] font-mono text-3xl tracking-[0.35em] md:text-3xl"
			/>
		</div>

		<div class="flex flex-col gap-2">
			<Label for="share-url">Link</Label>
			<!-- Whatever address this browser is on. For a GM on localhost
			     that is localhost, which is no use to anyone else — the
			     server prints its LAN addresses at startup and that is still
			     the only place they exist. Worth knowing before trusting
			     this box over the code above it. -->
			<Input
				id="share-url"
				value={url}
				readonly
				onfocus={selectAll}
				onclick={selectAll}
				class="font-mono"
			/>
		</div>
	</Dialog.Content>
</Dialog.Root>
