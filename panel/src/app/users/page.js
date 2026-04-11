'use client';
import { useState, useEffect, useCallback } from 'react';
import { useAdminAuth, AdminSidebar, authFetch } from '../../lib/adminAuth';

function formatBytes(bytes) {
  if (!bytes || bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

export default function UsersPage() {
  const { ready } = useAdminAuth();
  const [nodes, setNodes] = useState([]);
  const [selectedNode, setSelectedNode] = useState('all');
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

  // Proxy request to a node through our panel API (with auth)
  const nodeRequest = useCallback(async (nodeId, method, path, body = null) => {
    try {
      const res = await authFetch('/api/proxy', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ nodeId, method, path, body }),
      });
      return await res.json();
    } catch (e) {
      console.error(`Node ${nodeId} request failed:`, e);
      return { error: 'Connection failed' };
    }
  }, []);

  // Load nodes
  useEffect(() => {
    fetch('/api/nodes').then(r => r.json()).then(data => {
      const onlineNodes = data.filter(n => n.online);
      setNodes(onlineNodes);
      setLoading(false);
    });
  }, []);

  // Aggregation logic
  const fetchUsers = useCallback(async () => {
    setLoading(true);
    try {
      if (selectedNode === 'all') {
        const allResults = await Promise.all(
          nodes.map(async (n) => {
            const data = await nodeRequest(n.id, 'GET', '/api/v1/users');
            return { node: n, users: Array.isArray(data) ? data : [] };
          })
        );

        const aggregated = {};
        allResults.forEach(({ node, users }) => {
          users.forEach(u => {
            if (!aggregated[u.username]) {
              aggregated[u.username] = { 
                ...u, 
                traffic_up: 0, 
                traffic_down: 0, 
                nodes: [] 
              };
            }
            aggregated[u.username].traffic_up += u.traffic_up;
            aggregated[u.username].traffic_down += u.traffic_down;
            aggregated[u.username].nodes.push({ 
              name: node.name, 
              traffic_up: u.traffic_up, 
              traffic_down: u.traffic_down,
              nodeId: node.id,
              sub_token: u.sub_token
            });
          });
        });
        setUsers(Object.values(aggregated));
      } else {
        const data = await nodeRequest(selectedNode.id, 'GET', '/api/v1/users');
        const list = Array.isArray(data) ? data : [];
        // Map to include singular node info for details modal consistency
        const mapped = list.map(u => ({
          ...u,
          nodes: [{ 
            name: selectedNode.name, 
            traffic_up: u.traffic_up, 
            traffic_down: u.traffic_down,
            nodeId: selectedNode.id,
            sub_token: u.sub_token
          }]
        }));
        setUsers(mapped);
      }
    } catch (e) {
      console.error(e);
      setUsers([]);
    } finally {
      setLoading(false);
    }
  }, [selectedNode, nodes, nodeRequest]);

  useEffect(() => {
    if (nodes.length >= 0) fetchUsers();
  }, [selectedNode, nodes.length, fetchUsers]);

  const addUser = async () => {
    if (!newUsername.trim() || selectedNode === 'all') return;
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
    setUserConfig('正在加载首选节点配置...');
    setShowInfoModal(true);
    
    try {
      // For multi-node users, we use the first node's config as a sample
      const preferredNode = user.nodes[0];
      const res = await fetch(`/api/sub/${preferredNode.nodeId}/${preferredNode.sub_token}`);
      if (res.ok) {
        setUserConfig(await res.text());
      } else {
        setUserConfig('Error: ' + await res.text());
      }
    } catch (e) {
      setUserConfig('Failed to load config: ' + e.message);
    }
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

  if (!ready) return null;

  return (
    <div className="app-layout">
      <AdminSidebar currentPage="/users" />
      <main className="main-content">
        <div className="page-header page-header-row">
          <div>
            <h2>用户管理</h2>
            <p>为你的代理节点分配和管理使用者</p>
          </div>
          <div className="btn-group">
            <select className="form-input" style={{ width: 'auto' }}
              value={selectedNode === 'all' ? 'all' : selectedNode?.id || ''}
              onChange={(e) => {
                const val = e.target.value;
                if (val === 'all') setSelectedNode('all');
                else setSelectedNode(nodes.find(n => n.id === parseInt(val)));
              }}>
              <option value="all">全部节点 (在线汇总)</option>
              {nodes.map(n => <option key={n.id} value={n.id}>{n.name}</option>)}
            </select>
            
            <button className="btn btn-primary" onClick={() => setShowAddModal(true)}
              disabled={selectedNode === 'all'}>
              + 新建用户
            </button>
          </div>
        </div>

        {loading && users.length === 0 ? (
          <div className="loading"><div className="spinner"></div>正在加载...</div>
        ) : nodes.length === 0 ? (
          <div className="empty-state">
            <div className="icon">🖥️</div>
            <h3>没有可用的在线节点</h3>
            <p>请先在「节点管理」中配对连通一台服务器。</p>
          </div>
        ) : users.length === 0 ? (
          <div className="empty-state">
            <div className="icon">👥</div>
            <h3>暂无用户数据</h3>
            <p>{selectedNode === 'all' ? '所有节点上都没有发现用户。' : '这台节点下暂无用户，点击 "新建用户" 按钮。'}</p>
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
                  <tr key={u.username}>
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
                        {selectedNode !== 'all' && (
                          <>
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
                          </>
                        )}
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

              {selectedUser.nodes && selectedUser.nodes.length > 0 && (
                <div style={{ marginBottom: '20px' }}>
                  <label style={{ fontSize: '12px', color: 'var(--text-muted)' }}>多节点流量消耗详情</label>
                  <div className="table-container" style={{ marginTop: '8px', maxHeight: '150px', overflowY: 'auto' }}>
                    <table style={{ fontSize: '12px' }}>
                      <thead style={{ position: 'sticky', top: 0, background: 'var(--bg-card)' }}>
                        <tr>
                          <th>节点名称</th>
                          <th>上传</th>
                          <th>下载</th>
                          <th>一键订阅</th>
                        </tr>
                      </thead>
                      <tbody>
                        {selectedUser.nodes.map((n, idx) => (
                          <tr key={idx}>
                            <td>{n.name}</td>
                            <td>{formatBytes(n.traffic_up)}</td>
                            <td>{formatBytes(n.traffic_down)}</td>
                            <td>
                              <button className="btn btn-secondary btn-sm" style={{ padding: '2px 8px', fontSize: '10px' }}
                                onClick={() => copyToClipboard(`${window.location.origin}/api/sub/${n.nodeId}/${n.sub_token}`)}>
                                拷贝
                              </button>
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
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
                  maxHeight: '200px',
                  cursor: 'pointer',
                  marginTop: '6px',
                }} onClick={() => copyToClipboard(userConfig)}>
                  {userConfig}
                </pre>
              </div>

              <div className="modal-actions">
                <button className="btn btn-secondary" onClick={() => setShowInfoModal(false)}>关闭</button>
                <button className="btn btn-primary" onClick={() => copyToClipboard(userConfig)}>
                  拷贝当前节点配置
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
