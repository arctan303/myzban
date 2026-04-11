'use client';
import { useState, useEffect, useCallback } from 'react';

function formatBytes(bytes) {
  if (!bytes || bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

function useAuth() {
  const [token, setToken] = useState(null);
  const [user, setUser] = useState(null);
  const [checked, setChecked] = useState(false);

  useEffect(() => {
    const t = localStorage.getItem('pnm_token');
    const u = localStorage.getItem('pnm_user');
    if (!t || !u) {
      window.location.href = '/login';
      return;
    }
    const parsed = JSON.parse(u);
    if (parsed.role === 'admin') {
      window.location.href = '/';
      return;
    }
    setToken(t);
    setUser(parsed);
    setChecked(true);
  }, []);

  return { token, user, checked };
}

export default function MyPage() {
  const { token, user, checked } = useAuth();
  const [data, setData] = useState(null);
  const [loading, setLoading] = useState(true);
  const [toast, setToast] = useState(null);

  const showToast = (msg, type = 'success') => {
    setToast({ msg, type });
    setTimeout(() => setToast(null), 3000);
  };

  const copyToClipboard = (text) => {
    navigator.clipboard.writeText(text);
    showToast('已复制到剪贴板！');
  };

  const fetchMyInfo = useCallback(async () => {
    if (!token) return;
    try {
      const res = await fetch('/api/my', {
        headers: { Authorization: `Bearer ${token}` },
      });
      if (res.ok) {
        setData(await res.json());
      }
    } catch (e) {
      console.error(e);
    } finally {
      setLoading(false);
    }
  }, [token]);

  useEffect(() => {
    if (checked && token) fetchMyInfo();
  }, [checked, token, fetchMyInfo]);

  const logout = () => {
    localStorage.removeItem('pnm_token');
    localStorage.removeItem('pnm_user');
    window.location.href = '/login';
  };

  if (!checked) return null;

  return (
    <div className="app-layout">
      <nav className="sidebar">
        <div className="sidebar-logo">
          <h1>PNM Panel</h1>
          <p>ProxyNode Manager</p>
        </div>
        <ul className="sidebar-nav">
          <li>
            <a href="/my" className="active">
              <span className="nav-icon">📱</span>
              我的面板
            </a>
          </li>
        </ul>
        <div className="sidebar-footer">
          <button className="btn btn-secondary sidebar-logout-btn" onClick={logout}>
            🚪 退出登录
          </button>
        </div>
      </nav>

      <main className="main-content">
        <div className="page-header">
          <h2>👋 欢迎，{user?.username}</h2>
          <p>这是你的个人代理信息面板</p>
        </div>

        {loading ? (
          <div className="loading"><div className="spinner"></div>正在加载...</div>
        ) : !data || !data.nodes || data.nodes.length === 0 ? (
          <div className="empty-state">
            <div className="icon">📡</div>
            <h3>暂无可用节点</h3>
            <p>你的账号尚未被分配到任何代理节点，请联系管理员。</p>
          </div>
        ) : (
          <>
            {/* Traffic Overview */}
            <div className="card-grid" style={{ gridTemplateColumns: 'repeat(3, 1fr)' }}>
              <div className="stat-card">
                <div className="stat-label">总上传</div>
                <div className="stat-value">{formatBytes(data.total_traffic_up)}</div>
              </div>
              <div className="stat-card">
                <div className="stat-label">总下载</div>
                <div className="stat-value">{formatBytes(data.total_traffic_down)}</div>
              </div>
              <div className="stat-card">
                <div className="stat-label">总用量</div>
                <div className="stat-value">{formatBytes(data.total_traffic_up + data.total_traffic_down)}</div>
              </div>
            </div>

            {/* Node Details */}
            {data.nodes.map((node) => (
              <div key={node.node_id} className="card" style={{ marginBottom: '16px' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
                  <div>
                    <h3 style={{ fontSize: '16px', fontWeight: 600 }}>{node.node_name}</h3>
                    <div style={{ display: 'flex', gap: '8px', marginTop: '8px' }}>
                      {node.vless_running && <span className="protocol-badge running">VLESS</span>}
                      {node.hy2_running && <span className="protocol-badge running">Hy2</span>}
                    </div>
                  </div>
                  <div style={{ textAlign: 'right' }}>
                    <div style={{ fontSize: '12px', color: 'var(--text-muted)' }}>本节点用量</div>
                    <div style={{ fontWeight: 700, fontSize: '18px' }}>{formatBytes(node.traffic_up + node.traffic_down)}</div>
                  </div>
                </div>

                {/* Subscription link */}
                <div style={{ marginBottom: '12px' }}>
                  <label style={{ fontSize: '12px', color: 'var(--text-muted)' }}>一键订阅链接（Clash / Stash / Meta）</label>
                  <div className="copy-text" style={{ maxWidth: '100%', marginTop: '6px' }}
                    onClick={() => copyToClipboard(`${window.location.origin}/api/sub/${node.node_id}/${node.sub_token}`)}>
                    {window.location.origin}/api/sub/{node.node_id}/{node.sub_token}
                  </div>
                </div>

                {/* Credentials */}
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
                  {node.uuid && (
                    <div>
                      <label style={{ fontSize: '12px', color: 'var(--text-muted)' }}>VLESS UUID</label>
                      <div className="copy-text" onClick={() => copyToClipboard(node.uuid)}>
                        {node.uuid}
                      </div>
                    </div>
                  )}
                  {node.hy2_password && (
                    <div>
                      <label style={{ fontSize: '12px', color: 'var(--text-muted)' }}>Hy2 密码</label>
                      <div className="copy-text" onClick={() => copyToClipboard(node.hy2_password)}>
                        {node.hy2_password}
                      </div>
                    </div>
                  )}
                </div>
              </div>
            ))}
          </>
        )}

        {toast && <div className={`toast toast-${toast.type}`}>{toast.msg}</div>}
      </main>
    </div>
  );
}
