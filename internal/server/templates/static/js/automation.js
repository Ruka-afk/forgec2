function showNewRuleModal() { document.getElementById('rule-modal').classList.remove('hidden'); }
function closeRuleModal() { document.getElementById('rule-modal').classList.add('hidden'); }
function showNewWebhookModal() { document.getElementById('webhook-modal').classList.remove('hidden'); }
function closeWebhookModal() { document.getElementById('webhook-modal').classList.add('hidden'); }

function saveRule() {
    const name = document.getElementById('rule-name').value;
    const eventType = document.getElementById('rule-event').value;
    const condition = document.getElementById('rule-condition').value;
    const command = document.getElementById('rule-command').value;
    const webhook = document.getElementById('rule-webhook').value;
    if (!name) return showToast('请输入规则名称', 'error');
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
    fetch('/api/automation/rules', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify(rule)})
    .then(r=>r.json()).then(d=>{if(d.success){showToast('规则已保存','success');location.reload()}else showToast(d.error,'error')});
}

function deleteRule(id) {
    if (!confirm('确定删除此规则?')) return;
    fetch('/api/automation/rules/' + id, {method: 'DELETE'})
    .then(r=>r.json()).then(d=>{if(d.success){showToast('已删除','success');location.reload()}});
}

function toggleRule(id) {
    fetch('/api/automation/rules/' + id + '/toggle', {method: 'POST'})
    .then(r=>r.json()).then(d=>{if(d.success)location.reload()});
}

function saveWebhook() {
    const name = document.getElementById('wh-name').value;
    const url = document.getElementById('wh-url').value;
    const eventType = document.getElementById('wh-event').value;
    const method = document.getElementById('wh-method').value;
    if (!name || !url) return showToast('请填写名称和URL', 'error');
    fetch('/api/webhooks', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({name, url, event_type: eventType, method})})
    .then(r=>r.json()).then(d=>{if(d.success){showToast('Webhook已保存','success');location.reload()}else showToast(d.error,'error')});
}

function deleteWebhook(id) {
    if (!confirm('确定删除?')) return;
    fetch('/api/webhooks/' + id, {method: 'DELETE'}).then(r=>r.json()).then(d=>{if(d.success)location.reload()});
}

function testWebhook() {
    const url = document.getElementById('wh-url').value;
    if (!url) return showToast('请先填写URL', 'error');
    fetch('/api/webhooks/test', {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({url, method: document.getElementById('wh-method').value})})
    .then(r=>r.json()).then(d=>{showToast(d.success?'测试已发送':'失败: '+d.error, d.success?'success':'error')});
}
