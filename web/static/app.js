async function loadStatus() {
    const res = await fetch('/api/status');
    const data = await res.json();
    const container = document.getElementById('status');
    container.innerHTML = '';
    const launchers = Object.keys(data);
    if (!launchers.length) {
        container.textContent = '暂无数据';
        return;
    }
    for (const name of launchers) {
        const versions = data[name];
        versions.sort((a, b) => String(b.tag_name || b.name).localeCompare(String(a.tag_name || a.name)));
        for (const v of versions) {
            const card = document.createElement('div');
            card.className = 'card';

            const title = document.createElement('h3');
            title.textContent = `${name} - ${v.tag_name || v.name}`;

            const meta = document.createElement('div');
            meta.className = 'meta';
            const publishedDate = v.published_at ? new Date(v.published_at).toLocaleString() : '未知';
            meta.textContent = `发布于：${publishedDate}`;

            const pathDiv = document.createElement('div');
            pathDiv.className = 'path';
            pathDiv.textContent = `路径: ${v.download_path || '未知'}`;

            const assetsDiv = document.createElement('div');
            assetsDiv.className = 'assets';

            if (Array.isArray(v.assets)) {
                for (const a of v.assets) {
                    const item = document.createElement('div');
                    item.className = 'asset-item';

                    const link = document.createElement('a');
                    link.className = 'asset-link';
                    const downloadUrl = `/download/${name}/${v.tag_name || v.name}/${a.name}`;
                    link.href = downloadUrl;
                    link.textContent = a.name;
                    link.setAttribute('download', a.name);

                    // If download_url_base is used, the URL might be absolute, so we check if it starts with http
                    if (a.url && (a.url.startsWith('http://') || a.url.startsWith('https://'))) {
                        link.href = a.url;
                    }

                    const copyBtn = document.createElement('button');
                    copyBtn.className = 'copy-btn';
                    copyBtn.textContent = '复制链接';
                    copyBtn.onclick = (e) => {
                        e.preventDefault();
                        const fullUrl = link.href.startsWith('http') ? link.href : window.location.origin + link.href;
                        navigator.clipboard.writeText(fullUrl).then(() => {
                            const originalText = copyBtn.textContent;
                            copyBtn.textContent = '已复制';
                            setTimeout(() => copyBtn.textContent = originalText, 2000);
                        });
                    };

                    item.appendChild(link);
                    item.appendChild(copyBtn);
                    assetsDiv.appendChild(item);
                }
            }

            card.appendChild(title);
            card.appendChild(meta);
            card.appendChild(pathDiv);
            card.appendChild(assetsDiv);
            container.appendChild(card);
        }
    }
}

async function loadFiles() {
    const p = document.getElementById('path').value || '.';
    try {
        const res = await fetch(`/api/files?path=${encodeURIComponent(p)}`);
        const data = await res.json();
        document.getElementById('files').textContent = JSON.stringify(data, null, 2);
    } catch (error) {
        document.getElementById('files').textContent = '加载文件列表失败。';
    }
}

async function manualRefresh() {
    const refreshButton = document.getElementById('refresh');
    refreshButton.textContent = '正在刷新...';
    refreshButton.disabled = true;
    try {
        await fetch('/api/scan', { method: 'POST' });
        await loadStatus();
    } catch (e) {
        console.error('手动刷新失败:', e);
    } finally {
        refreshButton.textContent = '手动刷新';
        refreshButton.disabled = false;
    }
}

function toggleApiDocs() {
    const docs = document.getElementById('api-docs');
    const btn = document.getElementById('show-api-docs');
    if (docs.classList.contains('hidden')) {
        docs.classList.remove('hidden');
        btn.textContent = '隐藏 API 文档';
    } else {
        docs.classList.add('hidden');
        btn.textContent = 'API 文档';
    }
}

window.addEventListener('DOMContentLoaded', () => {
    document.getElementById('refresh').addEventListener('click', manualRefresh);
    document.getElementById('list').addEventListener('click', loadFiles);
    document.getElementById('show-api-docs').addEventListener('click', toggleApiDocs);
    loadStatus();
    loadFiles();
});
