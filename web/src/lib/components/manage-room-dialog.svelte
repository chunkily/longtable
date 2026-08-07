<script lang="ts">
	// The home for GM-level room settings. It is deliberately empty right
	// now: the three things that belong here — room privacy, the token
	// ownership lock, and deleting the room — are each their own open
	// backlog item, and this exists so the next one has somewhere to land
	// rather than inventing a second place to put room settings.
	//
	// Shown rather than hidden while empty, because the menu entry is part
	// of the layout the design session settled and a menu that grows a new
	// item later is harder to notice than one whose contents fill in.
	import * as Dialog from '$lib/components/ui/dialog';

	let { open = $bindable(false) }: { open?: boolean } = $props();
</script>

<Dialog.Root bind:open>
	<Dialog.Content>
		<Dialog.Header>
			<Dialog.Title>Manage room</Dialog.Title>
			<Dialog.Description
				>Settings for the room itself, rather than for this scene.</Dialog.Description
			>
		</Dialog.Header>
		<p class="text-sm text-muted-foreground">
			Nothing to configure yet. Room privacy, locking a token to its owner, and deleting the room
			will live here.
		</p>
	</Dialog.Content>
</Dialog.Root>
