function showAddUserModal() { document.getElementById('add-user-modal').classList.remove('hidden'); }

function addUser(e) {
    e.preventDefault();
    const form = document.getElementById('add-user-form');
    const fd = new FormData(form);
    fetch('/users/add', { method: 'POST', body: fd })
        .then(r => r.json())
        .then(d => {
            document.getElementById('add-user-result').innerHTML = d.success
                ? '<div class="text-emerald-600 text-xs">' + d.message + '</div>'
                : '<div class="text-red-600 text-xs">' + (d.error||'Error') + '</div>';
            if (d.success) setTimeout(() => location.reload(), 800);
        });
}

function showEditUserModal(id, username, role) {
    document.getElementById('edit-user-id').value = id;
    document.getElementById('edit-username').value = username;
    document.getElementById('edit-role').value = '';
    document.getElementById('edit-user-modal').classList.remove('hidden');
}

function editUser(e) {
    e.preventDefault();
    const id = document.getElementById('edit-user-id').value;
    const fd = new FormData(document.getElementById('edit-user-form'));
    fetch('/users/' + id + '/edit', { method: 'POST', body: fd })
        .then(r => r.json())
        .then(d => {
            document.getElementById('edit-user-result').innerHTML = d.success
                ? '<div class="text-emerald-600 text-xs">' + (d.message || 'Saved') + '</div>'
                : '<div class="text-red-600 text-xs">' + (d.error||'Error') + '</div>';
            if (d.success) setTimeout(() => location.reload(), 800);
        });
}

function toggleUser(id) {
    if (!confirm('Toggle this user account?')) return;
    fetch('/users/' + id + '/toggle', { method: 'POST' })
        .then(r => r.json())
        .then(d => { showToast(d.message || 'Updated'); if(d.success) location.reload(); });
}

function deleteUser(id) {
    if (!confirm('Delete user? This cannot be undone.')) return;
    fetch('/users/' + id, { method: 'DELETE' })
        .then(r => r.json())
        .then(d => { showToast(d.message || 'Deleted'); if(d.success) location.reload(); });
}

function setUserPassword(id) {
    const pw = prompt('New password (min 8 chars):');
    if (!pw || pw.length < 8) return;
    const fd = new FormData(); fd.append('password', pw);
    fetch('/users/' + id + '/password', { method: 'POST', body: fd })
        .then(r => r.json())
        .then(d => showToast(d.message || 'Password updated'));
}

function kickUser(id) {
    if (!confirm('Kick this user? They will be disconnected.')) return;
    fetch('/users/' + id + '/kick', { method: 'POST' })
        .then(r => r.json())
        .then(d => { showToast(d.message || 'Kicked'); })
        .catch(e => showToast('Error: ' + e, 'error'));
}

function forceLogoutUser(id) {
    if (!confirm('Force logout this user? All their sessions will be invalidated.')) return;
    fetch('/users/' + id + '/force-logout', { method: 'POST' })
        .then(r => r.json())
        .then(d => { showToast(d.message || 'Force logged out'); })
        .catch(e => showToast('Error: ' + e, 'error'));
}

function filterUsers() {
    const q = document.getElementById('user-search').value.toLowerCase();
    document.querySelectorAll('.user-row').forEach(row => {
        const username = row.getAttribute('data-username').toLowerCase();
        const role = row.getAttribute('data-role').toLowerCase();
        row.style.display = (username.includes(q) || role.includes(q)) ? '' : 'none';
    });
}

document.addEventListener('DOMContentLoaded', function() {
    function updateOnlineStatus() {
        fetch('/api/collab/online').then(r => r.json()).then(d => {
            if (!d.success || !d.users) return;
            const online = new Set(d.users.map(u => u.username));
            document.querySelectorAll('[data-online-user]').forEach(el => {
                const username = el.getAttribute('data-online-user');
                const dot = el.querySelector('span:first-child');
                const label = el.querySelector('span:last-child');
                if (online.has(username)) {
                    dot.className = 'w-2 h-2 bg-emerald-500 rounded-full';
                    label.textContent = '在线';
                    label.className = 'text-emerald-600';
                } else {
                    dot.className = 'w-2 h-2 bg-slate-300 rounded-full';
                    label.textContent = '离线';
                    label.className = 'text-slate-400';
                }
            });
        }).catch(() => showToast('在线状态刷新失败', 'error'));
    }
    updateOnlineStatus();
    setInterval(updateOnlineStatus, 10000);
});
