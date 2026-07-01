function showNewRuleModal() { document.getElementById('rule-modal').classList.remove('hidden'); }
function closeRuleModal() { document.getElementById('rule-modal').classList.add('hidden'); }
function showNewWebhookModal() { document.getElementById('webhook-modal').classList.remove('hidden'); }
function closeWebhookModal() { document.getElementById('webhook-modal').classList.add('hidden'); }

function saveRule(btn) {
    const name = document.getElementById('rule-name').value;
    const eventType = document.getElementById('rule-event').value;
    const condition = document.getElementById('rule-condition').value;
    const command = document.getElementById('rule-command').value;
    const webhook = document.getElementById('rule-webhook').value;
    if (!name) return showToast(__('Please enter a rule name'), 'error');
    const rule = {
        id: 'rule_' + Date.now(),
        name: name,
        enabled: true,
        event_type: eventType,
        conditions: condition ? [{field: 'agent.hostname', operator: 'contains', value: condition}] : [],
        actions: []
    };
    if (command) rule.actions.push({type: 'command', params: JSON.stringify({command: command})});
    if (webhook) rule.actions.push({type: 'webhook', params: JSON.stringify({url: webhook, method: 'POST'})});
    withLoading(btn, apiFetch('/api/automation/rules', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify(rule)})
        .then(d => { if (d.success) { showToast(__('Rule saved'), 'success'); location.reload(); } else showToast(d.error, 'error'); })
        .catch(e => showToast(__tf('Failed to save rule: {0}', e.message), 'error')));
}

function deleteRule(id) {
    if (!confirm(__('Delete this rule?'))) return;
    apiFetch('/api/automation/rules/' + id, {method: 'DELETE'})
        .then(d => { if (d.success) { showToast(__('Deleted'), 'success'); location.reload(); } })
        .catch(e => showToast(__tf('Failed to delete: {0}', e.message), 'error'));
}

function toggleRule(id) {
    apiFetch('/api/automation/rules/' + id + '/toggle', {method: 'POST'})
        .then(d => { if (d.success) location.reload(); })
        .catch(e => showToast(__tf('Failed to toggle: {0}', e.message), 'error'));
}

function saveWebhook(btn) {
    const name = document.getElementById('wh-name').value;
    const url = document.getElementById('wh-url').value;
    const eventType = document.getElementById('wh-event').value;
    const method = document.getElementById('wh-method').value;
    if (!name || !url) return showToast(__('Please enter name and URL'), 'error');
    withLoading(btn, apiFetch('/api/webhooks', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({name, url, event_type: eventType, method})})
        .then(d => { if (d.success) { showToast(__('Webhook saved'), 'success'); location.reload(); } else showToast(d.error, 'error'); })
        .catch(e => showToast(__tf('Failed to save webhook: {0}', e.message), 'error')));
}

function deleteWebhook(id) {
    if (!confirm(__('Delete?'))) return;
    apiFetch('/api/webhooks/' + id, {method: 'DELETE'})
        .then(d => { if (d.success) location.reload(); })
        .catch(e => showToast(__tf('Failed to delete: {0}', e.message), 'error'));
}

function testWebhook(btn) {
    const url = document.getElementById('wh-url').value;
    if (!url) return showToast(__('Please enter a URL first'), 'error');
    withLoading(btn, apiFetch('/api/webhooks/test', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({url, method: document.getElementById('wh-method').value})})
        .then(d => { showToast(d.success ? __('Test sent') : __('Failed: {0}', d.error), d.success ? 'success' : 'error'); })
        .catch(e => showToast(__tf('Failed: {0}', e.message), 'error')));
}
