function showLootScreenshot(agentId, filename) {
    const modal = document.getElementById('loot-modal');
    const img = document.getElementById('modal-img');
    const title = document.getElementById('modal-title');
    const dl = document.getElementById('modal-download');

    const src = `/screenshots/${agentId}/${filename}`;
    img.src = src;
    title.textContent = `${agentId} / ${filename}`;
    dl.href = src;
    dl.download = filename;

    modal.classList.remove('hidden');
}
