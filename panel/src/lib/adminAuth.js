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

// Admin sidebar with logout
export function AdminSidebar({ currentPage }) {
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
      <div className="sidebar-footer">
        <button className="btn btn-secondary sidebar-logout-btn" onClick={logout}>
          🚪 退出登录
        </button>
      </div>
    </nav>
  );
}
