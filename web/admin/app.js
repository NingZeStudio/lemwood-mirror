document.addEventListener('DOMContentLoaded', () => {
    const loginPage = document.getElementById('login-page');
    const adminPage = document.getElementById('admin-page');
    const loginForm = document.getElementById('login-form');
    const loginError = document.getElementById('login-error');
    const logoutBtn = document.getElementById('logout-btn');
    const tabLinks = document.querySelectorAll('nav ul li a[data-tab]');
    const tabContents = document.querySelectorAll('.tab-content');

    let currentPath = '';
    let adminUser = 'admin';

    // 检查是否有现有 token
    let token = localStorage.getItem('admin_token');
    if (!token) {
        // 尝试从 cookie 获取 (针对 OAuth2 回调)
        const cookies = document.cookie.split(';');
        for (let c of cookies) {
            c = c.trim();
            if (c.startsWith('admin_token=')) {
                token = c.substring('admin_token='.length);
                localStorage.setItem('admin_token', token);
                // 清除 cookie
                document.cookie = "admin_token=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;";
                break;
            }
        }
    }
    if (token) {
        showAdminPage();
    }

    // 检查 2FA 状态
    async function check2FAStatus() {
        try {
            const res = await fetch('/api/auth/2fa/status');
            if (res.ok) {
                const data = await res.json();
                if (data.enabled) {
                    document.getElementById('totp-input-group').style.display = 'block';
                    document.getElementById('otp_code').required = true;
                }
            }
        } catch (err) {
            console.error('Failed to check 2FA status:', err);
        }
    }
    check2FAStatus();

    // 登录
    loginForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        const username = document.getElementById('username').value || adminUser;
        const password = document.getElementById('password').value;
        const otp_code = document.getElementById('otp_code').value;

        try {
            const res = await fetch('/api/login', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ username, password, otp_code })
            });

            if (res.ok) {
                const data = await res.json();
                localStorage.setItem('admin_token', data.token);
                showAdminPage();
            } else {
                const text = await res.text();
                loginError.textContent = text || '登录失败，请检查用户名、密码或验证码';
            }
        } catch (err) {
            loginError.textContent = '连接服务器失败';
        }
    });

    // 登出
    logoutBtn.addEventListener('click', () => {
        localStorage.removeItem('admin_token');
        location.reload();
    });

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

            // 管理员限制
            form.admin_max_retries.value = config.admin_max_retries || 10;
            form.admin_lock_duration.value = config.admin_lock_duration || 120;

            // TOTP Config
            form.two_factor_enabled.checked = config.two_factor_enabled;
            form.two_factor_secret.value = config.two_factor_secret || '';
            document.getElementById('totp-setup-section').style.display = config.two_factor_enabled ? 'block' : 'none';
            if (config.two_factor_enabled && config.two_factor_secret) {
                updateTOTPQR(config.two_factor_secret);
            }
            
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

    // TOTP 相关逻辑
    const twoFactorEnabled = document.getElementById('two_factor_enabled');
    const totpSetupSection = document.getElementById('totp-setup-section');
    const generateTotpBtn = document.getElementById('generate-totp-btn');
    const twoFactorSecret = document.getElementById('two_factor_secret');

    twoFactorEnabled.addEventListener('change', () => {
        totpSetupSection.style.display = twoFactorEnabled.checked ? 'block' : 'none';
        if (twoFactorEnabled.checked && !twoFactorSecret.value) {
            generateNewSecret();
        }
    });

    generateTotpBtn.addEventListener('click', generateNewSecret);

    function generateNewSecret() {
        const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567'; // Base32
        let secret = '';
        for (let i = 0; i < 16; i++) {
            secret += chars.charAt(Math.floor(Math.random() * chars.length));
        }
        twoFactorSecret.value = secret;
        updateTOTPQR(secret);
    }

    function updateTOTPQR(secret) {
        const qrDiv = document.getElementById('totp-qr-code');
        qrDiv.innerHTML = '';
        const url = `otpauth://totp/LemwoodMirror:admin?secret=${secret}&issuer=LemwoodMirror`;
        const qrImg = document.createElement('img');
        qrImg.src = `https://api.qrserver.com/v1/create-qr-code/?size=150x150&data=${encodeURIComponent(url)}`;
        qrDiv.appendChild(qrImg);
    }

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
            admin_max_retries: parseInt(form.admin_max_retries.value),
            admin_lock_duration: parseInt(form.admin_lock_duration.value),
            two_factor_enabled: form.two_factor_enabled.checked,
            two_factor_secret: form.two_factor_secret.value,
            launchers: launchers
        };

        if (form.admin_password.value) {
            config.admin_password = form.admin_password.value;
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
