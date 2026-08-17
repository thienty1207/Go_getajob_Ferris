<script lang="ts">
	let query = $state('');
	let role = $state('');
	let status = $state('');
</script>

<svelte:head>
	<title>Users — Admin Go get a job ferris</title>
	<meta name="description" content="Quản lý user của Go get a job ferris." />
</svelte:head>

<section class="admin-page-heading">
	<div><h2>Users.</h2></div>
</section>

<section class="admin-panel">
	<div class="admin-panel-header">
		<h3>User list</h3>
	</div>
	<form class="admin-filter-bar" onsubmit={(event) => event.preventDefault()}>
		<div class="admin-field">
			<label for="user-search">User</label>
			<input id="user-search" bind:value={query} placeholder="Email hoặc user id" autocomplete="off" />
		</div>
		<div class="admin-field">
			<label for="user-role">Role</label>
			<select id="user-role" bind:value={role}>
				<option value="">Tất cả role</option>
				<option value="client">Client</option>
				<option value="admin">Admin</option>
			</select>
		</div>
		<div class="admin-field">
			<label for="user-status">Trạng thái</label>
			<select id="user-status" bind:value={status}>
				<option value="">Tất cả trạng thái</option>
				<option value="active">Active</option>
				<option value="disabled">Disabled</option>
			</select>
		</div>
	</form>
	<div class="admin-empty-state">
		<span class="admin-empty-state-mark" aria-hidden="true">◎</span>
		<strong>Chưa có user nào</strong>
	</div>
</section>
