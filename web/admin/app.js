document.addEventListener('DOMContentLoaded', () => {
    const loginPage = document.getElementById('login-page');
    const adminPage = document.getElementById('admin-page');
    const loginForm = document.getElementById('login-form');
    const loginError = document.getElementById('login-error');
    const logoutBtn = document.getElementById('logout-btn');
    const tabLinks = document.querySelectorAll('nav ul li a[data-tab]');
    const tabContents = document.querySelectorAll('.tab-content');

    let currentPath = '';
    let authInfo = { username: 'admin', salt: '' };

    // Fetch auth info (username and salt)
    async function fetchAuthInfo() {
        try {
            const res = await fetch('/api/auth/info');
            if (res.ok) {
                authInfo = await res.json();
                if (document.getElementById('username')) {
                    document.getElementById('username').placeholder = `用户名 (默认: ${authInfo.username})`;
                }
            }
        } catch (err) {
            console.error('Failed to fetch auth info:', err);
        }
    }
    fetchAuthInfo();

    // Check for existing token
    const token = localStorage.getItem('admin_token');
    if (token) {
        showAdminPage();
    }

    // Login
    loginForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        const username = document.getElementById('username').value || authInfo.username;
        const passwordRaw = document.getElementById('password').value;
        const password = await hashPassword(passwordRaw);

        try {
            const res = await fetch('/api/login', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ username, password })
            });

            if (res.ok) {
                const data = await res.json();
                localStorage.setItem('admin_token', data.token);
                showAdminPage();
            } else {
                loginError.textContent = '登录失败，请检查用户名和密码';
            }
        } catch (err) {
            loginError.textContent = '连接服务器失败';
        }
    });

    // Logout
    logoutBtn.addEventListener('click', () => {
        localStorage.removeItem('admin_token');
        location.reload();
    });

    // 辅助函数
    async function hashPassword(password) {
        if (!password) return '';
        const encoder = new TextEncoder();
        // 使用安装唯一的 salt 进行哈希，防止针对开源项目的通用彩虹表攻击
        const data = encoder.encode(password + authInfo.salt);
        const hashBuffer = await crypto.subtle.digest('SHA-256', data);
        const hashArray = Array.from(new Uint8Array(hashBuffer));
        return hashArray.map(b => b.toString(16).padStart(2, '0')).join('');
    }

    async function apiFetch(url, options = {}) {
        const token = localStorage.getItem('admin_token');
        options.headers = options.headers || {};
        if (token) {
            options.headers['Authorization'] = token;
        }
        if (options.body && !options.headers['Content-Type']) {
            options.headers['Content-Type'] = 'application/json';
        }
        
        const res = await fetch(url, options);
        if (res.status === 401) {
            localStorage.removeItem('admin_token');
            location.reload();
            throw new Error('会话过期，请重新登录');
        }
        return res;
    }

    function showMsg(id, text, type) {
        const el = document.getElementById(id);
        if (!el) {
            console.warn(`Element with id "${id}" not found for showMsg`);
            if (type === 'error') alert(text);
            return;
        }
        el.textContent = text;
        el.className = 'msg ' + type;
        setTimeout(() => {
            el.textContent = '';
            el.className = 'msg';
        }, 5000);
    }

    // Tabs
    tabLinks.forEach(link => {
        link.addEventListener('click', (e) => {
            e.preventDefault();
            const tabId = link.getAttribute('data-tab');
            showTab(tabId);
        });
    });

    function showAdminPage() {
        loginPage.style.display = 'none';
        adminPage.style.display = 'block';
        showTab('config');
    }

    function showTab(tabId) {
        tabContents.forEach(content => {
            content.style.display = content.id === `${tabId}-tab` ? 'block' : 'none';
        });

        if (tabId === 'config') loadConfig();
        if (tabId === 'files') loadFiles('');
        if (tabId === 'blacklist') loadBlacklist();
    }

    // 加载配置
    async function loadConfig() {
        try {
            console.log('Fetching config...');
            const res = await apiFetch('/api/admin/config');
            if (!res.ok) {
                throw new Error(`HTTP error! status: ${res.status}`);
            }
            const config = await res.json();
            console.log('Config loaded:', config);
            
            const form = document.getElementById('config-form');
            if (!form) {
                console.error('Config form not found!');
                return;
            }
            form.server_port.value = config.server_port;
            form.check_cron.value = config.check_cron;
            form.storage_path.value = config.storage_path;
            form.download_url_base.value = config.download_url_base || '';
            form.admin_user.value = config.admin_user;
            form.admin_enabled.checked = config.admin_enabled;
            form.proxy_url.value = config.proxy_url || '';
            form.asset_proxy_url.value = config.asset_proxy_url || '';
            form.concurrent_downloads.value = config.concurrent_downloads;
            form.download_timeout_minutes.value = config.download_timeout_minutes;
            form.xget_enabled.checked = config.xget_enabled;
            form.xget_domain.value = config.xget_domain || '';
            
            console.log('Populating launchers...');
            // 加载启动器
            const container = document.getElementById('launchers-container');
            container.innerHTML = '';
            if (config.launchers) {
                config.launchers.forEach(l => addLauncherItem(l));
            }
        } catch (err) {
            console.error('loadConfig error:', err);
            showMsg('config-msg', '加载配置失败: ' + err.message, 'error');
        }
    }

    // 添加启动器配置项
    function addLauncherItem(data = { name: '', source_url: '', repo_selector: '' }) {
        const container = document.getElementById('launchers-container');
        const item = document.createElement('div');
        item.className = 'launcher-item';
        item.innerHTML = `
            <button type="button" class="remove-btn">删除</button>
            <div class="form-group">
                <label>名称 (如 fcl, zl)</label>
                <input type="text" name="l_name" value="${data.name}" required>
            </div>
            <div class="form-group">
                <label>GitHub 仓库 URL / 来源页面</label>
                <input type="text" name="l_url" value="${data.source_url}" required>
            </div>
            <div class="form-group">
                <label>版本选择器 (可选)</label>
                <input type="text" name="l_selector" value="${data.repo_selector || ''}">
            </div>
        `;
        
        item.querySelector('.remove-btn').onclick = () => item.remove();
        container.appendChild(item);
    }

    document.getElementById('add-launcher-btn').onclick = () => addLauncherItem();

    // 保存配置
    document.getElementById('config-form').onsubmit = async (e) => {
        e.preventDefault();
        const form = e.target;
        
        // 收集启动器配置
        const launchers = [];
        const items = document.querySelectorAll('.launcher-item');
        items.forEach(item => {
            launchers.push({
                name: item.querySelector('[name="l_name"]').value,
                source_url: item.querySelector('[name="l_url"]').value,
                repo_selector: item.querySelector('[name="l_selector"]').value
            });
        });

        const config = {
            server_port: parseInt(form.server_port.value),
            check_cron: form.check_cron.value,
            storage_path: form.storage_path.value,
            download_url_base: form.download_url_base.value,
            admin_user: form.admin_user.value,
            admin_enabled: form.admin_enabled.checked,
            proxy_url: form.proxy_url.value,
            asset_proxy_url: form.asset_proxy_url.value,
            concurrent_downloads: parseInt(form.concurrent_downloads.value),
            download_timeout_minutes: parseInt(form.download_timeout_minutes.value),
            xget_enabled: form.xget_enabled.checked,
            xget_domain: form.xget_domain.value,
            launchers: launchers
        };

        // 仅当填写了密码时才发送
        if (form.admin_password.value) {
            config.admin_password = await hashPassword(form.admin_password.value);
        }
        if (form.github_token.value) {
            config.github_token = form.github_token.value;
        }

        try {
            const res = await apiFetch('/api/admin/config', {
                method: 'POST',
                body: JSON.stringify(config)
            });
            if (res.ok) {
                showMsg('config-msg', '保存成功，部分配置可能需要重启生效', 'success');
                loadConfig(); // 重新加载以清除密码字段
            } else {
                throw new Error(await res.text());
            }
        } catch (err) {
            showMsg('config-msg', '保存失败: ' + err.message, 'error');
        }
    };

    // File Manager
    async function loadFiles(path) {
        currentPath = path;
        const pathEl = document.getElementById('file-path');
        if (pathEl) {
            pathEl.textContent = '/' + path;
        }
        try {
            const res = await apiFetch(`/api/admin/files?path=${encodeURIComponent(path)}`);
            if (res.ok) {
                const files = await res.json();
                const tbody = document.querySelector('#file-table tbody');
                tbody.innerHTML = '';

                if (path !== '') {
                    const parentPath = path.split('/').slice(0, -1).join('/');
                    const tr = document.createElement('tr');
                    tr.innerHTML = `<td colspan="5" class="folder-link">.. (返回上一级)</td>`;
                    tr.onclick = () => loadFiles(parentPath);
                    tbody.appendChild(tr);
                }

                files.forEach(file => {
                    const tr = document.createElement('tr');
                    const size = file.is_dir ? '-' : formatSize(file.size);
                    const filePath = path ? `${path}/${file.name}` : file.name;
                    const actions = file.is_dir ? '' : `
                        <button class="download-btn" data-path="${filePath}">下载</button>
                    `;
                    tr.innerHTML = `
                        <td class="${file.is_dir ? 'folder-link' : ''}">${file.name}</td>
                        <td>${file.is_dir ? '目录' : '文件'}</td>
                        <td>${size}</td>
                        <td>${new Date(file.mod_time).toLocaleString()}</td>
                        <td class="actions-cell">
                            ${actions}
                            <button class="delete-btn" data-path="${filePath}">删除</button>
                        </td>
                    `;
                    if (file.is_dir) {
                        tr.querySelector('.folder-link').onclick = () => loadFiles(filePath);
                    } else {
                        tr.querySelector('.download-btn').onclick = () => downloadFile(filePath);
                    }
                    tr.querySelector('.delete-btn').onclick = (e) => {
                        e.stopPropagation();
                        deleteFile(filePath);
                    };
                    tbody.appendChild(tr);
                });
            }
        } catch (err) {
            console.error('加载文件失败:', err);
        }
    }

    async function deleteFile(path) {
        if (!confirm(`确定要删除 ${path} 吗？`)) return;
        try {
            const res = await apiFetch(`/api/admin/files?path=${encodeURIComponent(path)}`, {
                method: 'DELETE'
            });
            if (res.ok) {
                loadFiles(currentPath);
            } else {
                alert('删除失败');
            }
        } catch (err) {
            console.error('删除文件失败:', err);
        }
    }

    async function downloadFile(path) {
        const token = localStorage.getItem('admin_token');
        const url = `/api/admin/files/download?path=${encodeURIComponent(path)}`;
        
        // 使用 window.open 或创建一个隐藏的 <a> 标签
        // 因为需要 Authorization Header，不能直接使用 <a>，
        // 但我们可以通过 fetch 获取 blob 然后下载
        try {
            const res = await apiFetch(url);
            if (!res.ok) throw new Error('下载失败');
            const blob = await res.blob();
            const downloadUrl = window.URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = downloadUrl;
            a.download = path.split('/').pop();
            document.body.appendChild(a);
            a.click();
            window.URL.revokeObjectURL(downloadUrl);
            a.remove();
        } catch (err) {
            alert('下载失败: ' + err.message);
        }
    }

    const uploadBtn = document.getElementById('upload-btn');
    const uploadInput = document.getElementById('file-upload-input');

    if (uploadBtn && uploadInput) {
        uploadBtn.onclick = () => uploadInput.click();
        uploadInput.onchange = async () => {
            const file = uploadInput.files[0];
            if (!file) return;

            const path = currentPath ? `${currentPath}/${file.name}` : file.name;
            const formData = new FormData();
            formData.append('file', file);

            try {
                // apiFetch 默认设置 Content-Type: application/json，这里需要去掉
                const res = await fetch(`/api/admin/files?path=${encodeURIComponent(path)}`, {
                    method: 'POST',
                    headers: {
                        'Authorization': localStorage.getItem('admin_token')
                    },
                    body: formData
                });

                if (res.status === 401) {
                    localStorage.removeItem('admin_token');
                    window.location.reload();
                    return;
                }

                if (res.ok) {
                    showMsg('file-msg', '上传成功', 'success');
                    loadFiles(currentPath);
                } else {
                    alert('上传失败: ' + await res.text());
                }
            } catch (err) {
                alert('上传失败: ' + err.message);
            } finally {
                uploadInput.value = ''; // 清空选择
            }
        };
    }

    // Blacklist Manager
    async function loadBlacklist() {
        try {
            const res = await apiFetch('/api/admin/blacklist');
            if (res.ok) {
                const list = await res.json();
                const tbody = document.querySelector('#blacklist-table tbody');
                tbody.innerHTML = '';
                if (list && Array.isArray(list)) {
                    list.forEach(item => {
                        const tr = document.createElement('tr');
                        tr.innerHTML = `
                            <td>${item.ip}</td>
                            <td>${item.reason}</td>
                            <td>${new Date(item.created_at).toLocaleString()}</td>
                            <td>
                                <button class="delete-btn" data-ip="${item.ip}">移除</button>
                            </td>
                        `;
                        tr.querySelector('.delete-btn').onclick = () => removeBlacklist(item.ip);
                        tbody.appendChild(tr);
                    });
                }
            } else {
                const text = await res.text();
                console.error('加载黑名单失败:', text);
                alert('加载黑名单失败: ' + text);
            }
        } catch (err) {
            console.error('加载黑名单失败:', err);
            alert('加载黑名单失败: ' + err.message);
        }
    }

    document.getElementById('add-blacklist-form').onsubmit = async (e) => {
        e.preventDefault();
        const ip = document.getElementById('blacklist-ip').value;
        const reason = document.getElementById('blacklist-reason').value;
        try {
            const res = await apiFetch('/api/admin/blacklist', {
                method: 'POST',
                body: JSON.stringify({ ip, reason })
            });
            if (res.ok) {
                document.getElementById('blacklist-ip').value = '';
                document.getElementById('blacklist-reason').value = '';
                loadBlacklist();
            } else {
                alert('添加失败: ' + await res.text());
            }
        } catch (err) {
            alert('添加失败: ' + err.message);
        }
    };

    async function removeBlacklist(ip) {
        if (!confirm(`确定要移除 ${ip} 吗？`)) return;
        try {
            const res = await apiFetch(`/api/admin/blacklist?ip=${encodeURIComponent(ip)}`, {
                method: 'DELETE'
            });
            if (res.ok) {
                loadBlacklist();
            } else {
                alert('移除失败');
            }
        } catch (err) {
            console.error('移除黑名单失败:', err);
        }
    }

    function formatSize(bytes) {
        if (bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
    }
});
