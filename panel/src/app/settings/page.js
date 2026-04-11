'use client';
import { useState, useEffect } from 'react';
import { useAdminAuth, AdminSidebar, authFetch } from '../../lib/adminAuth';

export default function SettingsPage() {
  const { ready } = useAdminAuth();
  const [template, setTemplate] = useState('');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [toast, setToast] = useState(null);

  const showToast = (msg, type = 'success') => {
    setToast({ msg, type });
    setTimeout(() => setToast(null), 3000);
  };

  useEffect(() => {
    if (!ready) return;
    authFetch('/api/template')
      .then(res => res.json())
      .then(data => {
        setTemplate(data.template || '');
        setLoading(false);
      });
  }, [ready]);

  const saveTemplate = async () => {
    setSaving(true);
    try {
      const res = await authFetch('/api/template', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ template })
      });
      if (res.ok) {
        showToast('系统默认配置模板已成功更新！');
      } else {
        throw new Error('保存失败');
      }
    } catch (e) {
      showToast(e.message, 'error');
    } finally {
      setSaving(false);
    }
  };

  if (!ready) return null;

  return (
    <div className="app-layout">
      <AdminSidebar currentPage="/settings" />
      <main className="main-content" style={{ display: 'flex', flexDirection: 'column', height: '100vh', overflow: 'hidden' }}>
        <div className="page-header" style={{ paddingBottom: '16px', marginBottom: 0 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <div>
              <h2>⚙️ 系统配置</h2>
              <p>管理员可在此随时更改或覆盖向外分发配置的源模板规则</p>
            </div>
            <button className="btn btn-primary" onClick={saveTemplate} disabled={saving || loading}>
              {saving ? '保存中...' : '💾 保存默认模板'}
            </button>
          </div>
        </div>

        {loading ? (
          <div className="loading" style={{ flex: 1 }}><div className="spinner"></div>系统加载中...</div>
        ) : (
          <div style={{ flex: 1, padding: '0 24px 24px 24px', display: 'flex', flexDirection: 'column' }}>
            <div className="alert-info" style={{ marginBottom: '16px' }}>
              <strong>开发说明：</strong> 此处的配置将作为全网唯一的默认分流基石。当订阅分发时，系统会在 
              <code style={{margin: '0 4px', color:'var(--danger)'}}>&lt;__PROXIES__&gt;</code> 的位置注入全网在线节点信息阵列，并在
              <code style={{margin: '0 4px', color:'var(--danger)'}}>&lt;__PROXY_NAMES__&gt;</code> 的位置注入节点名字列表。<br/>
              如果不需要此功能，请勿随意改动占位符。
            </div>
            <textarea
              className="form-input"
              style={{
                flex: 1,
                fontFamily: 'Consolas, monospace',
                fontSize: '13px',
                lineHeight: 1.5,
                resize: 'none',
                minHeight: '400px'
              }}
              value={template}
              onChange={e => setTemplate(e.target.value)}
              spellCheck="false"
            />
          </div>
        )}

        {toast && <div className={`toast toast-${toast.type}`}>{toast.msg}</div>}
      </main>
    </div>
  );
}
