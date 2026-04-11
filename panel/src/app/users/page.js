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
            <a href={link.href} className={currentPage === link.href ? 'active' : ''}>
              <span className="nav-icon">{link.icon}</span>
              {link.label}
            </a>
          </li>
        ))}
      </ul>
    </nav>
  );
}

export default function UsersPage() {
  const [nodes, setNodes] = useState([]);
  const [selectedNode, setSelectedNode] = useState(null);
  const [users, setUsers] = useState([]);
  const [loading, setLoading] = useState(true);
  const [showAddModal, setShowAddModal] = useState(false);
  const [showInfoModal, setShowInfoModal] = useState(false);
  const [selectedUser, setSelectedUser] = useState(null);
  const [newUsername, setNewUsername] = useState('');
  const [toast, setToast] = useState(null);
  const [userConfig, setUserConfig] = useState('');

  const showToast = (msg, type = 'success') => {
    setToast({ msg, type });
    setTimeout(() => setToast(null), 3000);
  };

  // Proxy request to a node through our panel API
  const nodeRequest = useCallback(async (nodeId, method, path, body = null) => {
    const res = await fetch('/api/proxy', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ nodeId, method, path, body }),
    });
    return res.json();
  }, []);

  // Load nodes
  useEffect(() => {
    fetch('/api/nodes').then(r => r.json()).then(data => {
      setNodes(data.filter(n => n.online));
      if (data.length > 0 && data[0].online) {
        setSelectedNode(data[0]);
      }
      setLoading(false);
    });
  }, []);

  // Load users when node changes
  const fetchUsers = useCallback(async () => {
    if (!selectedNode) return;
    try {
      const data = await nodeRequest(selectedNode.id, 'GET', '/api/v1/users');
      setUsers(Array.isArray(data) ? data : []);
    } catch (e) {
      console.error(e);
      setUsers([]);
    }
  }, [selectedNode, nodeRequest]);

  useEffect(() => {
    if (selectedNode) fetchUsers();
  }, [selectedNode, fetchUsers]);

  const addUser = async () => {
    if (!newUsername.trim() || !selectedNode) return;
    try {
      const data = await nodeRequest(selectedNode.id, 'POST', '/api/v1/users', { username: newUsername.trim() });
      if (data.error) {
        showToast(data.error, 'error');
        return;
      }
      setShowAddModal(false);
      setNewUsername('');
      showToast(`User "${data.username}" created!`);
      fetchUsers();
    } catch (e) {
      showToast(e.message, 'error');
    }
  };

  const toggleUser = async (username, enabled) => {
    const action = enabled ? 'disable' : 'enable';
    await nodeRequest(selectedNode.id, 'POST', `/api/v1/users/${username}/${action}`);
    showToast(`User "${username}" ${action}d`);
    fetchUsers();
  };

  const deleteUser = async (username) => {
    if (!confirm(`确认要删除代理用户 "${username}" 吗？`)) return;
    await nodeRequest(selectedNode.id, 'DELETE', `/api/v1/users/${username}`);
    showToast(`用户 "${username}" 已删除`);
    fetchUsers();
  };

  const viewUserInfo = async (user) => {
    setSelectedUser(user);
    try {
      // Fetch YAML directly from our panel's API
      const res = await fetch(`/api/sub/${selectedNode.id}/${user.sub_token}`);
      if (res.ok) {
        setUserConfig(await res.text());
      } else {
        setUserConfig('Error: ' + await res.text());
      }
    } catch (e) {
      setUserConfig('Failed to load config: ' + e.message);
    }
    setShowInfoModal(true);
  };

  const resetTraffic = async (username) => {
    await nodeRequest(selectedNode.id, 'POST', `/api/v1/users/${username}/reset-traffic`);
    showToast(`Traffic reset for "${username}"`);
    fetchUsers();
  };

  const copyToClipboard = (text) => {
    navigator.clipboard.writeText(text);
    showToast('已复制到剪贴板！');
  };

  return (
    <div className="app-layout">
      <Sidebar currentPage="/users" />
      <main className="main-content">
        <div className="page-header page-header-row">
          <div>
            <h2>用户管理</h2>
            <p>为你的代理节点分配和管理使用者</p>
          </div>
          <div className="btn-group">
            {nodes.length > 1 && (
              <select className="form-input" style={{ width: 'auto' }}
                value={selectedNode?.id || ''}
                onChange={(e) => setSelectedNode(nodes.find(n => n.id === parseInt(e.target.value)))}>
                {nodes.map(n => <option key={n.id} value={n.id}>{n.name}</option>)}
              </select>
            )}
            <button className="btn btn-primary" onClick={() => setShowAddModal(true)}
              disabled={!selectedNode}>
              + 新建用户
            </button>
          </div>
        </div>

        {loading ? (
          <div className="loading"><div className="spinner"></div>正在加载...</div>
        ) : !selectedNode ? (
          <div className="empty-state">
            <div className="icon">🖥️</div>
            <h3>没有可用的在线节点</h3>
            <p>请先在「节点管理」中配对连通一台服务器。</p>
          </div>
        ) : users.length === 0 ? (
          <div className="empty-state">
            <div className="icon">👥</div>
            <h3>这台节点下暂无用户</h3>
            <p>点击上方 "新建用户" 按钮立刻创建。</p>
          </div>
        ) : (
          <div className="table-container">
            <table>
              <thead>
                <tr>
                  <th>使用者账号</th>
                  <th>状态</th>
                  <th>已上传</th>
                  <th>已下载</th>
                  <th>总用量</th>
                  <th>常规操作</th>
                </tr>
              </thead>
              <tbody>
                {users.map((u) => (
                  <tr key={u.id}>
                    <td><strong>{u.username}</strong></td>
                    <td>
                      <span className={`badge ${u.enabled ? 'badge-success' : 'badge-danger'}`}>
                        <span className="badge-dot"></span>
                        {u.enabled ? '正常启用' : '已停用禁用'}
                      </span>
                    </td>
                    <td>{formatBytes(u.traffic_up)}</td>
                    <td>{formatBytes(u.traffic_down)}</td>
                    <td><strong>{formatBytes(u.traffic_up + u.traffic_down)}</strong></td>
                    <td>
                      <div className="btn-group">
                        <button className="btn btn-secondary btn-sm" onClick={() => viewUserInfo(u)}>
                          详情/订阅
                        </button>
                        <button
                          className={`btn btn-sm ${u.enabled ? 'btn-danger' : 'btn-success'}`}
                          onClick={() => toggleUser(u.username, u.enabled)}
                        >
                          {u.enabled ? '停用' : '启用'}
                        </button>
                        <button className="btn btn-secondary btn-sm" onClick={() => resetTraffic(u.username)}>
                          清空流量
                        </button>
                        <button className="btn btn-danger btn-sm" onClick={() => deleteUser(u.username)}>
                          ✕
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {showAddModal && (
          <div className="modal-overlay" onClick={() => setShowAddModal(false)}>
            <div className="modal" onClick={(e) => e.stopPropagation()}>
              <h3>新建代理用户</h3>
              <p style={{ color: 'var(--text-muted)', fontSize: '14px', marginBottom: '20px' }}>
                将下发至节点：{selectedNode?.name}
              </p>
              <div className="form-group">
                <label>使用者标识(纯字母)</label>
                <input className="form-input" placeholder="例如：alice, iphone"
                  value={newUsername}
                  onChange={(e) => setNewUsername(e.target.value)}
                  onKeyDown={(e) => e.key === 'Enter' && addUser()} />
              </div>
              <div className="modal-actions">
                <button className="btn btn-secondary" onClick={() => setShowAddModal(false)}>取消</button>
                <button className="btn btn-primary" onClick={addUser}>创建下发</button>
              </div>
            </div>
          </div>
        )}

        {showInfoModal && selectedUser && (
          <div className="modal-overlay" onClick={() => setShowInfoModal(false)}>
            <div className="modal" onClick={(e) => e.stopPropagation()} style={{ maxWidth: '600px' }}>
              <h3>使用者详情：{selectedUser.username}</h3>

              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px', marginBottom: '20px' }}>
                <div>
                  <label style={{ fontSize: '12px', color: 'var(--text-muted)' }}>VLESS 专用 UUID 凭证</label>
                  <div className="copy-text" onClick={() => copyToClipboard(selectedUser.uuid)}>
                    {selectedUser.uuid}
                  </div>
                </div>
                <div>
                  <label style={{ fontSize: '12px', color: 'var(--text-muted)' }}>Hysteria 2 专用密码</label>
                  <div className="copy-text" onClick={() => copyToClipboard(selectedUser.hy2_password)}>
                    {selectedUser.hy2_password}
                  </div>
                </div>
              </div>

              {selectedUser.sub_token && (
                <div style={{ marginBottom: '20px' }}>
                  <label style={{ fontSize: '12px', color: 'var(--text-muted)' }}>一键订阅链接 (导入各类客户端)</label>
                  <div className="copy-text" style={{ maxWidth: '100%' }}
                    onClick={() => copyToClipboard(`${window.location.origin}/api/sub/${selectedNode?.id}/${selectedUser.sub_token}`)}>
                    {window.location.origin}/api/sub/{selectedNode?.id}/{selectedUser.sub_token}
                  </div>
                </div>
              )}

              <div>
                <label style={{ fontSize: '12px', color: 'var(--text-muted)' }}>Clash / Meta 裸客户端配置 (自动生成)</label>
                <pre style={{
                  background: 'var(--bg-primary)',
                  border: '1px solid var(--border)',
                  borderRadius: '8px',
                  padding: '16px',
                  fontSize: '12px',
                  fontFamily: 'monospace',
                  color: 'var(--text-secondary)',
                  overflow: 'auto',
                  maxHeight: '300px',
                  cursor: 'pointer',
                  marginTop: '6px',
                }} onClick={() => copyToClipboard(userConfig)}>
                  {userConfig}
                </pre>
              </div>

              <div className="modal-actions">
                <button className="btn btn-secondary" onClick={() => setShowInfoModal(false)}>关闭</button>
                <button className="btn btn-primary" onClick={() => copyToClipboard(userConfig)}>
                  一键拷贝配置
                </button>
              </div>
            </div>
          </div>
        )}

        {toast && <div className={`toast toast-${toast.type}`}>{toast.msg}</div>}
      </main>
    </div>
  );
}
