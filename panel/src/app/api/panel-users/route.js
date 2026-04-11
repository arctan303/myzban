// GET  /api/panel-users — list panel accounts (admin only)
// POST /api/panel-users — create panel account (admin only)
// PUT  /api/panel-users — update panel account username/password (admin only)
// DELETE /api/panel-users?id=X — delete panel account (admin only)
import { getDb } from '../../../lib/db';
import { requireAdmin } from '../../../lib/auth';

const bcrypt = require('bcryptjs');

export async function GET(request) {
  const admin = requireAdmin(request);
  if (!admin) {
    return Response.json({ error: 'Admin access required' }, { status: 403 });
  }

  const db = getDb();
  const users = db.prepare(
    'SELECT id, username, role, proxy_username, created_at FROM panel_users ORDER BY id'
  ).all();

  return Response.json(users);
}

export async function POST(request) {
  const admin = requireAdmin(request);
  if (!admin) {
    return Response.json({ error: 'Admin access required' }, { status: 403 });
  }

  const { username, password, role, proxy_username } = await request.json();

  if (!username || !password) {
    return Response.json({ error: '用户名和密码必填' }, { status: 400 });
  }

  const finalRole = role === 'admin' ? 'admin' : 'user';

  const db = getDb();
  const existing = db.prepare('SELECT id FROM panel_users WHERE username = ?').get(username);
  if (existing) {
    return Response.json({ error: `面板账号 "${username}" 已存在` }, { status: 409 });
  }

  const hash = bcrypt.hashSync(password, 10);
  const result = db.prepare(
    'INSERT INTO panel_users (username, password_hash, role, proxy_username) VALUES (?, ?, ?, ?)'
  ).run(username, hash, finalRole, proxy_username || null);

  return Response.json({
    id: result.lastInsertRowid,
    username,
    role: finalRole,
    proxy_username: proxy_username || null,
  }, { status: 201 });
}

export async function PUT(request) {
  const admin = requireAdmin(request);
  if (!admin) {
    return Response.json({ error: 'Admin access required' }, { status: 403 });
  }

  const { id, username, new_password } = await request.json();

  if (!id || !username) {
    return Response.json({ error: 'id and username required' }, { status: 400 });
  }

  const db = getDb();
  const existing = db.prepare('SELECT id FROM panel_users WHERE username = ? AND id != ?').get(username, id);
  if (existing) {
    return Response.json({ error: `用户名 "${username}" 已被其他账号使用` }, { status: 409 });
  }

  if (new_password) {
    if (new_password.length < 4) {
      return Response.json({ error: '密码长度至少 4 位' }, { status: 400 });
    }
    const hash = bcrypt.hashSync(new_password, 10);
    db.prepare('UPDATE panel_users SET username = ?, password_hash = ? WHERE id = ?').run(username, hash, id);
  } else {
    db.prepare('UPDATE panel_users SET username = ? WHERE id = ?').run(username, id);
  }

  return Response.json({ status: 'updated' });
}

export async function DELETE(request) {
  const admin = requireAdmin(request);
  if (!admin) {
    return Response.json({ error: 'Admin access required' }, { status: 403 });
  }

  const { searchParams } = new URL(request.url);
  const id = searchParams.get('id');

  if (!id) {
    return Response.json({ error: 'id required' }, { status: 400 });
  }

  const db = getDb();
  const target = db.prepare('SELECT * FROM panel_users WHERE id = ?').get(id);
  if (!target) {
    return Response.json({ error: '账号不存在' }, { status: 404 });
  }

  // Prevent deleting the last admin
  if (target.role === 'admin') {
    const adminCount = db.prepare('SELECT COUNT(*) as cnt FROM panel_users WHERE role = ?').get('admin');
    if (adminCount.cnt <= 1) {
      return Response.json({ error: '不能删除最后一个管理员账号' }, { status: 400 });
    }
  }

  db.prepare('DELETE FROM panel_users WHERE id = ?').run(id);
  return Response.json({ status: 'deleted' });
}
