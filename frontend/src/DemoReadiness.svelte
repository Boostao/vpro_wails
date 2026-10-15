<script lang="ts">
  const reports = [
    { name: 'Long Environment', enabled: import.meta.env.VITE_LONG_ENVIRONMENT_REPORT === 'true' },
    { name: 'Long Vegetation', enabled: import.meta.env.VITE_LONG_VEGETATION_REPORT === 'true' },
    { name: 'Summary Environment', enabled: import.meta.env.VITE_SITE_UNIT_SUMMARY === 'true' },
    { name: 'Lifeform Summary', enabled: import.meta.env.VITE_LIFEFORM_SUMMARY === 'true' },
    { name: 'Summary Vegetation', enabled: import.meta.env.VITE_SUMMARY_VEGETATION === 'true' },
  ];
</script>

<section class="demo-readiness" data-demo-readiness aria-labelledby="demo-readiness-title">
  <h2 id="demo-readiness-title">Experimental desktop migration</h2>
  <p>Browse projects and the mapped FS882 editor, with offline SQLite data and explicit
    project, Site Unit and hierarchy selection. Use disposable projects for writes.</p>
  <h3>Report availability in this build</h3>
  <ul>
    {#each reports as report (report.name)}
      <li data-demo-report={report.name}><strong>{report.name}</strong>
        <span>{report.enabled ? 'Enabled in this build' : 'Not enabled in this build'}</span></li>
    {/each}
  </ul>
  <p>Enabled workflows still check their data, context and supported options. Disabled
    menus are not a promise that every Access feature is ready.</p>
  <h3>Not yet a complete Access replacement</h3>
  <p>Remaining work includes full linked-form callbacks and calculations, complete combined
    and formatted Summary output, printing, hierarchy editing, project creation/import and
    merge/compare administration. Herbarium, broader standards compliance, TurboVeg and
    UI-free R automation also remain separate unfinished workflows.</p>
</section>

<style>
  .demo-readiness { display: grid; gap: .75rem; min-width: 0; margin-top: 1.5rem; padding: 1rem; border: 1px solid #d1d5db; border-radius: .5rem; overflow-wrap: anywhere; }
  h2, h3, p { margin: 0; }
  h2 { font-size: 1.1rem; }
  h3 { font-size: .95rem; }
  p { line-height: 1.5; }
  ul { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 16rem), 1fr)); gap: .75rem; margin: 0; padding: 0; list-style: none; }
  li { display: grid; gap: .25rem; min-width: 0; }
  li span { color: #57534e; }
</style>
