<script lang="ts">
	import './layout.css';
	import favicon from '$lib/assets/favicon.svg';
	import { Toaster } from '$lib/components/ui/sonner';
	import HostNotice from '$lib/components/host-notice.svelte';
	import { hostNotice } from '$lib/host-notice.svelte';

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
<!-- Above every page rather than inside one: it is the Host's, not a
     room's, and it is shown to people who have never joined anything.

     The banner is `fixed`, so ordinary pages are padded out from under
     it here. The room page is `fixed inset-0` and ignores this padding
     entirely — it reads the same height itself. -->
<HostNotice />
<div style="padding-top: {hostNotice.height}px">{@render children()}</div>
