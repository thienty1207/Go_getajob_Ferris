<script lang="ts">
	let user = $state('');
	let role = $state('');
</script>

<svelte:head>
	<title>CV Profiles — Admin Go get a job ferris</title>
	<meta name="description" content="Quản lý CV profiles của Go get a job ferris." />
</svelte:head>

<section class="admin-page-heading">
	<div><h2>CV profiles.</h2></div>
</section>

<section class="admin-panel">
	<form class="admin-filter-bar cv-filter-bar" onsubmit={(event) => event.preventDefault()}>
		<div class="admin-field">
			<label for="cv-user">User</label>
			<input id="cv-user" bind:value={user} placeholder="User" autocomplete="off" />
		</div>
		<div class="admin-field">
			<label for="cv-role">Role</label>
			<input id="cv-role" bind:value={role} placeholder="Role" autocomplete="off" />
		</div>
	</form>
</section>

<section class="admin-panel admin-link-list-panel">
	<div class="admin-panel-header">
		<h3>CV list</h3>
	</div>
	<div class="admin-empty-state">
		<span class="admin-empty-state-mark" aria-hidden="true">▤</span>
		<strong>Chưa có CV nào</strong>
	</div>
</section>
