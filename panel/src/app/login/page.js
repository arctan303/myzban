'use client';
import { useState } from 'react';

export default function LoginPage() {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const handleLogin = async (e) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    try {
      const res = await fetch('/api/auth', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password }),
      });

      const data = await res.json();

      if (!res.ok) {
        setError(data.error || '登录失败');
        setLoading(false);
        return;
      }

      // Store token and user info
      localStorage.setItem('pnm_token', data.token);
      localStorage.setItem('pnm_user', JSON.stringify(data.user));

      // Redirect based on role
      if (data.user.role === 'admin') {
        window.location.href = '/';
      } else {
        window.location.href = '/my';
      }
    } catch (err) {
      setError('网络错误，请稍后重试');
      setLoading(false);
    }
  };

  return (
    <div className="login-container">
      <div className="login-card">
        <div className="login-logo">🛡️</div>
        <h1>PNM Panel</h1>
        <p>Proxy Node Manager — 安全登录</p>

        <form onSubmit={handleLogin}>
          {error && (
            <div className="login-error">{error}</div>
          )}

          <div className="form-group">
            <label>用户名</label>
            <input
              className="form-input"
              type="text"
              placeholder="请输入用户名"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              autoFocus
              autoComplete="username"
            />
          </div>

          <div className="form-group">
            <label>密码</label>
            <input
              className="form-input"
              type="password"
              placeholder="请输入密码"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="current-password"
            />
          </div>

          <button
            className="btn btn-primary login-btn"
            type="submit"
            disabled={loading || !username || !password}
          >
            {loading ? '正在验证...' : '登 录'}
          </button>
        </form>
      </div>
    </div>
  );
}
