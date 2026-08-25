package admin

import "html/template"

// page is the settings screen.
//
// Server-rendered HTML and a plain form, rather than a route in the
// SvelteKit app the game is served from. Three reasons, and the last is
// the one that decided it:
//
//   - The admin surface is a different listener on a different port.
//     Serving the SPA from it would ship the admin bundle to every
//     Player as well, for a screen none of them can open.
//   - A settings form needs no interactivity worth a build step.
//   - Validation belongs on the server anyway, because the rules live in
//     internal/config's registry. A form that posts and comes back with
//     the message beside the field is the *same* check `longtable config
//     set` makes, rather than a second implementation in TypeScript that
//     drifts from it.
//
// The values interpolated here are a Host's own settings, and
// html/template escapes each for the context it lands in — attribute,
// text or href — which is the other reason not to build strings.
var page = template.Must(template.New("settings").Parse(`<!doctype html>
<html lang="en">
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Longtable admin</title>
<style>
  :root {
    color-scheme: light dark;
    --edge: #d4d4d8;
    --muted: #52525b;
    --bad: #b91c1c;
    --warn: #92400e;
    --warn-bg: #fef3c7;
    --good: #166534;
    --good-bg: #dcfce7;
    --field: #ffffff;
    --ink: #18181b;
    --page: #fafafa;
  }
  @media (prefers-color-scheme: dark) {
    :root {
      --edge: #3f3f46; --muted: #a1a1aa; --bad: #fca5a5;
      --warn: #fcd34d; --warn-bg: #292524;
      --good: #86efac; --good-bg: #14532d;
      --field: #18181b; --ink: #fafafa; --page: #09090b;
    }
  }
  body {
    font-family: system-ui, -apple-system, sans-serif;
    line-height: 1.5; margin: 0; padding: 2rem 1rem 4rem;
    background: var(--page); color: var(--ink);
  }
  main { max-width: 44rem; margin: 0 auto; }
  h1 { margin: 0 0 .25rem; font-size: 1.5rem; }
  .sub { color: var(--muted); margin: 0 0 2rem; }
  .sub a { color: inherit; }
  .note { padding: .75rem 1rem; border-radius: 6px; margin: 0 0 1.5rem; }
  .note.good { background: var(--good-bg); color: var(--good); }
  .note.bad { background: var(--warn-bg); color: var(--bad); }
  .note.warn { background: var(--warn-bg); color: var(--warn); }
  .setting { padding: 1.25rem 0; border-top: 1px solid var(--edge); }
  label { display: block; font-weight: 600; font-family: ui-monospace, monospace; }
  .summary { color: var(--muted); margin: .35rem 0 .6rem; font-size: .925rem; }
  input {
    width: 100%; box-sizing: border-box; padding: .5rem .6rem;
    font: inherit; font-family: ui-monospace, monospace;
    border: 1px solid var(--edge); border-radius: 6px;
    background: var(--field); color: var(--ink);
  }
  input[aria-invalid="true"] { border-color: var(--bad); border-width: 2px; }
  .meta { color: var(--muted); font-size: .85rem; margin-top: .4rem; }
  .field-error { color: var(--bad); font-size: .9rem; margin-top: .4rem; font-weight: 600; }
  .field-warning { color: var(--warn); font-size: .9rem; margin-top: .4rem; }
  .actions { margin-top: 2rem; padding-top: 1.5rem; border-top: 1px solid var(--edge); }
  button {
    font: inherit; font-weight: 600; padding: .55rem 1.1rem;
    border: 1px solid var(--edge); border-radius: 6px; cursor: pointer;
    background: var(--ink); color: var(--page);
  }
</style>

<main>
  <h1>Longtable settings</h1>
  <p class="sub">Longtable itself is at <a href="{{.GameURL}}">{{.GameURL}}</a> — that's the address to share.</p>

  {{if .Saved}}
    <p class="note good">
      Saved.{{if .Restart}} Restart Longtable for the changes to take effect.{{end}}
    </p>
  {{end}}

  {{if .Problem}}
    <p class="note bad">{{.Problem}}</p>
  {{end}}

  <form method="post" action="/">
    {{range .Settings}}
      <div class="setting">
        <label for="{{.Key}}">{{.Key}}</label>
        <p class="summary">{{.Summary}}</p>
        <input id="{{.Key}}" name="{{.Key}}" value="{{.Value}}"
               spellcheck="false" autocapitalize="off" autocomplete="off"
               {{if .Error}}aria-invalid="true" aria-describedby="{{.Key}}-error"{{end}}>
        {{if .Error}}
          <p class="field-error" id="{{.Key}}-error">{{.Error}}</p>
        {{end}}
        {{if .Warning}}
          <p class="field-warning">{{.Warning}}</p>
        {{end}}
        <p class="meta">
          Default: <code>{{if .Default}}{{.Default}}{{else}}nothing{{end}}</code>{{if .NeedsRestart}} · needs a restart{{end}}
        </p>
      </div>
    {{end}}

    <div class="actions">
      <button type="submit">Save changes</button>
    </div>
  </form>
</main>
`))
