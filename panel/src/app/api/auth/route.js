// POST /api/auth — login
// GET  /api/auth — get current user info (requires token)
// PUT  /api/auth — change user password (admin only)
import { getDb } from '../../../lib/db';
import { signToken, requireAuth, requireAdmin } from '../../../lib/auth';

const bcrypt = require('bcryptjs');

export async function POST(request) {
  const { username, password } = await request.json();

  if (!username || !password) {
    return Response.json({ error: '请输入用户名和密码' }, { status: 400 });
  }

  const db = getDb();
  const user = db.prepare('SELECT * FROM panel_users WHERE username = ?').get(username);

  if (!user || !bcrypt.compareSync(password, user.password_hash)) {
    return Response.json({ error: '用户名或密码不正确' }, { status: 401 });
  }

  const token = signToken({
    id: user.id,
    username: user.username,
    role: user.role,
    proxy_username: user.proxy_username,
  });

  return Response.json({
    token,
    user: {
      id: user.id,
      username: user.username,
      role: user.role,
      proxy_username: user.proxy_username,
    },
  });
}

// GET /api/auth — verify token and return user info
export async function GET(request) {
  const user = requireAuth(request);
  if (!user) {
    return Response.json({ error: 'Unauthorized' }, { status: 401 });
  }
  return Response.json({ user });
}

// PUT /api/auth — admin changes a user's password
export async function PUT(request) {
  const admin = requireAdmin(request);
  if (!admin) {
    return Response.json({ error: 'Admin access required' }, { status: 403 });
  }

  const { target_username, new_password } = await request.json();

  if (!target_username || !new_password) {
    return Response.json({ error: '请提供目标用户名和新密码' }, { status: 400 });
  }

  if (new_password.length < 4) {
    return Response.json({ error: '密码长度至少 4 位' }, { status: 400 });
  }

  const db = getDb();
  const target = db.prepare('SELECT id FROM panel_users WHERE username = ?').get(target_username);
  if (!target) {
    return Response.json({ error: '用户不存在' }, { status: 404 });
  }

  const hash = bcrypt.hashSync(new_password, 10);
  db.prepare('UPDATE panel_users SET password_hash = ? WHERE id = ?').run(hash, target.id);

  return Response.json({ status: 'ok', message: `密码已更新：${target_username}` });
}
