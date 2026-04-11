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
  const [users, setUsers] = useState([]);
  const [panelAccounts, setPanelAccounts] = useState([]);
  const [loading, setLoading] = useState(true);
  const [showAddModal, setShowAddModal] = useState(false);
  const [showInfoModal, setShowInfoModal] = useState(false);
  const [selectedUser, setSelectedUser] = useState(null);
  const [newUsername, setNewUsername] = useState('');
  const [createPanelAccount, setCreatePanelAccount] = useState(false);
  const [newPassword, setNewPassword] = useState('');
  const [toast, setToast] = useState(null);
  const [syncing, setSyncing] = useState(false);
  const [resetPwd, setResetPwd] = useState('');

  const showToast = (msg, type = 'success') => {
    setToast({ msg, type });
    setTimeout(() => setToast(null), 3000);
  };

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

  const fetchUsers = useCallback(async (currentNodes) => {
    setLoading(true);
    try {
      // Fetch proxy users across all nodes
      const allResults = await Promise.all(
        currentNodes.map(async (n) => {
          const data = await nodeRequest(n.id, 'GET', '/api/v1/users');
          return { node: n, users: Array.isArray(data) ? data : [] };
        })
      );

      // Fetch panel accounts to get global sub_tokens
      const panelRes = await authFetch('/api/panel-users');
      const panelData = await panelRes.json();
      setPanelAccounts(Array.isArray(panelData) ? panelData : []);

      // Aggregate
      const aggregated = {};
      allResults.forEach(({ node, users }) => {
        users.forEach(u => {
          if (!aggregated[u.username]) {
            aggregated[u.username] = { 
              username: u.username,
              traffic_up: 0, 
              traffic_down: 0, 
              nodes: [] 
            };
          }
          aggregated[u.username].traffic_up += u.traffic_up;
          aggregated[u.username].traffic_down += u.traffic_down;
          // Store raw node info
          aggregated[u.username].nodes.push({ 
            nodeId: node.id,
            name: node.name, 
            traffic_up: u.traffic_up, 
            traffic_down: u.traffic_down,
            enabled: u.enabled,
            sub_token: u.sub_token, // Legacy node token
            uuid: u.uuid,
            hy2_password: u.hy2_password
          });
        });
      });
      setUsers(Object.values(aggregated));
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  }, [nodeRequest]);

  useEffect(() => {
    if (!ready) return;
    fetch('/api/nodes').then(r => r.json()).then(data => {
      const onlineNodes = data.filter(n => n.online);
      setNodes(onlineNodes);
      fetchUsers(onlineNodes);
    });
  }, [ready, fetchUsers]);

  const addGlobalUser = async () => {
    if (!newUsername.trim()) return;
    try {
      const reqs = nodes.map(n => nodeRequest(n.id, 'POST', '/api/v1/users', { username: newUsername.trim() }));
      await Promise.all(reqs);

      // Create panel auth account if requested
      if (createPanelAccount) {
        if (!newPassword || newPassword.length < 4) {
          showToast('密码不能少于 4 位', 'error');
          return;
        }
        const panelRes = await authFetch('/api/panel-users', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            username: newUsername.trim(),
            password: newPassword,
            role: 'user',
            proxy_username: newUsername.trim()
          })
        });
        const pData = await panelRes.json();
        if (!panelRes.ok && pData.error && !pData.error.includes("已存在")) {
             throw new Error("面板账号创建失败: " + pData.error);
        }
      }

      setShowAddModal(false);
      setNewUsername('');
      setNewPassword('');
      setCreatePanelAccount(false);
      showToast(`User "${newUsername}" globally created!`);
      fetchUsers(nodes);
    } catch (e) {
      showToast(e.message, 'error');
    }
  };

  const deleteGlobalUser = async (username) => {
    if (!confirm(`确认要彻底抹除代理用户 "${username}" 吗？这将在所有节点上删除该用户！`)) return;
    try {
      const reqs = nodes.map(n => nodeRequest(n.id, 'DELETE', `/api/v1/users/${username}`));
      await Promise.all(reqs);
      showToast(`用户 "${username}" 已从所有节点彻底删除`);
      fetchUsers(nodes);
    } catch(e) {
      showToast(e.message, 'error');
    }
  };

  const toggleUserOnNode = async (nodeId, username, currentlyEnabled) => {
    const action = currentlyEnabled ? 'disable' : 'enable';
    await nodeRequest(nodeId, 'POST', `/api/v1/users/${username}/${action}`);
    showToast(`在指定节点已${currentlyEnabled ? '停用' : '启用'}该用户`);
    fetchUsers(nodes);
    // UI hack to close modal so it refreshes properly with new node info
    setShowInfoModal(false);
  };

  const handeResetPassword = async (panelAccountId) => {
    if (!resetPwd || resetPwd.length < 4) {
      showToast('新密码长度不能少于 4 位', 'error');
      return;
    }
    const pAcc = panelAccounts.find(p => p.id === panelAccountId);
    if (!pAcc) return;
    try {
      const res = await authFetch('/api/panel-users', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          id: pAcc.id,
          username: pAcc.username,
          new_password: resetPwd
        })
      });
      if (res.ok) {
        showToast('面板账号密码修改成功！');
        setResetPwd('');
      } else {
        const data = await res.json();
        showToast(data.error || '修改失败', 'error');
      }
    } catch (e) {
      showToast(e.message, 'error');
    }
  };

  // Syncs all existing users to any potentially missing node
  const syncGlobalUsers = async () => {
    if (!confirm('这会收集当前所有的用户，并下发到可能遗漏的新节点。确定继续吗？')) return;
    setSyncing(true);
    try {
      const allUsernames = [...new Set(users.map(u => u.username))];
      for (const username of allUsernames) {
        // Send a create request to ALL nodes. Nodes that already have it will essentially ignore or harmlessly update.
        await Promise.all(nodes.map(n => nodeRequest(n.id, 'POST', '/api/v1/users', { username })));
      }
      showToast('所有用户已同步至所有节点！');
      fetchUsers(nodes);
    } catch (e) {
      showToast('同步期间发生错误: ' + e.message, 'error');
    } finally {
      setSyncing(false);
    }
  };

  const copyToClipboard = (text) => {
    navigator.clipboard.writeText(text);
    showToast('已复制！');
  };

  if (!ready) return null;

  return (
    <div className="app-layout">
      <AdminSidebar currentPage="/users" />
      <main className="main-content">
        <div className="page-header page-header-row">
          <div>
            <h2>全局用户统筹</h2>
            <p>从大局视角，一键管理所有节点下的使用者</p>
          </div>
          <div style={{ display: 'flex', gap: '12px' }}>
             <button className="btn btn-secondary" onClick={syncGlobalUsers} disabled={syncing}>
              {syncing ? '同步中...' : '⟳ 补齐所有用户'}
            </button>
            <button className="btn btn-primary" onClick={() => setShowAddModal(true)} disabled={nodes.length === 0}>
              + 全局下发用户
            </button>
          </div>
        </div>

        {loading ? (
          <div className="loading"><div className="spinner"></div>聚合在线节点中...</div>
        ) : users.length === 0 ? (
          <div className="empty-state">
            <div className="icon">👥</div>
            <h3>暂无代理用户</h3>
            <p>点击“全局下发用户”按钮，快速在所有集群内创建第一个账号。</p>
          </div>
        ) : (
          <div className="table-container">
            <table>
              <thead>
                <tr>
                  <th>网络使用者</th>
                  <th>分布情况</th>
                  <th>全网总上传</th>
                  <th>全网总下载</th>
                  <th>消耗总流</th>
                  <th>全局操作</th>
                </tr>
              </thead>
              <tbody>
                {users.map((u) => {
                  const nodeCount = u.nodes.length;
                  const totalUsed = formatBytes(u.traffic_up + u.traffic_down);
                  
                  return (
                    <tr key={u.username}>
                      <td><strong>{u.username}</strong></td>
                      <td>
                        <span className="badge badge-success">分布在 {nodeCount} 个节点</span>
                      </td>
                      <td>{formatBytes(u.traffic_up)}</td>
                      <td>{formatBytes(u.traffic_down)}</td>
                      <td><strong>{totalUsed}</strong></td>
                      <td>
                        <div className="btn-group">
                          <button className="btn btn-secondary btn-sm" onClick={() => {
                            setSelectedUser(u);
                            setShowInfoModal(true);
                          }}>
                            管理分节点明细
                          </button>
                          <button className="btn btn-danger btn-sm" onClick={() => deleteGlobalUser(u.username)}>
                            彻底除名
                          </button>
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}

        {/* Global Add User Modal */}
        {showAddModal && (
          <div className="modal-overlay" onClick={() => setShowAddModal(false)}>
            <div className="modal" onClick={(e) => e.stopPropagation()}>
              <h3>新增代理使用者</h3>
              <p style={{ color: 'var(--text-muted)', fontSize: '13px', marginBottom: '20px' }}>
                将同时在当前在线的 {nodes.length} 台节点上创建配置。
              </p>
              
              <div className="form-group">
                <label>英文用户名 (例如: alice)</label>
                <input className="form-input" value={newUsername} onChange={(e) => setNewUsername(e.target.value)} />
              </div>

              <div className="form-group" style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <input 
                  type="checkbox" 
                  id="createAuth" 
                  checked={createPanelAccount} 
                  onChange={(e) => setCreatePanelAccount(e.target.checked)} 
                />
                <label htmlFor="createAuth" style={{ margin: 0, cursor: 'pointer' }}>
                  同时为TA创建独立的“面板登录账号”（普通用户权限）
                </label>
              </div>

              {createPanelAccount && (
                <div className="form-group" style={{ marginTop: '12px', background: 'var(--bg-input)', padding: '16px', borderRadius: '8px' }}>
                  <label>为其设置面板登录密码：</label>
                  <input 
                    className="form-input" 
                    type="password"
                    placeholder="至少 4 位"
                    value={newPassword}
                    onChange={(e) => setNewPassword(e.target.value)}
                  />
                  <p style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '8px' }}>
                    注：面板登录账号为与此相同的 `{newUsername}`。<br/>
                    TA 登录面板后，将能看到这里刚创建的代理配置信息，但无法改动。
                  </p>
                </div>
              )}

              <div className="modal-actions">
                <button className="btn btn-secondary" onClick={() => setShowAddModal(false)}>取消</button>
                <button className="btn btn-primary" onClick={addGlobalUser}>统一下发</button>
              </div>
            </div>
          </div>
        )}

        {/* Detailed Breakdown Modal */}
        {showInfoModal && selectedUser && (
          <div className="modal-overlay" onClick={() => setShowInfoModal(false)}>
            <div className="modal" style={{ maxWidth: '800px' }} onClick={(e) => e.stopPropagation()}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
                <h3>👤 【{selectedUser.username}】 的资产版图</h3>
                <h3 style={{ color: 'var(--accent)' }}>总计: {formatBytes(selectedUser.traffic_up + selectedUser.traffic_down)}</h3>
              </div>
              
              {(() => {
                const pAcc = panelAccounts.find(p => p.proxy_username === selectedUser.username);
                return pAcc && pAcc.sub_token ? (
                  <div className="form-group" style={{ background: 'var(--bg-input)', padding: '16px', borderRadius: '8px', marginBottom: '16px' }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                      <strong style={{ fontSize: '15px' }}>🌐 全局融合订阅链接</strong>
                      <button onClick={() => copyToClipboard(`${window.location.origin}/api/sub/global/${pAcc.sub_token}`)} className="btn btn-primary btn-sm">
                        🔗 一键复制订阅 (Clash)
                      </button>
                    </div>
                    <p style={{ fontSize: '13px', color: 'var(--text-muted)', margin: '8px 0 16px 0' }}>
                      该订阅链接返回系统配置的**统一样板**，并自动灌入该用户在全网所有节点的可用代理配置。随时保持最新！
                    </p>
                    <div style={{ borderTop: '1px solid var(--border)', paddingTop: '12px', display: 'flex', gap: '8px', alignItems: 'center' }}>
                      <span style={{ fontSize: '13px', color: 'var(--text-muted)' }}>修改 {pAcc.username} 面板密码:</span>
                      <input 
                         type="text" 
                         className="form-input" 
                         style={{ width: '150px', padding: '4px 8px', fontSize: '13px' }}
                         placeholder="输入新密码" 
                         value={resetPwd}
                         onChange={(e) => setResetPwd(e.target.value)} 
                      />
                      <button className="btn btn-secondary btn-sm" onClick={() => handeResetPassword(pAcc.id)}>保存新密码</button>
                    </div>
                  </div>
                ) : (
                  <div className="alert-info" style={{ marginBottom: '16px', fontSize: '13px' }}>
                    尚无全局订阅链接：该代理账号没有绑定的“面板系统账号”。请在新建用户时勾选“生成面板账号”。
                  </div>
                );
              })()}

              <div className="table-container" style={{ maxHeight: '400px', overflowY: 'auto' }}>
                <table style={{ minWidth: '100%' }}>
                  <thead style={{ position: 'sticky', top: 0, zIndex: 10 }}>
                    <tr>
                      <th>节点名称</th>
                      <th>占用流量</th>
                      <th>状态</th>
                      <th>单点配置鉴权凭证</th>
                      <th>操作</th>
                    </tr>
                  </thead>
                  <tbody>
                    {selectedUser.nodes.map(n => (
                      <tr key={n.nodeId}>
                        <td><strong>{n.name}</strong></td>
                        <td>{formatBytes(n.traffic_up + n.traffic_down)}<br/><span style={{fontSize: '11px', color: 'var(--text-muted)'}}>↑{formatBytes(n.traffic_up)} ↓{formatBytes(n.traffic_down)}</span></td>
                        <td>
                           <span className={`badge ${n.enabled ? 'badge-success' : 'badge-danger'}`}>
                            {n.enabled ? '正常通行' : '已没收'}
                          </span>
                        </td>
                        <td>
                          <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                             {n.enabled ? (
                               <>
                                  <button onClick={() => copyToClipboard(n.uuid)} className="copy-text" style={{ borderColor: 'var(--text-muted)' }}>
                                     VLESS UUID
                                  </button>
                                  <button onClick={() => copyToClipboard(n.hy2_password)} className="copy-text" style={{ borderColor: 'var(--text-muted)' }}>
                                     Hy2 Password
                                  </button>
                               </>
                             ) : (
                               <span style={{ fontSize: '12px', color: 'var(--text-muted)' }}>暂定中，无法获取配置</span>
                             )}
                          </div>
                        </td>
                        <td>
                           <button 
                             className={`btn btn-sm ${n.enabled ? 'btn-danger' : 'btn-success'}`}
                             onClick={() => toggleUserOnNode(n.nodeId, selectedUser.username, n.enabled)}
                           >
                             {n.enabled ? '拔网线 (单点禁用)' : '插网线 (单点启用)'}
                           </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>

              <div className="modal-actions" style={{ marginTop: '24px' }}>
                <button className="btn btn-secondary" onClick={() => setShowInfoModal(false)}>关闭面板</button>
              </div>
            </div>
          </div>
        )}

        {toast && <div className={`toast toast-${toast.type}`}>{toast.msg}</div>}
      </main>
    </div>
  );
}
