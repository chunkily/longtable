<script lang="ts">
	// The Host's banner, across the top of every page.
	//
	// Fixed rather than in the document flow: the room page is
	// `fixed inset-0` and would paint straight over a block element in
	// the layout. The room's toolbar shifts down while this is up — the
	// same arrangement the reconnect banner already uses — and it carries
	// an opaque background for the same reason that one does, since on
	// the room page what is behind it is a battle map.
	import { onMount } from 'svelte';
	import { hostNotice } from '$lib/host-notice.svelte';
	import { Button } from '$lib/components/ui/button';
	import X from '@lucide/svelte/icons/x';

	onMount(() => {
		hostNotice.load();
	});
</script>

{#if hostNotice.visible}
	<!-- offsetHeight, not clientHeight: this has a bottom border, and
	     clientHeight leaves it out — the map then started one pixel under
	     the banner, which is invisible until you are comparing bounding
	     boxes in a test. -->
	<div
		role="status"
		bind:offsetHeight={hostNotice.measured}
		class="fixed inset-x-0 top-0 z-50 flex items-center gap-3 border-b bg-primary px-3 py-2 text-sm text-primary-foreground shadow-md"
	>
		<p class="min-w-0 flex-1">{hostNotice.text}</p>
		<Button
			variant="ghost"
			size="sm"
			class="h-7 w-7 shrink-0 p-0 hover:bg-primary-foreground/15"
			aria-label="Dismiss this message"
			title="Dismiss this message"
			onclick={() => hostNotice.dismiss()}
		>
			<X class="h-4 w-4" />
		</Button>
	</div>
{/if}
