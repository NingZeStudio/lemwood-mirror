async function loadStatus() {
    const res = await fetch('/api/status');
    const data = await res.json();
    
    // 获取最新版本信息
    const latestRes = await fetch('/api/latest');
    const latestData = await latestRes.json();
    
    const container = document.getElementById('status');
    container.innerHTML = '';
    const launchers = Object.keys(data);
    if (!launchers.length) {
        container.textContent = '暂无数据';
        return;
    }
    
    // 按启动器分组
    for (const name of launchers) {
        const versions = data[name];
        versions.sort((a, b) => String(b.tag_name || b.name).localeCompare(String(a.tag_name || a.name)));
        
        // 创建启动器分组容器
        const launcherGroup = document.createElement('div');
        launcherGroup.className = 'launcher-group';
        
        // 创建启动器标题（可折叠）
        const launcherHeader = document.createElement('div');
        launcherHeader.className = 'launcher-header';
        
        const launcherTitle = document.createElement('h3');
        launcherTitle.className = 'launcher-title';
        
        const toggleIcon = document.createElement('span');
        toggleIcon.className = 'toggle-icon';
        toggleIcon.textContent = '▼';
        
        const titleText = document.createElement('span');
        titleText.textContent = name;
        
        // 显示最新版本标签
        const latestVersion = latestData[name];
        if (latestVersion) {
            const latestBadge = document.createElement('span');
            latestBadge.className = 'latest-badge';
            latestBadge.textContent = `最新: ${latestVersion}`;
            launcherTitle.appendChild(toggleIcon);
            launcherTitle.appendChild(titleText);
            launcherTitle.appendChild(latestBadge);
        } else {
            launcherTitle.appendChild(toggleIcon);
            launcherTitle.appendChild(titleText);
        }
        
        launcherHeader.appendChild(launcherTitle);
        
        // 创建版本容器
        const versionsContainer = document.createElement('div');
        versionsContainer.className = 'versions-container';
        
        // 添加所有版本
        for (const v of versions) {
            const card = document.createElement('div');
            card.className = 'card';
            
            // 如果是最新版本，添加标记
            const isLatest = latestVersion === (v.tag_name || v.name);
            if (isLatest) {
                card.classList.add('latest-version');
            }

            const title = document.createElement('h3');
            title.textContent = v.tag_name || v.name;
            if (isLatest) {
                const badge = document.createElement('span');
                badge.className = 'version-badge';
                badge.textContent = 'Latest';
                title.appendChild(badge);
            }

            const meta = document.createElement('div');
            meta.className = 'meta';
            const publishedDate = v.published_at ? new Date(v.published_at).toLocaleString() : '未知';
            meta.textContent = `发布于：${publishedDate}`;

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

                    if (a.url && (a.url.startsWith('http://') || a.url.startsWith('https://'))) {
                        link.href = a.url;
                    }

                    const copyBtn = document.createElement('button');
                    copyBtn.className = 'copy-btn';
                    copyBtn.innerHTML = '<svg viewBox="0 0 24 24" width="14" height="14" stroke="currentColor" stroke-width="2" fill="none" stroke-linecap="round" stroke-linejoin="round" style="margin-right: 4px;"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path></svg>复制';
                    copyBtn.onclick = (e) => {
                        e.preventDefault();
                        const fullUrl = link.href.startsWith('http') ? link.href : window.location.origin + link.href;
                        navigator.clipboard.writeText(fullUrl).then(() => {
                            const originalHTML = copyBtn.innerHTML;
                            copyBtn.classList.add('copied');
                            copyBtn.innerHTML = '<svg viewBox="0 0 24 24" width="14" height="14" stroke="currentColor" stroke-width="2" fill="none" stroke-linecap="round" stroke-linejoin="round" style="margin-right: 4px;"><polyline points="20 6 9 17 4 12"></polyline></svg>已复制';
                            setTimeout(() => {
                                copyBtn.classList.remove('copied');
                                copyBtn.innerHTML = originalHTML;
                            }, 2000);
                        });
                    };

                    item.appendChild(link);
                    item.appendChild(copyBtn);
                    assetsDiv.appendChild(item);
                }
            }

            card.appendChild(title);
            card.appendChild(meta);
            card.appendChild(assetsDiv);
            versionsContainer.appendChild(card);
        }
        
        // 添加折叠功能
        launcherHeader.addEventListener('click', () => {
            versionsContainer.classList.toggle('collapsed');
            toggleIcon.textContent = versionsContainer.classList.contains('collapsed') ? '▶' : '▼';
        });
        
        launcherGroup.appendChild(launcherHeader);
        launcherGroup.appendChild(versionsContainer);
        container.appendChild(launcherGroup);
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

async function loadStats() {
    try {
        const res = await fetch('/api/stats');
        if (!res.ok) return;
        const data = await res.json();

        // Overview
        document.getElementById('total-visits').textContent = data.total_visits.toLocaleString();
        document.getElementById('total-downloads').textContent = data.total_downloads.toLocaleString();

        // Daily Chart
        const chartContainer = document.getElementById('daily-chart');
        chartContainer.innerHTML = '';
        
        if (data.daily_stats && data.daily_stats.length > 0) {
             // 翻转数组，让日期从左到右递增（API 返回的是降序）
            const stats = [...data.daily_stats].reverse();
            
            // 找出最大值用于缩放
            let maxVal = 0;
            for (const d of stats) {
                if (d.visit_count > maxVal) maxVal = d.visit_count;
                if (d.download_count > maxVal) maxVal = d.download_count;
            }
            if (maxVal === 0) maxVal = 1;

            for (const d of stats) {
                const group = document.createElement('div');
                group.className = 'chart-bar-group';
                
                // Tooltip
                const tooltip = document.createElement('div');
                tooltip.className = 'chart-tooltip';
                tooltip.textContent = `${d.date}: 访问 ${d.visit_count} / 下载 ${d.download_count}`;
                
                // Visit Bar
                const visitBar = document.createElement('div');
                visitBar.className = 'chart-bar visit';
                visitBar.style.height = `${(d.visit_count / maxVal) * 100}%`;
                
                // Download Bar
                const dlBar = document.createElement('div');
                dlBar.className = 'chart-bar download';
                dlBar.style.height = `${(d.download_count / maxVal) * 100}%`;

                // Date Label
                const dateLabel = document.createElement('div');
                dateLabel.className = 'chart-date';
                dateLabel.textContent = d.date.slice(5); // MM-DD

                group.appendChild(tooltip);
                group.appendChild(visitBar);
                group.appendChild(dlBar);
                group.appendChild(dateLabel);
                chartContainer.appendChild(group);
            }
        } else {
            chartContainer.textContent = '暂无数据';
            chartContainer.style.alignItems = 'center';
            chartContainer.style.justifyContent = 'center';
        }

        // Top Downloads
        const topContainer = document.getElementById('top-downloads');
        topContainer.innerHTML = '';
        if (data.top_downloads && data.top_downloads.length > 0) {
            for (const item of data.top_downloads) {
                const row = document.createElement('div');
                row.className = 'stat-list-item';
                
                const name = document.createElement('div');
                name.className = 'stat-list-name';
                name.textContent = `${item.launcher} ${item.version}`;
                name.title = `${item.launcher} ${item.version}`;
                
                const count = document.createElement('div');
                count.className = 'stat-list-count';
                count.textContent = item.count.toLocaleString();
                
                row.appendChild(name);
                row.appendChild(count);
                topContainer.appendChild(row);
            }
        } else {
             topContainer.textContent = '暂无数据';
        }

        // Geo Distribution
        const geoContainer = document.getElementById('geo-dist');
        geoContainer.innerHTML = '';
        if (data.geo_distribution && data.geo_distribution.length > 0) {
             for (const item of data.geo_distribution) {
                const row = document.createElement('div');
                row.className = 'stat-list-item';
                
                const name = document.createElement('div');
                name.className = 'stat-list-name';
                name.textContent = item.country || '未知';
                
                const count = document.createElement('div');
                count.className = 'stat-list-count';
                count.textContent = item.count.toLocaleString();
                
                row.appendChild(name);
                row.appendChild(count);
                geoContainer.appendChild(row);
            }
        } else {
            geoContainer.textContent = '暂无数据';
        }

    } catch (e) {
        console.error('加载统计数据失败:', e);
    }
}

async function manualRefresh() {
    const refreshButton = document.getElementById('refresh');
    refreshButton.textContent = '正在刷新...';
    refreshButton.disabled = true;
    try {
        await fetch('/api/scan', { method: 'POST' });
        await loadStatus();
        await loadStats(); // 刷新后重新加载统计
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
    loadStats();
});
