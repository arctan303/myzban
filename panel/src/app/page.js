'use client';
import { useState, useEffect, useCallback } from 'react';

function formatBytes(bytes) {
  if (!bytes || bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

function Sidebar({ currentPage }) {
  const links = [
    { href: '/', icon: '📊', label: 'Dashboard' },
    { href: '/nodes', icon: '🖥️', label: 'Nodes' },
    { href: '/users', icon: '👥', label: 'Users' },
  ];

  return (
    <nav className="sidebar">
      <div className="sidebar-logo">
        <h1>PNM Panel</h1>
        <p>ProxyNode Manager</p>
      </div>
      <ul className="sidebar-nav">
        {links.map((link) => (
          <li key={link.href}>
            <a
              href={link.href}
              className={currentPage === link.href ? 'active' : ''}
            >
              <span className="nav-icon">{link.icon}</span>
              {link.label}
            </a>
          </li>
        ))}
      </ul>
    </nav>
  );
}

export default function DashboardPage() {
  const [nodes, setNodes] = useState([]);
  const [loading, setLoading] = useState(true);

  const fetchNodes = useCallback(async () => {
    try {
      const res = await fetch('/api/nodes');
      const data = await res.json();
      setNodes(data);
    } catch (e) {
      console.error(e);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchNodes();
    const interval = setInterval(fetchNodes, 15000);
    return () => clearInterval(interval);
  }, [fetchNodes]);

  const totalUsers = nodes.reduce((sum, n) => sum + (n.status?.total_users || 0), 0);
  const onlineNodes = nodes.filter((n) => n.online).length;
  const totalUp = nodes.reduce((sum, n) => sum + (n.status?.total_upload || 0), 0);
  const totalDown = nodes.reduce((sum, n) => sum + (n.status?.total_download || 0), 0);

  return (
    <div className="app-layout">
      <Sidebar currentPage="/" />
      <main className="main-content">
        <div className="page-header">
          <h2>Dashboard</h2>
          <p>Overview of all managed proxy nodes</p>
        </div>

        <div className="card-grid">
          <div className="stat-card">
            <div className="stat-label">Total Nodes</div>
            <div className="stat-value">{nodes.length}</div>
            <div className="stat-sub">{onlineNodes} online</div>
          </div>
          <div className="stat-card">
            <div className="stat-label">Total Users</div>
            <div className="stat-value">{totalUsers}</div>
          </div>
          <div className="stat-card">
            <div className="stat-label">Total Traffic</div>
            <div className="stat-value">{formatBytes(totalUp + totalDown)}</div>
            <div className="stat-sub">↑ {formatBytes(totalUp)} ↓ {formatBytes(totalDown)}</div>
          </div>
        </div>

        <div className="page-header">
          <h2 style={{ fontSize: '20px' }}>Nodes</h2>
        </div>

        {loading ? (
          <div className="loading">
            <div className="spinner"></div>
            Loading nodes...
          </div>
        ) : nodes.length === 0 ? (
          <div className="empty-state">
            <div className="icon">🖥️</div>
            <h3>No nodes connected</h3>
            <p>Go to the Nodes page to add your first node.</p>
            <a href="/nodes" className="btn btn-primary" style={{ marginTop: '16px' }}>
              Add Node
            </a>
          </div>
        ) : (
          <div className="card-grid">
            {nodes.map((node) => (
              <div className="node-card" key={node.id}>
                <div className="node-card-header">
                  <div>
                    <h3>{node.name}</h3>
                    <div className="node-ip">{node.status?.server_ip || node.address}</div>
                  </div>
                  <span className={`badge ${node.online ? 'badge-success' : 'badge-danger'}`}>
                    <span className="badge-dot"></span>
                    {node.online ? 'Online' : 'Offline'}
                  </span>
                </div>

                {node.online && node.status && (
                  <>
                    <div className="node-protocols">
                      <span className={`protocol-badge ${node.status.vless?.running ? 'running' : ''}`}>
                        VLESS :{node.status.vless?.port}
                      </span>
                      <span className={`protocol-badge ${node.status.hysteria2?.running ? 'running' : ''}`}>
                        Hy2 :{node.status.hysteria2?.port}
                      </span>
                    </div>

                    <div className="node-stats">
                      <div className="node-stat-item">
                        <div className="label">Users</div>
                        <div className="value">{node.status.total_users}</div>
                      </div>
                      <div className="node-stat-item">
                        <div className="label">Upload</div>
                        <div className="value">{formatBytes(node.status.total_upload)}</div>
                      </div>
                      <div className="node-stat-item">
                        <div className="label">Download</div>
                        <div className="value">{formatBytes(node.status.total_download)}</div>
                      </div>
                    </div>
                  </>
                )}

                {!node.online && (
                  <div style={{ color: 'var(--text-muted)', fontSize: '13px', marginTop: '8px' }}>
                    {node.error || 'Connection failed'}
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </main>
    </div>
  );
}
