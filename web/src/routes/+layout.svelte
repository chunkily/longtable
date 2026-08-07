<script lang="ts">
	import './layout.css';
	import favicon from '$lib/assets/favicon.svg';
	import { Toaster } from '$lib/components/ui/sonner';

	let { children } = $props();
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<!-- A page-level <svelte:head><title> further down the tree replaces
	     this one rather than duplicating it, so this is a real fallback,
	     not a tag that would linger next to a page's own title. -->
	<title>Longtable</title>
</svelte:head>
<Toaster />
{@render children()}
