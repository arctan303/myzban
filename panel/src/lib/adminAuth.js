'use client';
import { useState, useEffect } from 'react';

// Auth guard hook — redirects to /login if not authenticated, or to /my if non-admin
export function useAdminAuth() {
  const [token, setToken] = useState(null);
  const [user, setUser] = useState(null);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    const t = localStorage.getItem('pnm_token');
    const u = localStorage.getItem('pnm_user');
    if (!t || !u) {
      window.location.href = '/login';
      return;
    }
    const parsed = JSON.parse(u);
    if (parsed.role !== 'admin') {
      window.location.href = '/my';
      return;
    }
    setToken(t);
    setUser(parsed);
    setReady(true);
  }, []);

  return { token, user, ready };
}

// Fetch wrapper that attaches the auth token from localStorage
export function authFetch(url, options = {}) {
  const token = localStorage.getItem('pnm_token');
  return fetch(url, {
    ...options,
    headers: {
      ...options.headers,
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
  });
}

// Admin sidebar with logout and settings
export function AdminSidebar({ currentPage }) {
  const [showSettings, setShowSettings] = useState(false);
  const [settingsForm, setSettingsForm] = useState({ username: '', password: '' });
  const [settingsMsg, setSettingsMsg] = useState('');
  const [settingsLoading, setSettingsLoading] = useState(false);

  // Initialize form with current user from localStorage
  useEffect(() => {
    if (showSettings) {
      const u = localStorage.getItem('pnm_user');
      if (u) {
        const parsed = JSON.parse(u);
        setSettingsForm({ username: parsed.username, password: '' });
      }
      setSettingsMsg('');
    }
  }, [showSettings]);

  const handleSettingsSave = async () => {
    setSettingsMsg('');
    setSettingsLoading(true);
    const u = JSON.parse(localStorage.getItem('pnm_user'));
    
    try {
      const res = await authFetch('/api/panel-users', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          id: u.id,
          username: settingsForm.username,
          new_password: settingsForm.password || null
        })
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error);

      // Update local storage
      u.username = settingsForm.username;
      localStorage.setItem('pnm_user', JSON.stringify(u));
      
      setSettingsMsg('✅ 账号信息已更新！');
      setTimeout(() => setShowSettings(false), 1500);
    } catch (e) {
      setSettingsMsg(`❌ ${e.message}`);
    } finally {
      setSettingsLoading(false);
    }
  };
  const links = [
    { href: '/', icon: '📊', label: '控制台' },
    { href: '/nodes', icon: '🖥️', label: '节点管理' },
    { href: '/users', icon: '👥', label: '用户管理' },
  ];

  const logout = () => {
    localStorage.removeItem('pnm_token');
    localStorage.removeItem('pnm_user');
    window.location.href = '/login';
  };

  return (
    <nav className="sidebar">
      <div className="sidebar-logo">
        <h1>PNM Panel</h1>
        <p>ProxyNode Manager</p>
      </div>
      <ul className="sidebar-nav">
        {links.map((link) => (
          <li key={link.href}>
            <a href={link.href} className={currentPage === link.href ? 'active' : ''}>
              <span className="nav-icon">{link.icon}</span>
              {link.label}
            </a>
          </li>
        ))}
      </ul>
      <div className="sidebar-footer" style={{ display: 'flex', gap: '8px', flexDirection: 'column' }}>
        <button className="btn btn-secondary sidebar-logout-btn" onClick={() => setShowSettings(true)}>
          ⚙️ 账号设置
        </button>
        <button className="btn btn-danger sidebar-logout-btn" onClick={logout}>
          🚪 退出登录
        </button>
      </div>

      {showSettings && (
        <div className="modal-overlay" onClick={() => setShowSettings(false)}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <h3>⚙️ 修改管理员账号</h3>
            
            {settingsMsg && (
              <div style={{ marginBottom: '16px', fontSize: '14px', color: settingsMsg.startsWith('✅') ? 'var(--success)' : 'var(--danger)' }}>
                {settingsMsg}
              </div>
            )}

            <div className="form-group">
              <label>新用户名 (默认 admin)</label>
              <input 
                className="form-input" 
                value={settingsForm.username}
                onChange={e => setSettingsForm({...settingsForm, username: e.target.value})}
              />
            </div>

            <div className="form-group">
              <label>新密码 (留空则不修改)</label>
              <input 
                className="form-input" 
                type="password"
                placeholder="若不修改密码请留空"
                value={settingsForm.password}
                onChange={e => setSettingsForm({...settingsForm, password: e.target.value})}
              />
            </div>

            <div className="modal-actions">
              <button className="btn btn-secondary" onClick={() => setShowSettings(false)}>取消</button>
              <button 
                className="btn btn-primary" 
                onClick={handleSettingsSave}
                disabled={settingsLoading || !settingsForm.username}
              >
                {settingsLoading ? '保存中...' : '保存更改'}
              </button>
            </div>
          </div>
        </div>
      )}
    </nav>
  );
}
