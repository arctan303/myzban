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
    { href: '/', icon: '📊', label: '控制台' },
    { href: '/nodes', icon: '🖥️', label: '节点管理' },
    { href: '/users', icon: '👥', label: '用户管理' },
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
          <h2>控制台概览</h2>
          <p>所有代理节点的监控运行状态</p>
        </div>

        <div className="card-grid">
          <div className="stat-card">
            <div className="stat-label">总节点数</div>
            <div className="stat-value">{nodes.length}</div>
            <div className="stat-sub">{onlineNodes} 在线运行</div>
          </div>
          <div className="stat-card">
            <div className="stat-label">总用户数</div>
            <div className="stat-value">{totalUsers}</div>
          </div>
          <div className="stat-card">
            <div className="stat-label">消耗总流量</div>
            <div className="stat-value">{formatBytes(totalUp + totalDown)}</div>
            <div className="stat-sub">↑ {formatBytes(totalUp)} ↓ {formatBytes(totalDown)}</div>
          </div>
        </div>

        <div className="page-header">
          <h2 style={{ fontSize: '20px' }}>服务器集群</h2>
        </div>

        {loading ? (
          <div className="loading">
            <div className="spinner"></div>
            正在加载节点数据...
          </div>
        ) : nodes.length === 0 ? (
          <div className="empty-state">
            <div className="icon">🖥️</div>
            <h3>暂未接入任何节点</h3>
            <p>请前往「节点管理」页面接入你的第一台代理服务器。</p>
            <a href="/nodes" className="btn btn-primary" style={{ marginTop: '16px' }}>
              添加节点
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
                    {node.online ? '在线运行可用' : '离线/失联'}
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
                        <div className="label">承载用户</div>
                        <div className="value">{node.status.total_users}</div>
                      </div>
                      <div className="node-stat-item">
                        <div className="label">上行流量</div>
                        <div className="value">{formatBytes(node.status.total_upload)}</div>
                      </div>
                      <div className="node-stat-item">
                        <div className="label">下行流量</div>
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
