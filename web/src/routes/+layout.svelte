<script lang="ts">
	import './layout.css';
	import { ModeWatcher } from 'mode-watcher';
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
<!-- Puts the `dark` class on <html> and keeps it in step: with the
     stored choice when there is one, and with the OS setting live when
     there isn't. It writes the class *and* reads the preference, so it
     has to be above every page rather than on any one of them.

     `modeStorageKey` is the same string the boot script in app.html
     reads. Renamed off the library's default so this browser's keys are
     all `longtable:` — mode-watcher migrates the old key's value across
     and deletes it, so nobody loses a choice they'd already made.

     Head-script injection is off because it does nothing here: the
     library adds its FOUC script through <svelte:head>, and a script
     inserted that way never executes. app.html carries the real one. -->
<ModeWatcher modeStorageKey="longtable:theme" disableHeadScriptInjection />
<Toaster />
<!-- Above every page rather than inside one: it is the Host's, not a
     room's, and it is shown to people who have never joined anything.

     The banner is `fixed`, so ordinary pages are padded out from under
     it here. The room page is `fixed inset-0` and ignores this padding
     entirely — it reads the same height itself. -->
<HostNotice />
<div style="padding-top: {hostNotice.height}px">{@render children()}</div>
