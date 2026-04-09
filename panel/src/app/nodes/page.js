'use client';
import { useState, useEffect, useCallback } from 'react';

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

export default function NodesPage() {
  const [nodes, setNodes] = useState([]);
  const [loading, setLoading] = useState(true);
  const [showModal, setShowModal] = useState(false);
  const [form, setForm] = useState({ name: '', address: '', admin_token: '' });
  const [error, setError] = useState('');
  const [toast, setToast] = useState(null);

  const fetchNodes = useCallback(async () => {
    try {
      const res = await fetch('/api/nodes');
      setNodes(await res.json());
    } catch (e) {
      console.error(e);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchNodes();
  }, [fetchNodes]);

  const showToast = (msg, type = 'success') => {
    setToast({ msg, type });
    setTimeout(() => setToast(null), 3000);
  };

  const addNode = async () => {
    setError('');
    if (!form.name || !form.address || !form.admin_token) {
      setError('All fields are required');
      return;
    }

    // Normalize address
    let addr = form.address.trim();
    if (!addr.startsWith('http')) {
      addr = `http://${addr}`;
    }

    try {
      const res = await fetch('/api/nodes', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ...form, address: addr }),
      });
      const data = await res.json();
      if (!res.ok) {
        setError(data.error);
        return;
      }
      setShowModal(false);
      setForm({ name: '', address: '', admin_token: '' });
      showToast('Node added successfully!');
      fetchNodes();
    } catch (e) {
      setError(e.message);
    }
  };

  const deleteNode = async (id, name) => {
    if (!confirm(`Delete node "${name}"?`)) return;
    await fetch(`/api/nodes?id=${id}`, { method: 'DELETE' });
    showToast('Node removed');
    fetchNodes();
  };

  return (
    <div className="app-layout">
      <Sidebar currentPage="/nodes" />
      <main className="main-content">
        <div className="page-header page-header-row">
          <div>
            <h2>Nodes</h2>
            <p>Manage your proxy server nodes</p>
          </div>
          <button className="btn btn-primary" onClick={() => setShowModal(true)}>
            + Add Node
          </button>
        </div>

        {loading ? (
          <div className="loading"><div className="spinner"></div>Loading...</div>
        ) : nodes.length === 0 ? (
          <div className="empty-state">
            <div className="icon">🖥️</div>
            <h3>No nodes yet</h3>
            <p>Add your first node to get started.</p>
          </div>
        ) : (
          <div className="table-container">
            <table>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Address</th>
                  <th>Status</th>
                  <th>VLESS</th>
                  <th>Hy2</th>
                  <th>Users</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {nodes.map((node) => (
                  <tr key={node.id}>
                    <td><strong>{node.name}</strong></td>
                    <td><span className="mono">{node.status?.server_ip || node.address}</span></td>
                    <td>
                      <span className={`badge ${node.online ? 'badge-success' : 'badge-danger'}`}>
                        <span className="badge-dot"></span>
                        {node.online ? 'Online' : 'Offline'}
                      </span>
                    </td>
                    <td>
                      {node.status?.vless?.installed ? (
                        <span className={`badge ${node.status.vless.running ? 'badge-success' : 'badge-warning'}`}>
                          {node.status.vless.running ? 'Running' : 'Stopped'}
                        </span>
                      ) : <span className="badge">—</span>}
                    </td>
                    <td>
                      {node.status?.hysteria2?.installed ? (
                        <span className={`badge ${node.status.hysteria2.running ? 'badge-success' : 'badge-warning'}`}>
                          {node.status.hysteria2.running ? 'Running' : 'Stopped'}
                        </span>
                      ) : <span className="badge">—</span>}
                    </td>
                    <td>{node.status?.total_users || 0}</td>
                    <td>
                      <button className="btn btn-danger btn-sm" onClick={() => deleteNode(node.id, node.name)}>
                        Delete
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {showModal && (
          <div className="modal-overlay" onClick={() => setShowModal(false)}>
            <div className="modal" onClick={(e) => e.stopPropagation()}>
              <h3>Add Node</h3>
              {error && <div style={{ color: 'var(--danger)', fontSize: '14px', marginBottom: '16px' }}>{error}</div>}
              <div className="form-group">
                <label>Name</label>
                <input className="form-input" placeholder="e.g. US-West-1" value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })} />
              </div>
              <div className="form-group">
                <label>Address (IP:Port)</label>
                <input className="form-input" placeholder="e.g. 16.148.174.149:9090" value={form.address}
                  onChange={(e) => setForm({ ...form, address: e.target.value })} />
              </div>
              <div className="form-group">
                <label>Admin Token</label>
                <input className="form-input" placeholder="from: pnm token show" value={form.admin_token}
                  onChange={(e) => setForm({ ...form, admin_token: e.target.value })} />
              </div>
              <div className="modal-actions">
                <button className="btn btn-secondary" onClick={() => setShowModal(false)}>Cancel</button>
                <button className="btn btn-primary" onClick={addNode}>Connect</button>
              </div>
            </div>
          </div>
        )}

        {toast && <div className={`toast toast-${toast.type}`}>{toast.msg}</div>}
      </main>
    </div>
  );
}
