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
    if (!confirm(`Delete user "${username}"?`)) return;
    await nodeRequest(selectedNode.id, 'DELETE', `/api/v1/users/${username}`);
    showToast(`User "${username}" deleted`);
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
    showToast('Copied to clipboard!');
  };

  return (
    <div className="app-layout">
      <Sidebar currentPage="/users" />
      <main className="main-content">
        <div className="page-header page-header-row">
          <div>
            <h2>Users</h2>
            <p>Manage users on your proxy nodes</p>
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
              + Add User
            </button>
          </div>
        </div>

        {loading ? (
          <div className="loading"><div className="spinner"></div>Loading...</div>
        ) : !selectedNode ? (
          <div className="empty-state">
            <div className="icon">🖥️</div>
            <h3>No online nodes</h3>
            <p>Add and connect a node first.</p>
          </div>
        ) : users.length === 0 ? (
          <div className="empty-state">
            <div className="icon">👥</div>
            <h3>No users on this node</h3>
            <p>Click "Add User" to create one.</p>
          </div>
        ) : (
          <div className="table-container">
            <table>
              <thead>
                <tr>
                  <th>Username</th>
                  <th>Status</th>
                  <th>Upload</th>
                  <th>Download</th>
                  <th>Total</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {users.map((u) => (
                  <tr key={u.id}>
                    <td><strong>{u.username}</strong></td>
                    <td>
                      <span className={`badge ${u.enabled ? 'badge-success' : 'badge-danger'}`}>
                        <span className="badge-dot"></span>
                        {u.enabled ? 'Active' : 'Disabled'}
                      </span>
                    </td>
                    <td>{formatBytes(u.traffic_up)}</td>
                    <td>{formatBytes(u.traffic_down)}</td>
                    <td><strong>{formatBytes(u.traffic_up + u.traffic_down)}</strong></td>
                    <td>
                      <div className="btn-group">
                        <button className="btn btn-secondary btn-sm" onClick={() => viewUserInfo(u)}>
                          Info
                        </button>
                        <button
                          className={`btn btn-sm ${u.enabled ? 'btn-danger' : 'btn-success'}`}
                          onClick={() => toggleUser(u.username, u.enabled)}
                        >
                          {u.enabled ? 'Disable' : 'Enable'}
                        </button>
                        <button className="btn btn-secondary btn-sm" onClick={() => resetTraffic(u.username)}>
                          Reset
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

        {/* Add User Modal */}
        {showAddModal && (
          <div className="modal-overlay" onClick={() => setShowAddModal(false)}>
            <div className="modal" onClick={(e) => e.stopPropagation()}>
              <h3>Add User</h3>
              <p style={{ color: 'var(--text-muted)', fontSize: '14px', marginBottom: '20px' }}>
                Node: {selectedNode?.name}
              </p>
              <div className="form-group">
                <label>Username</label>
                <input className="form-input" placeholder="e.g. alice"
                  value={newUsername}
                  onChange={(e) => setNewUsername(e.target.value)}
                  onKeyDown={(e) => e.key === 'Enter' && addUser()} />
              </div>
              <div className="modal-actions">
                <button className="btn btn-secondary" onClick={() => setShowAddModal(false)}>Cancel</button>
                <button className="btn btn-primary" onClick={addUser}>Create</button>
              </div>
            </div>
          </div>
        )}

        {/* User Info Modal */}
        {showInfoModal && selectedUser && (
          <div className="modal-overlay" onClick={() => setShowInfoModal(false)}>
            <div className="modal" onClick={(e) => e.stopPropagation()} style={{ maxWidth: '600px' }}>
              <h3>User: {selectedUser.username}</h3>

              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px', marginBottom: '20px' }}>
                <div>
                  <label style={{ fontSize: '12px', color: 'var(--text-muted)' }}>VLESS UUID</label>
                  <div className="copy-text" onClick={() => copyToClipboard(selectedUser.uuid)}>
                    {selectedUser.uuid}
                  </div>
                </div>
                <div>
                  <label style={{ fontSize: '12px', color: 'var(--text-muted)' }}>Hy2 Password</label>
                  <div className="copy-text" onClick={() => copyToClipboard(selectedUser.hy2_password)}>
                    {selectedUser.hy2_password}
                  </div>
                </div>
              </div>

              {selectedUser.sub_token && (
                <div style={{ marginBottom: '20px' }}>
                  <label style={{ fontSize: '12px', color: 'var(--text-muted)' }}>Subscription URL</label>
                  <div className="copy-text" style={{ maxWidth: '100%' }}
                    onClick={() => copyToClipboard(`${window.location.origin}/api/sub/${selectedNode?.id}/${selectedUser.sub_token}`)}>
                    {window.location.origin}/api/sub/{selectedNode?.id}/{selectedUser.sub_token}
                  </div>
                </div>
              )}

              <div>
                <label style={{ fontSize: '12px', color: 'var(--text-muted)' }}>Clash Client Config (YAML)</label>
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
                <button className="btn btn-secondary" onClick={() => setShowInfoModal(false)}>Close</button>
                <button className="btn btn-primary" onClick={() => copyToClipboard(userConfig)}>
                  Copy Config
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
