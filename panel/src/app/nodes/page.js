'use client';
import { useState, useEffect, useCallback } from 'react';
import { useAdminAuth, AdminSidebar, authFetch } from '../../lib/adminAuth';

export default function NodesPage() {
  const { ready } = useAdminAuth();
  const [nodes, setNodes] = useState([]);
  const [loading, setLoading] = useState(true);
  const [showModal, setShowModal] = useState(false);
  const [editingNodeId, setEditingNodeId] = useState(null);
  const [form, setForm] = useState({ name: '', address: '', admin_token: '' });
  const [isSaving, setIsSaving] = useState(false);
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
      setError('所有字段均为必填项目');
      return;
    }

    // Normalize address
    let addr = form.address.trim();
    if (!addr.startsWith('http')) {
      addr = `http://${addr}`;
    }

    setIsSaving(true);
    try {
      const isEditing = editingNodeId !== null;
      const res = await authFetch('/api/nodes', {
        method: isEditing ? 'PUT' : 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          ...(isEditing && { id: editingNodeId }),
          ...form,
          address: addr
        }),
      });
      const data = await res.json();
      if (!res.ok) {
        setError(data.error);
        return;
      }
      setShowModal(false);
      setEditingNodeId(null);
      setForm({ name: '', address: '', admin_token: '' });
      showToast(isEditing ? '节点更新成功！' : '节点添加成功！');
      fetchNodes();
    } catch (e) {
      setError(e.message);
    } finally {
      setIsSaving(false);
    }
  };

  const handleAddClick = () => {
    setForm({ name: '', address: '', admin_token: '' });
    setEditingNodeId(null);
    setError('');
    setShowModal(true);
  };

  const handleEditClick = (node) => {
    setForm({ name: node.name, address: node.address.replace(/^http:\/\//, ''), admin_token: '' });
    setEditingNodeId(node.id);
    setError('');
    setShowModal(true);
  };

  const deleteNode = async (id, name) => {
    if (!confirm(`确实要删除节点 "${name}" 吗？该操作不可逆转！`)) return;
    await authFetch(`/api/nodes?id=${id}`, { method: 'DELETE' });
    showToast('节点已移除');
    fetchNodes();
  };

  if (!ready) return null;

  return (
    <div className="app-layout">
      <AdminSidebar currentPage="/nodes" />
      <main className="main-content">
        <div className="page-header page-header-row">
          <div>
            <h2>节点管理</h2>
            <p>连接并管理远端的代理服务器集群</p>
          </div>
          <button className="btn btn-primary" onClick={handleAddClick}>
            + 添加节点
          </button>
        </div>

        {loading ? (
          <div className="loading"><div className="spinner"></div>正在加载...</div>
        ) : nodes.length === 0 ? (
          <div className="empty-state">
            <div className="icon">🖥️</div>
            <h3>当前还没有节点</h3>
            <p>点击上方按钮添加你的第一台服务器。</p>
          </div>
        ) : (
          <div className="table-container">
            <table>
              <thead>
                <tr>
                  <th>别名</th>
                  <th>连接地址</th>
                  <th>健康状态</th>
                  <th>VLESS 协议</th>
                  <th>Hy2 协议</th>
                  <th>承载用户数</th>
                  <th>操作</th>
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
                        {node.online ? '在线可用' : '离线/失联'}
                      </span>
                    </td>
                    <td>
                      {node.status?.vless?.installed ? (
                        <span className={`badge ${node.status.vless.running ? 'badge-success' : 'badge-warning'}`}>
                          {node.status.vless.running ? '运行中' : '已停止'}
                        </span>
                      ) : <span className="badge">—</span>}
                    </td>
                    <td>
                      {node.status?.hysteria2?.installed ? (
                        <span className={`badge ${node.status.hysteria2.running ? 'badge-success' : 'badge-warning'}`}>
                          {node.status.hysteria2.running ? '运行中' : '已停止'}
                        </span>
                      ) : <span className="badge">—</span>}
                    </td>
                    <td>{node.status?.total_users || 0}</td>
                    <td>
                      <div style={{ display: 'flex', gap: '8px' }}>
                        <button className="btn btn-secondary btn-sm" onClick={() => handleEditClick(node)}>
                          编辑
                        </button>
                        <button className="btn btn-danger btn-sm" onClick={() => deleteNode(node.id, node.name)}>
                          移除
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {showModal && (
          <div className="modal-overlay" onClick={() => { setShowModal(false); setEditingNodeId(null); }}>
            <div className="modal" onClick={(e) => e.stopPropagation()}>
              <h3>{editingNodeId ? '编辑节点' : '添加新节点'}</h3>
              {error && <div style={{ color: 'var(--danger)', fontSize: '14px', marginBottom: '16px' }}>{error}</div>}
              <div className="form-group">
                <label>给节点起个易记的名称</label>
                <input className="form-input" placeholder="例如：美国硅谷高级优化线路" value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })} />
              </div>
              <div className="form-group">
                <label>通信地址 (IP:端口)</label>
                <input className="form-input" placeholder="例如：16.148.174.149:9090" value={form.address}
                  onChange={(e) => setForm({ ...form, address: e.target.value })} />
              </div>
              <div className="form-group">
                <label>管理员凭证 (Admin Token)</label>
                <input className="form-input" placeholder="在远端运行 pnm token show 获取" value={form.admin_token}
                  onChange={(e) => setForm({ ...form, admin_token: e.target.value })} />
              </div>
              <div className="modal-actions">
                <button className="btn btn-secondary" onClick={() => { setShowModal(false); setEditingNodeId(null); }} disabled={isSaving}>取消</button>
                <button className="btn btn-primary" onClick={addNode} disabled={isSaving}>
                  {isSaving ? '正在测试并保存...' : (editingNodeId ? '保存并测试' : '检查并连接')}
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
